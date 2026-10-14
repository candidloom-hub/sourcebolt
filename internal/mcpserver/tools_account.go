package mcpserver

import (
	"context"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AccountSettingsIn struct {
	OutputLanguage string `json:"output_language,omitempty" jsonschema:"language code to set, for example en or de; omit to only read"`
}

type AccountSettingsOut struct {
	Quota          nlm.Quota `json:"quota"`
	OutputLanguage string    `json:"output_language,omitempty"`
	LanguageSet    bool      `json:"language_set,omitempty"`
}

func (s *server) registerAccount(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "account_settings",
		Description: "Read the account's quota windows, or set the output language when output_language is given. The language setting slot is not live-verified (the call is sent, but Google may ignore or reject it).",
		Annotations: writeAnn("Account settings", false, true, false),
	}, s.accountSettings)
}

func (s *server) accountSettings(ctx context.Context, in AccountSettingsIn) (AccountSettingsOut, error) {
	out := AccountSettingsOut{}
	if in.OutputLanguage != "" {
		if err := s.NLM.SetLanguage(ctx, in.OutputLanguage); err != nil {
			return out, err
		}
		out.OutputLanguage, out.LanguageSet = in.OutputLanguage, true
	}
	q, err := s.NLM.Quota(ctx)
	if err != nil {
		return out, err
	}
	out.Quota = q
	return out, nil
}
