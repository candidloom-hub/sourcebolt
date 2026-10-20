package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type StudioListIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Artifact string `json:"artifact,omitempty" jsonschema:"one artifact id or title; returns it with its interactive content when available"`
	Kind     string `json:"kind,omitempty" jsonschema:"filter by kind: audio, video, report, slide_deck, mind_map, quiz, flashcards, infographic, data_table (interactive and unknown for unrecognized rows)"`
	Status   string `json:"status,omitempty" jsonschema:"filter by status: pending (queued or generating), ready or failed"`
}

type StudioListOut struct {
	Artifacts []nlm.Artifact       `json:"artifacts"`
	Total     int                  `json:"total"`
	Content   *nlm.ArtifactContent `json:"content,omitempty"`
}

type StudioGenerateIn struct {
	Notebook     string   `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Kind         string   `json:"kind" jsonschema:"the artifact kind"`
	Sources      []string `json:"sources,omitempty" jsonschema:"source ids, id prefixes or titles; default all sources (notes cannot be used as sources)"`
	Instructions string   `json:"instructions,omitempty" jsonschema:"required free text except for infographic (which refuses it): audio and video focus, report prompt, slide deck and data table description, mind map, quiz and flashcards topic"`
	Language     string   `json:"language,omitempty" jsonschema:"output language code as the web app sends it, for example en or en_in (default en)"`

	Audio       *AudioOptions       `json:"audio,omitempty" jsonschema:"options for kind audio"`
	Video       *VideoOptions       `json:"video,omitempty" jsonschema:"options for kind video"`
	Report      *ReportOptions      `json:"report,omitempty" jsonschema:"options for kind report (required)"`
	SlideDeck   *SlideDeckOptions   `json:"slide_deck,omitempty" jsonschema:"options for kind slide_deck"`
	Quiz        *QuizOptions        `json:"quiz,omitempty" jsonschema:"options for kind quiz"`
	Flashcards  *FlashcardsOptions  `json:"flashcards,omitempty" jsonschema:"options for kind flashcards"`
	Infographic *InfographicOptions `json:"infographic,omitempty" jsonschema:"options for kind infographic"`
}

// The option objects of studio_generate. Their schema enums list only the
// values pinned by a live capture (filled in from nlm.StudioOptions); other
// values the web app offers return UNSUPPORTED_BY_TRANSPORT.
type AudioOptions struct {
	Format string `json:"format,omitempty" jsonschema:"audio format"`
	Length string `json:"length,omitempty" jsonschema:"audio length"`
}

type VideoOptions struct {
	Format string `json:"format,omitempty" jsonschema:"video format"`
}

type ReportOptions struct {
	Title       string `json:"title,omitempty" jsonschema:"report title (required), for example from suggest surface report"`
	Description string `json:"description,omitempty" jsonschema:"report description (required), for example from suggest surface report"`
}

type SlideDeckOptions struct {
	Format string `json:"format,omitempty" jsonschema:"slide deck format"`
	Length string `json:"length,omitempty" jsonschema:"slide deck length"`
}

type QuizOptions struct {
	Quantity   string `json:"quantity,omitempty" jsonschema:"number of questions"`
	Difficulty string `json:"difficulty,omitempty" jsonschema:"question difficulty"`
}

type FlashcardsOptions struct {
	Quantity   string `json:"quantity,omitempty" jsonschema:"number of cards"`
	Difficulty string `json:"difficulty,omitempty" jsonschema:"card difficulty"`
	Images     string `json:"images,omitempty" jsonschema:"card images"`
}

type InfographicOptions struct {
	Orientation string `json:"orientation,omitempty" jsonschema:"page orientation"`
	Detail      string `json:"detail,omitempty" jsonschema:"level of detail"`
	Style       string `json:"style,omitempty" jsonschema:"visual style"`
}

// studioOptionObjects maps each option object's property name to its kind.
var studioOptionObjects = map[string]nlm.StudioKind{
	"audio": nlm.StudioAudio, "video": nlm.StudioVideo, "slide_deck": nlm.StudioSlides,
	"quiz": nlm.StudioQuiz, "flashcards": nlm.StudioFlashcards, "infographic": nlm.StudioInfographic,
}

// studioGenerateSchema is StudioGenerateIn's schema with enums for kind
// and every option: only live-proven values, with the default named in the
// description. Input is still validated against the plain type schema, so
// an unproven value reaches the handler and gets UNSUPPORTED_BY_TRANSPORT
// naming the option instead of a schema error.
func studioGenerateSchema() *jsonschema.Schema {
	s, _ := schemaFor[StudioGenerateIn]()
	for _, k := range nlm.GeneratableKinds() {
		s.Properties["kind"].Enum = append(s.Properties["kind"].Enum, string(k))
	}
	for prop, kind := range studioOptionObjects {
		opts, _ := nlm.StudioOptions(kind)
		obj := s.Properties[prop]
		for _, o := range opts {
			p := obj.Properties[o.Name]
			for _, v := range o.Proven {
				p.Enum = append(p.Enum, v)
			}
			p.Description = fmt.Sprintf("%s: %s (default %s)", p.Description, strings.Join(o.Proven, ", "), o.Default)
		}
	}
	return s
}

func (s *server) registerStudio(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "studio_list",
		Description: "List a notebook's Studio artifacts (audio, video, report, slide_deck, mind_map, quiz, flashcards, infographic, data_table) with kind and status (pending while queued or generating, then ready or failed). Pass artifact to read one with its interactive content (HTML or mind-map tree) when Google has any.",
		Annotations: readOnly("List studio artifacts"),
	}, s.studioList)

	register(s, m, "core", &mcp.Tool{
		Name: "studio_generate",
		Description: "Generate a Studio artifact: audio, video, report, slide_deck, mind_map, quiz, flashcards, infographic or data_table. " +
			"Each kind takes only the option values a live capture has proven (the enums); other values the web app offers return UNSUPPORTED_BY_TRANSPORT naming the option, before any call. " +
			"instructions is required for every kind except infographic, which was captured without a description and refuses one. " +
			"report needs report.title and report.description plus instructions as the prompt (suggest surface report returns ready-made triples). " +
			"Checks the account quota first and returns a QUOTA error when it is used up. Artifacts are built from the notebook's sources only, not notes (the web app can add notes; that is not supported). " +
			"Returns the new artifact as pending; poll studio_list for its status. Not retried after sending.",
		Annotations: writeAnn("Generate studio artifact", false, false, false),
		InputSchema: studioGenerateSchema(),
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

// studioRequest maps the tool input to an nlm request; an option object of
// another kind is invalid input.
func studioRequest(in StudioGenerateIn) (nlm.StudioRequest, error) {
	r := nlm.StudioRequest{Kind: nlm.StudioKind(in.Kind), Instructions: in.Instructions, Language: in.Language}
	given := map[string]bool{
		"audio": in.Audio != nil, "video": in.Video != nil, "report": in.Report != nil, "slide_deck": in.SlideDeck != nil,
		"quiz": in.Quiz != nil, "flashcards": in.Flashcards != nil, "infographic": in.Infographic != nil,
	}
	for name, ok := range given {
		if ok && name != in.Kind {
			return r, badInput("%s options apply to kind %s only, got kind %q", name, name, in.Kind)
		}
	}
	switch {
	case in.Audio != nil:
		r.Format, r.Length = in.Audio.Format, in.Audio.Length
	case in.Video != nil:
		r.Format = in.Video.Format
	case in.Report != nil:
		r.Title, r.Description = in.Report.Title, in.Report.Description
	case in.SlideDeck != nil:
		r.Format, r.Length = in.SlideDeck.Format, in.SlideDeck.Length
	case in.Quiz != nil:
		r.Quantity, r.Difficulty = in.Quiz.Quantity, in.Quiz.Difficulty
	case in.Flashcards != nil:
		r.Quantity, r.Difficulty, r.Images = in.Flashcards.Quantity, in.Flashcards.Difficulty, in.Flashcards.Images
	case in.Infographic != nil:
		r.Orientation, r.Detail, r.Style = in.Infographic.Orientation, in.Infographic.Detail, in.Infographic.Style
	}
	return r, nil
}

func (s *server) studioGenerate(ctx context.Context, in StudioGenerateIn) (nlm.Artifact, error) {
	req, err := studioRequest(in)
	if err != nil {
		return nlm.Artifact{}, err
	}
	// Refuse unproven requests before any call, the quota check included.
	if err := nlm.ValidateStudioRequest(req); err != nil {
		return nlm.Artifact{}, err
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
	req.SourceIDs = ids
	a, err := s.NLM.Generate(ctx, nb, req)
	s.invalidateNotebook(nb)
	return a, err
}
