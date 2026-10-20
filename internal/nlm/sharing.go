package nlm

import (
	"context"
	"encoding/json/jsontext"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// ShareUser is one collaborator row of a notebook's sharing state.
type ShareUser struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role"` // owner | editor | viewer | role-N
}

// ShareStatus is the sharing state of one notebook (JFMDGd).
type ShareStatus struct {
	Public bool        `json:"public"`
	Users  []ShareUser `json:"users"`
}

// ShareChange is one per-user change. Role is "editor", "viewer" or "remove".
type ShareChange struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// ShareUpdate is a sharing change. Public is nil for "leave as is". Users
// and the link setting go out as two separate QDyure calls.
type ShareUpdate struct {
	Public  *bool         `json:"public,omitempty"`
	Users   []ShareChange `json:"users,omitempty"`
	Notify  bool          `json:"notify,omitempty"`
	Message string        `json:"message,omitempty"`
}

// Permission codes: 1 owner (read only), 2 editor, 3 viewer, 4 remove.
func roleName(code int64) string {
	switch code {
	case 1:
		return "owner"
	case 2:
		return "editor"
	case 3:
		return "viewer"
	}
	return fmt.Sprintf("role-%d", code)
}

func roleCode(role string) (int, bool) {
	switch role {
	case "editor":
		return 2, true
	case "viewer":
		return 3, true
	case "remove":
		return 4, true
	}
	return 0, false
}

// Live capture 2026-10-13 (JFMDGd): params [notebook_id, HDR].
func getShareParams(nb string) []any { return []any{nb, requestOptions()} }

// shape: catalog consensus, not live-verified (QDyure, LabsTailwindSharingService).
// Link setting: [[[nb,null,[v],[v,""]]],1,null,[2]] with v 0 restricted / 1 anyone with the link.
func shareLinkParams(nb string, public bool) []any {
	v := 0
	if public {
		v = 1
	}
	return []any{[]any{[]any{nb, nil, []any{v}, []any{v, ""}}}, 1, nil, []any{2}}
}

// shape: catalog consensus, not live-verified.
// Users: [[[nb,[[email,null,perm]..],null,[msg_flag,msg]]],notify,null,[2]].
// msg_flag is 1 when a message is attached, else 0 (assumption: only the
// flag's presence is catalogued, its meaning is inferred).
func shareUsersParams(nb string, u ShareUpdate) ([]any, error) {
	rows := make([]any, 0, len(u.Users))
	for _, ch := range u.Users {
		if ch.Email == "" {
			return nil, fmt.Errorf("nlm: share change needs an email")
		}
		code, ok := roleCode(ch.Role)
		if !ok {
			return nil, fmt.Errorf("nlm: share role %q must be editor, viewer or remove", ch.Role)
		}
		rows = append(rows, []any{ch.Email, nil, code})
	}
	flag := 0
	if u.Message != "" {
		flag = 1
	}
	notify := 0
	if u.Notify {
		notify = 1
	}
	return []any{[]any{[]any{nb, rows, nil, []any{flag, u.Message}}}, notify, nil, []any{2}}, nil
}

// GetShare reads who a notebook is shared with and whether the link is public (JFMDGd).
// Response: [[ [email,perm,[],[name,avatar]]..], [isPublic], maxShare, allowed, ...].
// Live capture showed null (not [false]) at [1] for a private notebook.
func (c *Client) GetShare(ctx context.Context, nb string) (ShareStatus, error) {
	p, err := c.do(ctx, "sharing.details", getShareParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return ShareStatus{}, err
	}
	s := ShareStatus{Users: []ShareUser{}}
	for _, r := range wire.Elems(p, 0) {
		perm, _ := wire.Int(r, 1)
		s.Users = append(s.Users, ShareUser{Email: wire.Str(r, 0), Name: wire.Str(r, 3, 0), Role: roleName(perm)})
	}
	s.Public = boolAt(p, 1, 0) || boolAt(p, 1)
	return s, nil
}

func boolAt(v jsontext.Value, path ...int) bool {
	x, ok := wire.At(v, path...)
	return ok && string(x) == "true"
}

// UpdateShare changes link visibility and/or per-user permissions (QDyure).
// The payload shapes are catalog consensus and not live-verified. Never
// retried: a replayed share could notify people twice.
//
// The link and the users go out as two calls, so one can apply while the
// other fails. Both are always tried and the result has one outcome per
// part sent (link first); the returned error is only for invalid input,
// and then nothing is sent.
func (c *Client) UpdateShare(ctx context.Context, nb string, u ShareUpdate) ([]SharePart, error) {
	if u.Public == nil && len(u.Users) == 0 {
		return nil, fmt.Errorf("nlm: share update changes nothing")
	}
	var users []any
	if len(u.Users) > 0 {
		var err error
		if users, err = shareUsersParams(nb, u); err != nil { // validate before any call
			return nil, err
		}
	}
	opts := callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed}
	var out []SharePart
	send := func(part string, params []any) {
		r := SharePart{Part: part, Status: "applied"}
		if err := ctx.Err(); err != nil {
			r.Status, r.Err = "failed", err
		} else if _, err := c.do(ctx, "sharing.share", params, opts); err != nil {
			r.Status, r.Err = "failed", err
		}
		out = append(out, r)
	}
	if u.Public != nil {
		send("link", shareLinkParams(nb, *u.Public))
	}
	if users != nil {
		send("users", users)
	}
	return out, nil
}

// SharePart is the outcome of one of UpdateShare's calls.
type SharePart struct {
	Part   string // link | users
	Status string // applied | failed
	Err    error  // set when Status is "failed"
}
