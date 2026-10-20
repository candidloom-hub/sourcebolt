package browser

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// DOM fallback for add-source.
//
// Open the add-source dialog, pick the source type, fill the field and
// click Insert. Done is the source row count rising by one with the dialog
// gone. Only URL and pasted text exist here: Drive sources stay rejected
// (KindUnsupported) by the caller, as on the RPC path.

const (
	sourcePoll = 250 * time.Millisecond
	// sourceWait bounds the wait for the new source row after Insert.
	sourceWait = 90 * time.Second
)

// AddURL adds a website (or YouTube) URL to notebook nb.
func (d *DOM) AddURL(ctx context.Context, nb, url string) error {
	return d.addSource(ctx, nb, "sources.type_link", "sources.url_input", url)
}

// AddText adds pasted text to notebook nb. title is not used: the paste
// dialog has no title field, NotebookLM names pasted text itself.
func (d *DOM) AddText(ctx context.Context, nb, title, content string) error {
	return d.addSource(ctx, nb, "sources.type_text", "sources.text_input", content)
}

func (d *DOM) addSource(ctx context.Context, nb, typeKey, inputKey, value string) error {
	const rpc = "sources.add"
	p, release, err := d.acquire(ctx)
	if err != nil {
		return d.wrap(ctx, rpc, false, err)
	}
	defer release()
	if navigated, err := d.open(ctx, p, nb, ""); err != nil {
		return d.wrap(ctx, rpc, false, err)
	} else if navigated {
		d.setAbandoned(-1) // a reload drops any answer still on its way
	}
	before, err := d.openAddDialog(ctx, p, nb)
	if err != nil {
		return d.wrap(ctx, rpc, false, err)
	}
	// Type, input and Insert resolve inside the add dialog only: the page
	// has look-alike controls elsewhere (Studio buttons, row icons).
	root := d.addDialog()
	if err := d.press(ctx, p, typeKey, root); err != nil {
		return d.wrap(ctx, rpc, false, err)
	}
	if err := d.fill(ctx, p, inputKey, value, root); err != nil {
		return d.wrap(ctx, rpc, false, err)
	}
	// From the Insert attempt on the source may have been added, and the
	// caller must not add it again: every error but an Insert that was not
	// there to click (drift) reports Sent.
	if err := d.press(ctx, p, "sources.insert", root); err != nil {
		return d.wrap(ctx, rpc, !isDriftErr(err) || ctx.Err() != nil, err)
	}
	deadline := time.Now().Add(sourceWait)
	for {
		rows, open, err := d.sourceState(ctx, p)
		if err != nil {
			return d.wrap(ctx, rpc, true, err)
		}
		if rows > before && !open {
			return nil
		}
		if time.Now().After(deadline) {
			return &transport.Error{Kind: transport.KindTransport, RPC: rpc, Transport: domTransport, Sent: true,
				Message: fmt.Sprintf("the new source did not appear within %s", sourceWait)}
		}
		select {
		case <-ctx.Done():
			return d.wrap(ctx, rpc, true, ctx.Err())
		case <-time.After(sourcePoll):
		}
	}
}

// addDialog is the JS expression for the open add-source dialog element
// (null when none): a sources.dialog match holding both source-type
// buttons. Another dialog (share, confirm) is not the add dialog.
func (d *DOM) addDialog() string {
	arg, _ := json.Marshal(map[string]any{
		"dialog":  d.sel["sources.dialog"],
		"anchors": [][]string{d.sel["sources.type_link"], d.sel["sources.type_text"]},
	})
	return `__sbDom.addDialog(` + string(arg) + `)`
}

// openAddDialog opens the add-source dialog and returns the source row
// count before the add. When the add button is missing it reloads the
// notebook once with ?addSource=true, which opens the dialog (or at least
// shows the button), before reporting drift on sources.add.
func (d *DOM) openAddDialog(ctx context.Context, p page, nb string) (int, error) {
	add := d.addDialog()
	if _, err := domEval(ctx, p, `__sbDom.clearAdd()`); err != nil {
		return 0, err
	}
	if _, err := d.sel.poll(ctx, p, add+` ? 0 : -1`, 0, "sources.dialog"); err == nil {
		return d.rowCount(ctx, p) // already open
	}
	before, err := d.rowCount(ctx, p)
	if err != nil {
		return 0, err
	}
	err = d.press(ctx, p, "sources.add", "")
	if isDriftErr(err) {
		if _, err := d.open(ctx, p, nb, "addSource=true"); err != nil {
			return 0, err
		}
		d.setAbandoned(-1)
		if before, err = d.rowCount(ctx, p); err != nil {
			return 0, err
		}
		cands, _ := json.Marshal(d.sel["sources.add"])
		idx, ferr := d.sel.poll(ctx, p, `(`+add+` ? 0 : (__sbDom.find(`+string(cands)+`) ? 1 : -1))`, d.wait, "sources.add")
		switch {
		case ferr != nil:
			return 0, ferr
		case idx == 0:
			return before, nil
		}
		err = d.press(ctx, p, "sources.add", "")
	}
	if err != nil {
		return 0, err
	}
	if _, err := d.sel.poll(ctx, p, add+` ? 0 : -1`, d.wait, "sources.dialog"); err != nil {
		return 0, err
	}
	return before, nil
}

func (d *DOM) rowCount(ctx context.Context, p page) (int, error) {
	rows, _, err := d.sourceState(ctx, p)
	return rows, err
}

// sourceState counts the source rows and reports whether the add dialog
// is open.
func (d *DOM) sourceState(ctx context.Context, p page) (rows int, open bool, err error) {
	arg, err := json.Marshal(d.sel["sources.row"])
	if err != nil {
		return 0, false, err
	}
	raw, err := domEval(ctx, p, `[__sbDom.present(`+string(arg)+`).length, (__sbDom.dismissPromos(), `+d.addDialog()+` ? 1 : 0)]`)
	if err != nil {
		return 0, false, err
	}
	var n []int
	if err := json.Unmarshal(raw, &n); err != nil || len(n) != 2 {
		return 0, false, fmt.Errorf("browser: source state %s: %v", raw, err)
	}
	return n[0], n[1] == 1, nil
}
