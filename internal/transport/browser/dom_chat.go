package browser

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// DOM fallback for chat.
//
// When the chat RPC shape drifts, the question is asked by driving the
// signed-in NotebookLM page: set the query box, click submit, then poll the
// answer thread. The answer is complete when the newest bubble is new (the
// bubble count rose), carries the done marker (.message-actions) and its
// text has not changed for answerStable. Only the text is returned;
// citation markers stay as rendered ("[1]") and are not resolved.

const (
	answerPoll   = 250 * time.Millisecond
	answerStable = time.Second
	// answerTotal is the hard ceiling for one answer, the rpc transport's
	// default StreamTotal.
	answerTotal = 15 * time.Minute
	// limitGrace is how long a rate-limit alert that was already showing
	// before the question was sent may stay without a new answer bubble
	// before it counts as this question's answer.
	limitGrace = selectorWait
	navTimeout = 45 * time.Second
	// idleWait bounds the wait for an earlier answer to finish before a
	// question is sent.
	idleWait = 60 * time.Second
)

// DOM drives NotebookLM's web UI as the fallback for chat and add-source
// when their RPC shape drifted. Calls share the browser's warm tab (FIFO)
// with the fetch transport.
type DOM struct {
	acquire func(ctx context.Context) (page, func(), error)
	origin  string // https://<host>; tests point it at an httptest server
	sel     Selectors

	wait  time.Duration // per-control wait (selectorWait)
	total time.Duration // answer ceiling (answerTotal)
	idle  time.Duration // wait for an earlier answer before sending (idleWait)

	mu sync.Mutex
	// abandoned is the bubble count before a question whose Ask ended
	// (cancelled) after sending it; -1 = none. Its answer may still be
	// on the way, so the next Ask waits until the count passes it.
	abandoned int
}

// NewDOM returns the DOM fallback on the browser that get returns (see
// Launcher.Get), for notebooks on host, resolving controls with sel (see
// LoadSelectors; nil = embedded defaults).
func NewDOM(get func(ctx context.Context) (*Browser, error), host session.Host, sel Selectors) *DOM {
	return newDOM(tabAcquirer(get), host, sel)
}

func newDOM(acquire func(ctx context.Context) (page, func(), error), host session.Host, sel Selectors) *DOM {
	if host == "" {
		host = session.HostDefault
	}
	if sel == nil {
		sel = LoadSelectors("", nil)
	}
	return &DOM{acquire: acquire, origin: host.Origin(), sel: sel, wait: selectorWait, total: answerTotal,
		idle: idleWait, abandoned: -1}
}

// chatState is what __sbDom.chatState reports.
type chatState struct {
	Count int    `json:"count"`
	Text  string `json:"text"`
	Done  bool   `json:"done"`
	Alert string `json:"alert"`
}

// Ask asks question in notebook nb's default chat thread and returns the
// answer text. progress, if not nil, gets the answer so far at most once
// per answerStable.
func (d *DOM) Ask(ctx context.Context, nb, question string, progress func(text string)) (string, error) {
	const rpc = "chat.ask"
	p, release, err := d.acquire(ctx)
	if err != nil {
		return "", d.wrap(ctx, rpc, false, err)
	}
	defer release()
	navigated, err := d.open(ctx, p, nb, "")
	if err != nil {
		return "", d.wrap(ctx, rpc, false, err)
	}
	if navigated {
		d.setAbandoned(-1) // a fresh page: no earlier answer is on its way
	}
	if err := d.sel.find(ctx, p, "chat.input", d.wait); err != nil {
		return "", d.wrap(ctx, rpc, false, err)
	}
	before, err := d.waitIdle(ctx, p)
	if err != nil {
		return "", d.wrap(ctx, rpc, false, err)
	}
	if err := d.fill(ctx, p, "chat.input", question, ""); err != nil {
		return "", d.wrap(ctx, rpc, false, err)
	}
	// From the submit click on the question may have been sent. If this
	// call ends without its answer (cancelled, page error), the answer may
	// still arrive later; the next Ask must not take it for its own. Only a
	// submit button that was not there to click (drift) means not sent.
	sent, answered := true, false
	defer func() {
		if sent && !answered {
			d.setAbandoned(before.Count)
		}
	}()
	if err := d.press(ctx, p, "chat.submit", ""); err != nil {
		if isDriftErr(err) {
			sent = false
			return "", d.wrap(ctx, rpc, false, err)
		}
		return "", d.wrap(ctx, rpc, true, err)
	}
	sentAt := time.Now()
	deadline := sentAt.Add(d.total)
	var last string
	var changedAt, progressAt time.Time
	for {
		st, err := d.chat(ctx, p)
		if err != nil {
			return "", d.wrap(ctx, rpc, true, err)
		}
		now := time.Now()
		if st.Count > before.Count {
			text := cleanAnswer(st.Text)
			switch {
			case text != last || changedAt.IsZero():
				last, changedAt = text, now
				if progress != nil && text != "" && (progressAt.IsZero() || now.Sub(progressAt) >= answerStable) {
					progress(text)
					progressAt = now
				}
			case st.Done && now.Sub(changedAt) >= answerStable:
				answered = true
				if text == "" {
					// Finished, but nothing readable: the text selector drifted.
					e := d.sel.drift("chat.bubble_text")
					e.RPC, e.Sent = rpc, true
					return "", e
				}
				return text, nil
			}
		} else if isRateLimit(st.Alert) && (st.Alert != before.Alert || now.Sub(sentAt) >= limitGrace) {
			answered = true // refused: no answer will follow
			return "", &transport.Error{Kind: transport.KindRateLimited, RPC: rpc, Transport: domTransport, Sent: true,
				Message: "NotebookLM shows a chat limit: " + firstLine(st.Alert)}
		}
		if now.After(deadline) {
			return "", &transport.Error{Kind: transport.KindTransport, RPC: rpc, Transport: domTransport, Sent: true,
				Message: fmt.Sprintf("answer did not complete within %s", d.total)}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(answerPoll):
		}
	}
}

// waitIdle waits until no answer is in progress (no bubbles, or the newest
// one carries the done marker) and the answer to an abandoned question, if
// any, has arrived; it returns the thread state to compare against. When
// the page stays busy for d.idle it fails without sending anything.
//
// An abandoned question that has shown no bubble after limitGrace (the
// count has not changed since the wait began) is taken as never answered:
// it no longer holds up the next question.
func (d *DOM) waitIdle(ctx context.Context, p page) (chatState, error) {
	abandoned := d.getAbandoned()
	start := time.Now()
	deadline := start.Add(d.idle)
	startCount := -1
	for {
		st, err := d.chat(ctx, p)
		if err != nil {
			return chatState{}, err
		}
		if startCount < 0 {
			startCount = st.Count
		}
		idle := st.Count == 0 || st.Done
		if idle && (st.Count > abandoned || (st.Count == startCount && time.Since(start) >= limitGrace)) {
			d.setAbandoned(-1)
			return st, nil
		}
		if time.Now().After(deadline) {
			// Do not block later calls on an answer that never came.
			d.setAbandoned(-1)
			reason := "an earlier answer is still in progress"
			if idle {
				reason = "the answer to an abandoned question has not arrived"
			}
			return chatState{}, &transport.Error{Kind: transport.KindTransport, Transport: domTransport,
				Message: fmt.Sprintf("chat not idle after %s: %s; the question was not sent", d.idle, reason)}
		}
		select {
		case <-ctx.Done():
			return chatState{}, ctx.Err()
		case <-time.After(answerPoll):
		}
	}
}

func (d *DOM) getAbandoned() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.abandoned
}

func (d *DOM) setAbandoned(n int) {
	d.mu.Lock()
	d.abandoned = n
	d.mu.Unlock()
}

func (d *DOM) chat(ctx context.Context, p page) (chatState, error) {
	s := d.sel
	arg, err := json.Marshal([][]string{s["chat.bubble"], s["chat.bubble_text"], s["chat.done"], s["chat.alert"], s["chat.panel"], s["chat.input"]})
	if err != nil {
		return chatState{}, err
	}
	raw, err := domEval(ctx, p, `__sbDom.chatState(...`+string(arg)+`)`)
	if err != nil {
		return chatState{}, err
	}
	var st chatState
	if err := json.Unmarshal(raw, &st); err != nil {
		return chatState{}, fmt.Errorf("browser: chat state: %w", err)
	}
	return st, nil
}

// open puts the tab on notebook nb (with query, if any) unless it is
// already there, and reports a sign-in redirect as KindAuth. navigated
// reports that the page was (re)loaded.
func (d *DOM) open(ctx context.Context, p page, nb, query string) (navigated bool, err error) {
	target := d.origin + "/notebook/" + url.PathEscape(nb)
	if query == "" {
		if href, err := d.href(ctx, p); err == nil && samePage(href, target) {
			return false, nil
		}
	} else {
		target += "?" + query
	}
	nctx, cancel := context.WithTimeout(ctx, navTimeout)
	defer cancel()
	if err := p.Navigate(nctx, target); err != nil {
		return true, fmt.Errorf("browser: open notebook: %w", err)
	}
	href, err := d.href(ctx, p)
	if err != nil {
		return true, err
	}
	if u, err := url.Parse(href); err == nil && u.Hostname() == "accounts.google.com" {
		return true, &transport.Error{Kind: transport.KindAuth, Transport: domTransport, Cause: fmt.Errorf("%w (sign-in page)", session.ErrSignedOut)}
	}
	return true, nil
}

func (d *DOM) href(ctx context.Context, p page) (string, error) {
	raw, err := p.Eval(ctx, `location.href`)
	if err != nil {
		return "", fmt.Errorf("browser: read location: %w", err)
	}
	var href string
	if err := json.Unmarshal(raw, &href); err != nil {
		return "", fmt.Errorf("browser: location: %w", err)
	}
	return href, nil
}

// samePage reports whether href is target, ignoring query and fragment.
func samePage(href, target string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	u.RawQuery, u.Fragment = "", ""
	return strings.TrimSuffix(u.String(), "/") == strings.TrimSuffix(target, "/")
}

// fill waits for control key inside root (see Selectors.findIn; "" = the
// document) and sets its value.
func (d *DOM) fill(ctx context.Context, p page, key, value, root string) error {
	return d.act(ctx, p, key, root, func(cands []byte, r string) (string, error) {
		v, err := json.Marshal(value)
		return `__sbDom.setValue(` + string(cands) + `, ` + string(v) + `, ` + r + `)`, err
	})
}

// press waits for control key inside root and clicks it.
func (d *DOM) press(ctx context.Context, p page, key, root string) error {
	return d.act(ctx, p, key, root, func(cands []byte, r string) (string, error) {
		return `__sbDom.click(` + string(cands) + `, ` + r + `)`, nil
	})
}

// act finds key inside root and runs the action script on it. A KindDrift
// from act means the control was not clicked or filled.
func (d *DOM) act(ctx context.Context, p page, key, root string, script func(cands []byte, root string) (string, error)) error {
	if _, err := d.sel.findIn(ctx, p, root, []string{key}, d.wait); err != nil {
		return err
	}
	if root == "" {
		root = "undefined"
	}
	cands, err := json.Marshal(d.sel[key])
	if err != nil {
		return err
	}
	js, err := script(cands, root)
	if err != nil {
		return err
	}
	raw, err := domEval(ctx, p, js)
	if err != nil {
		return err
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil || !ok {
		return d.sel.drift(key) // it vanished between find and the action
	}
	return nil
}

// wrap returns a *transport.Error for rpc: its own kind when err already is
// one, else KindTransport. When ctx ended it returns the caller's error,
// still typed with Sent when the request may have gone out (errors.Is
// keeps matching context.Canceled through Cause).
func (d *DOM) wrap(ctx context.Context, rpc string, sent bool, err error) error {
	if ctx.Err() != nil {
		if !sent {
			return ctx.Err()
		}
		err = ctx.Err()
	}
	var te *transport.Error
	if errors.As(err, &te) {
		out := *te
		if out.RPC == "" {
			out.RPC = rpc
		}
		if out.Transport == "" || out.Transport == "browser" {
			out.Transport = domTransport
		}
		out.Sent = out.Sent || sent
		return &out
	}
	return &transport.Error{Kind: transport.KindTransport, RPC: rpc, Transport: domTransport, Sent: sent, Cause: err}
}

// iconLines are Material icon ligature names that leak into innerText.
var iconLines = map[string]bool{"more_vert": true, "content_copy": true, "keep_pin": true, "thumb_up": true, "thumb_down": true}

// cleanAnswer drops icon ligature lines and trims the text.
func cleanAnswer(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := lines[:0]
	for _, l := range lines {
		if !iconLines[strings.TrimSpace(l)] {
			out = append(out, l)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// rateLimitRE matches chat-limit alerts only: the daily-limit message and
// generic rate-limit wording. Broader words (quota, "reached ... limit")
// also appear in unrelated Studio and upload alerts.
var rateLimitRE = regexp.MustCompile(`(?i)\bdaily limit\b|\brate limit\b|\btoo many requests\b`)

// isDriftErr reports whether err is a KindDrift transport error.
func isDriftErr(err error) bool {
	var te *transport.Error
	return errors.As(err, &te) && te.Kind == transport.KindDrift
}

func isRateLimit(alert string) bool { return alert != "" && rateLimitRE.MatchString(alert) }

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}
