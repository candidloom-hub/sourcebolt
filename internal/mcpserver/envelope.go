package mcpserver

import (
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// Envelope is the error body of every failed tool call. It is sent as the
// JSON text content of an IsError result, so agents can branch on Code.
type Envelope struct {
	Status      string   `json:"status"`
	Error       string   `json:"error"`
	Code        string   `json:"code"`
	Retriable   bool     `json:"retriable"`
	Guidance    string   `json:"guidance,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Transport   string   `json:"transport,omitempty"`
}

// envelopeError carries an Envelope through the SDK's error path.
type envelopeError struct{ env Envelope }

func (e *envelopeError) Error() string {
	b, _ := json.Marshal(e.env)
	return string(b)
}

var guidance = map[transport.Kind]string{
	transport.KindAuth:          "The Google session is missing or expired. Run: sourcebolt login --import <cookies file>",
	transport.KindCookieBinding: "Google rejected the cookies for this client. Sign in again with: sourcebolt login --import <fresh cookies file>",
	transport.KindDrift:         "NotebookLM changed its web API. Update sourcebolt, or override RPC ids with NOTEBOOKLM_RPC_IDS.",
	transport.KindTransport:     "Network or Google server error. Retry shortly.",
	transport.KindRateLimited:   "Google is rate limiting this account. Wait before retrying.",
	transport.KindQuota:         "The account's NotebookLM quota is used up. It resets later; check server_info include_usage.",
	transport.KindPermission:    "This account cannot access that notebook or action.",
	transport.KindNotFound:      "The item does not exist or was deleted. List again to get current ids.",
	transport.KindPending:       "Google accepted the request but has not confirmed it. List the items before retrying.",
	transport.KindUnsupported:   "The active transport cannot do this.",
}

// toEnvelope converts any handler error into the error envelope.
func toEnvelope(err error) error {
	env := Envelope{Status: "error", Error: err.Error(), Code: "INTERNAL"}
	var te *transport.Error
	var amb *resolve.ErrAmbiguous
	var nf *resolve.ErrNotFound
	switch {
	case errors.As(err, &te):
		env.Code, env.Retriable, env.Transport = te.Kind.Code(), te.Retriable, te.Transport
		env.Guidance = guidance[te.Kind]
	case errors.As(err, &amb):
		env.Code = "AMBIGUOUS"
		env.Guidance = "Several items match. Pass an id or the exact title."
		for _, c := range amb.Candidates {
			env.Suggestions = append(env.Suggestions, fmt.Sprintf("%s (%s)", c.Title, c.ID))
		}
	case errors.As(err, &nf):
		env.Code = "NOT_FOUND"
		env.Guidance = "No item matches. Check the spelling or list items first."
		for _, c := range nf.Suggestions {
			env.Suggestions = append(env.Suggestions, fmt.Sprintf("%s (%s)", c.Title, c.ID))
		}
	case errors.As(err, new(*inputError)):
		env.Code = "INVALID_ARGUMENT"
	}
	return &envelopeError{env: env}
}

// inputError marks a bad tool argument detected by a handler.
type inputError struct{ msg string }

func (e *inputError) Error() string { return e.msg }

func badInput(format string, a ...any) error { return &inputError{fmt.Sprintf(format, a...)} }
