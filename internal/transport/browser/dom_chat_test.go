package browser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// chatPage is a fake notebook tab with an idle, answered thread: every
// control resolves, setValue succeeds, and the submit click answers click.
type chatPage struct {
	click func() (string, error) // JSON value of __sbDom.click, or an error
}

func (p *chatPage) Eval(ctx context.Context, js string) ([]byte, error) {
	v := ""
	switch {
	case js == `location.href`:
		return []byte(`"https://notebook.google.com/notebook/nb"`), nil
	case js == domJS:
		return []byte("true"), nil
	case strings.Contains(js, "__sbDom.chatState("):
		v = `{"count":1,"text":"old","done":true,"alert":""}`
	case strings.Contains(js, "__sbDom.setValue("):
		v = "true"
	case strings.Contains(js, "__sbDom.click("):
		r, err := p.click()
		if err != nil {
			return nil, err
		}
		v = r
	case strings.Contains(js, "findIndex"):
		v = "0"
	default:
		return nil, errors.New("unexpected script")
	}
	return []byte(`{"missing":false,"v":` + v + `}`), nil
}

func (p *chatPage) Navigate(ctx context.Context, u string) error { return nil }

func newChatDOM(p page) *DOM {
	return newDOM(func(context.Context) (page, func(), error) { return p, func() {}, nil }, session.HostDefault, nil)
}

// Finding 5: a submit click that may have gone out (the eval failed after
// dispatch) counts as sent, and the next ask waits for its answer.
func TestAskDOMSubmitErrorIsSent(t *testing.T) {
	d := newChatDOM(&chatPage{click: func() (string, error) { return "", errors.New("connection reset") }})
	_, err := d.Ask(t.Context(), "nb", "q", nil)
	var te *transport.Error
	if !errors.As(err, &te) || !te.Sent {
		t.Fatalf("err = %#v, want Sent", err)
	}
	if n := d.getAbandoned(); n != 1 {
		t.Fatalf("abandoned = %d, want 1 (the bubble count before the question)", n)
	}
}

// sourcesFakePage is a fake notebook tab with the add dialog open: every
// control resolves and the Insert click answers insert.
type sourcesFakePage struct {
	insert func() (string, error)
}

func (p *sourcesFakePage) Eval(ctx context.Context, js string) ([]byte, error) {
	v := ""
	switch {
	case js == `location.href`:
		return []byte(`"https://notebook.google.com/notebook/nb"`), nil
	case js == domJS:
		return []byte("true"), nil
	case strings.Contains(js, "__sbDom.clearAdd()"):
		v = "true"
	case strings.Contains(js, "__sbDom.present("):
		v = "[1,1]" // one row, add dialog open
	case strings.Contains(js, "__sbDom.setValue("):
		v = "true"
	case strings.Contains(js, "__sbDom.click(") && strings.Contains(js, "mat-primary"):
		r, err := p.insert()
		if err != nil {
			return nil, err
		}
		v = r
	case strings.Contains(js, "__sbDom.click("):
		v = "true"
	case strings.Contains(js, "findIndex"), strings.Contains(js, "? 0 : -1"):
		v = "0"
	default:
		return nil, errors.New("unexpected script")
	}
	return []byte(`{"missing":false,"v":` + v + `}`), nil
}

func (p *sourcesFakePage) Navigate(ctx context.Context, u string) error { return nil }

// Finding 3: an Insert click that may have gone out (the eval failed after
// dispatch) reports Sent; an Insert that was not there to click does not.
func TestAddSourceDOMInsertErrorIsSent(t *testing.T) {
	d := newChatDOM(&sourcesFakePage{insert: func() (string, error) { return "", errors.New("connection reset") }})
	err := d.AddURL(t.Context(), "nb", "https://example.test/")
	var te *transport.Error
	if !errors.As(err, &te) || !te.Sent {
		t.Fatalf("err = %#v, want Sent", err)
	}
	d = newChatDOM(&sourcesFakePage{insert: func() (string, error) { return "false", nil }})
	err = d.AddURL(t.Context(), "nb", "https://example.test/")
	if !errors.As(err, &te) || te.Kind != transport.KindDrift || te.Sent {
		t.Fatalf("err = %#v, want unsent KindDrift", err)
	}
}

// A submit button that vanished before the click (drift) was not sent.
func TestAskDOMSubmitDriftIsNotSent(t *testing.T) {
	d := newChatDOM(&chatPage{click: func() (string, error) { return "false", nil }})
	_, err := d.Ask(t.Context(), "nb", "q", nil)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindDrift || te.Sent {
		t.Fatalf("err = %#v, want unsent KindDrift", err)
	}
	if n := d.getAbandoned(); n != -1 {
		t.Fatalf("abandoned = %d, want -1", n)
	}
}
