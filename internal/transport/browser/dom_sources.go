package browser

import (
	"context"
	"encoding/json/v2"
	"errors"
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
	for _, step := range []func() error{
		func() error { return d.press(ctx, p, typeKey) },
		func() error { return d.fill(ctx, p, inputKey, value) },
		func() error { return d.press(ctx, p, "sources.insert") },
	} {
		if err := step(); err != nil {
			return d.wrap(ctx, rpc, false, err)
		}
	}
	// Insert was clicked: from here on the source may have been added, and
	// the caller must not add it again.
	deadline := time.Now().Add(sourceWait)
	for {
		rows, dialogs, err := d.sourceState(ctx, p)
		if err != nil {
			return d.wrap(ctx, rpc, true, err)
		}
		if rows > before && dialogs == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return &transport.Error{Kind: transport.KindTransport, RPC: rpc, Transport: domTransport, Sent: true,
				Message: fmt.Sprintf("the new source did not appear within %s", sourceWait)}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sourcePoll):
		}
	}
}

// openAddDialog opens the add-source dialog and returns the source row
// count before the add. When the add button is missing it reloads the
// notebook once with ?addSource=true, which opens the dialog (or at least
// shows the button), before reporting drift on sources.add.
func (d *DOM) openAddDialog(ctx context.Context, p page, nb string) (int, error) {
	if err := d.sel.find(ctx, p, "sources.dialog", 0); err == nil {
		return d.rowCount(ctx, p) // already open
	}
	before, err := d.rowCount(ctx, p)
	if err != nil {
		return 0, err
	}
	err = d.press(ctx, p, "sources.add")
	var te *transport.Error
	if errors.As(err, &te) && te.Kind == transport.KindDrift {
		if _, err := d.open(ctx, p, nb, "addSource=true"); err != nil {
			return 0, err
		}
		d.setAbandoned(-1)
		if before, err = d.rowCount(ctx, p); err != nil {
			return 0, err
		}
		key, ferr := d.sel.findAny(ctx, p, []string{"sources.dialog", "sources.add"}, d.wait)
		var fe *transport.Error
		switch {
		case errors.As(ferr, &fe) && fe.Kind == transport.KindDrift:
			return 0, d.sel.drift("sources.add")
		case ferr != nil:
			return 0, ferr
		case key == "sources.dialog":
			return before, nil
		}
		err = d.press(ctx, p, "sources.add")
	}
	if err != nil {
		return 0, err
	}
	if err := d.sel.find(ctx, p, "sources.dialog", d.wait); err != nil {
		return 0, err
	}
	return before, nil
}

func (d *DOM) rowCount(ctx context.Context, p page) (int, error) {
	rows, _, err := d.sourceState(ctx, p)
	return rows, err
}

// sourceState counts the source rows and the visible (non-promo) dialogs.
func (d *DOM) sourceState(ctx context.Context, p page) (rows, dialogs int, err error) {
	arg, err := json.Marshal([][]string{d.sel["sources.row"], d.sel["sources.dialog"]})
	if err != nil {
		return 0, 0, err
	}
	raw, err := domEval(ctx, p, `((r, g) => [__sbDom.present(r).length, (__sbDom.dismissPromos(), __sbDom.all(g).length)])(...`+string(arg)+`)`)
	if err != nil {
		return 0, 0, err
	}
	var n []int
	if err := json.Unmarshal(raw, &n); err != nil || len(n) != 2 {
		return 0, 0, fmt.Errorf("browser: source state %s: %v", raw, err)
	}
	return n[0], n[1], nil
}
