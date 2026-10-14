package nlm

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// QuotaWindow is one usage window of the account's quota summary.
type QuotaWindow struct {
	Type    string    `json:"type"` // "5h", "weekly" or "type-N"
	ResetAt time.Time `json:"reset_at,omitzero"`
	UsedPct float64   `json:"used_pct"`
	LeftPct float64   `json:"left_pct"`
}

// Quota is the account's quota summary (EylDcb).
type Quota struct {
	Windows []QuotaWindow `json:"windows"`
}

// Live capture 2026-10-13 (EylDcb): params [HDR].
func quotaParams() []any { return []any{requestOptions()} }

// accountSettingParams wraps one settings entry the way the web app does
// (live capture 2026-10-13, hT54vc): [[[null,[[null,null,null,null,SLOT]]]],HDR].
// The live capture's SLOT was [null,null,null,[[2,2]]] (a setting other than language).
func accountSettingParams(slot any) []any {
	return []any{[]any{[]any{nil, []any{[]any{nil, nil, nil, nil, slot}}}}, requestOptions()}
}

func floatAt(v jsontext.Value, path ...int) (float64, bool) {
	x, ok := wire.At(v, path...)
	if !ok {
		return 0, false
	}
	var f float64
	if json.Unmarshal(x, &f) != nil {
		return 0, false
	}
	return f, true
}

// Window type codes seen live: 1 (resets within hours) and 2 (resets in about a week).
func quotaType(code int64) string {
	switch code {
	case 1:
		return "5h"
	case 2:
		return "weekly"
	}
	return fmt.Sprintf("type-%d", code)
}

// Quota reads the quota summary (EylDcb). Windows are at [1]; per window
// [4] type, [5] reset [s,ns], [6] percent used (null when unused), [7]
// percent left. Whichever of used/left is missing is derived from the other.
// The per-action cost table at [3] is not decoded.
func (c *Client) Quota(ctx context.Context) (Quota, error) {
	p, err := c.do(ctx, "account.quota", quotaParams(), callOpts{idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return Quota{}, err
	}
	q := Quota{Windows: []QuotaWindow{}}
	for _, r := range wire.Elems(p, 1) {
		typ, _ := wire.Int(r, 4)
		w := QuotaWindow{Type: quotaType(typ), ResetAt: stamp(r, 5)}
		used, hasUsed := floatAt(r, 6)
		left, hasLeft := floatAt(r, 7)
		switch {
		case hasUsed && hasLeft:
		case hasLeft:
			used = 100 - left
		case hasUsed:
			left = 100 - used
		default:
			left = 100
		}
		w.UsedPct, w.LeftPct = used, left
		q.Windows = append(q.Windows, w)
	}
	return q, nil
}

// SetLanguage sets the output language (hT54vc).
// shape: the envelope is live-verified; the language slot `[lang]` at the
// entry's index 4 is rpc-catalog (TL) only. The live capture sent a different
// setting there ([null,null,null,[[2,2]]]), so the language slot is not
// live-verified.
func (c *Client) SetLanguage(ctx context.Context, lang string) error {
	if lang == "" {
		return fmt.Errorf("nlm: language is empty")
	}
	_, err := c.do(ctx, "account.mutate", accountSettingParams([]any{lang}), callOpts{null: wire.NullAllowed})
	return err
}
