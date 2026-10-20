package mcpserver

import (
	"context"
	"errors"
	"slices"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type StudioListIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Artifact string `json:"artifact,omitempty" jsonschema:"one artifact id or title; returns it with its interactive content when available"`
	Kind     string `json:"kind,omitempty" jsonschema:"filter by kind, for example audio, report, video"`
	Status   string `json:"status,omitempty" jsonschema:"filter by status: pending, ready or failed"`
}

type StudioListOut struct {
	Artifacts []nlm.Artifact       `json:"artifacts"`
	Total     int                  `json:"total"`
	Content   *nlm.ArtifactContent `json:"content,omitempty"`
}

type StudioGenerateIn struct {
	Notebook     string             `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Kind         string             `json:"kind" jsonschema:"audio, video, mind_map, report, quiz, flashcards, slide_deck, infographic, data_table or interactive_report"`
	Sources      []string           `json:"sources,omitempty" jsonschema:"source ids, id prefixes or titles; default all sources (notes cannot be used as sources)"`
	Instructions string             `json:"instructions,omitempty" jsonschema:"focus or style instructions; for flashcards the topic (required)"`
	Language     string             `json:"language,omitempty" jsonschema:"output language code (default en)"`
	Flashcards   *FlashcardsOptions `json:"flashcards,omitempty" jsonschema:"options for kind flashcards"`
}

// FlashcardsOptions are the flashcards choices of the Studio dialog. Only
// the values pinned by live capture are accepted; the others return
// UNSUPPORTED_BY_TRANSPORT.
type FlashcardsOptions struct {
	Quantity   string `json:"quantity,omitempty" jsonschema:"number of cards: fewer (default; standard and more are not available yet)"`
	Difficulty string `json:"difficulty,omitempty" jsonschema:"hard (default; easy and medium are not available yet)"`
}

var studioKinds = map[string]nlm.StudioKind{
	"audio": nlm.StudioAudio, "video": nlm.StudioVideo, "mind_map": nlm.StudioMindMap,
	"report": nlm.StudioReport, "quiz": nlm.StudioQuiz, "flashcards": nlm.StudioFlashcards,
	"slide_deck": nlm.StudioSlides, "infographic": nlm.StudioInfographic,
	"data_table": nlm.StudioDataTable, "interactive_report": nlm.StudioInteractiveReport,
}

func (s *server) registerStudio(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "studio_list",
		Description: "List a notebook's Studio artifacts (audio, reports, video, ...) with kind and status. Pass artifact to read one with its interactive content (HTML or mind-map tree) when Google has any.",
		Annotations: readOnly("List studio artifacts"),
	}, s.studioList)

	register(s, m, "core", &mcp.Tool{
		Name:        "studio_generate",
		Description: "Generate a Studio artifact. Checks the account quota first and returns a QUOTA error when it is used up. Only kind flashcards is available: it needs instructions (the topic) and accepts flashcards.quantity fewer and flashcards.difficulty hard (the defaults; the only options pinned by a live capture). Flashcards are built from the notebook's sources only, not notes (the web app can add notes; that is not supported). Returns the new artifact as pending; poll studio_list for its status. Other kinds and options return UNSUPPORTED_BY_TRANSPORT after the quota check. Not retried after sending.",
		Annotations: writeAnn("Generate studio artifact", false, false, false),
	}, s.studioGenerate)
}

func (s *server) studioList(ctx context.Context, in StudioListIn) (StudioListOut, error) {
	switch in.Status {
	case "", "pending", "ready", "failed":
	default:
		return StudioListOut{}, badInput("status must be pending, ready or failed, got %q", in.Status)
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return StudioListOut{}, err
	}
	all, err := s.NLM.ListArtifacts(ctx, nb)
	if err != nil {
		return StudioListOut{}, err
	}
	if in.Artifact != "" {
		items := make([]resolve.Item, len(all))
		for i, a := range all {
			items[i] = resolve.Item{ID: a.ID, Title: a.Title}
		}
		it, err := resolve.Resolve(in.Artifact, items)
		if err != nil {
			return StudioListOut{}, err
		}
		out := StudioListOut{Total: 1}
		for _, a := range all {
			if a.ID == it.ID {
				out.Artifacts = []nlm.Artifact{a}
			}
		}
		c, err := s.NLM.GetArtifact(ctx, it.ID)
		var te *transport.Error
		switch {
		case err == nil:
			out.Content = &c
		case errors.As(err, &te) && te.Kind == transport.KindNotFound: // no interactive content for this kind
		default:
			return StudioListOut{}, err
		}
		return out, nil
	}
	out := StudioListOut{Artifacts: []nlm.Artifact{}}
	for _, a := range all {
		if in.Kind != "" && string(a.Kind) != in.Kind {
			continue
		}
		if in.Status != "" && string(a.Status) != in.Status {
			continue
		}
		out.Artifacts = append(out.Artifacts, a)
	}
	out.Total = len(out.Artifacts)
	return out, nil
}

func (s *server) studioGenerate(ctx context.Context, in StudioGenerateIn) (nlm.Artifact, error) {
	kind, ok := studioKinds[in.Kind]
	if !ok {
		keys := make([]string, 0, len(studioKinds))
		for k := range studioKinds {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		return nlm.Artifact{}, badInput("kind must be one of %v, got %q", keys, in.Kind)
	}
	if in.Flashcards != nil && kind != nlm.StudioFlashcards {
		return nlm.Artifact{}, badInput("flashcards options apply to kind flashcards only, got kind %q", in.Kind)
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return nlm.Artifact{}, err
	}
	can, q, err := s.NLM.CanGenerate(ctx)
	if err != nil {
		return nlm.Artifact{}, err
	}
	if !can {
		msg := "quota used up"
		for _, w := range q.Windows {
			if w.LeftPct == 0 && !w.ResetAt.IsZero() {
				msg += "; the " + w.Type + " window resets at " + w.ResetAt.UTC().Format("2006-01-02 15:04 MST")
			}
		}
		return nlm.Artifact{}, &transport.Error{Kind: transport.KindQuota, RPC: "studio.create", Message: msg}
	}
	ids, err := s.lookupSourceIDs(ctx, nb, in.Sources)
	if err != nil {
		return nlm.Artifact{}, err
	}
	req := nlm.StudioRequest{Kind: kind, SourceIDs: ids, Instructions: in.Instructions, Language: in.Language}
	if in.Flashcards != nil {
		req.Quantity, req.Difficulty = in.Flashcards.Quantity, in.Flashcards.Difficulty
	}
	a, err := s.NLM.Generate(ctx, nb, req)
	s.invalidateNotebook(nb)
	return a, err
}
