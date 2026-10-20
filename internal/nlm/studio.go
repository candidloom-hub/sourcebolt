package nlm

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// StudioKind names a Studio artifact kind.
type StudioKind string

const (
	StudioAudio             StudioKind = "audio"              // type code 1
	StudioReport            StudioKind = "report"             // type code 2
	StudioVideo             StudioKind = "video"              // type code 3
	StudioQuiz              StudioKind = "quiz"               // type code 4 (shared)
	StudioFlashcards        StudioKind = "flashcards"         // type code 4 (shared)
	StudioMindMap           StudioKind = "mind_map"           // type code 4 (shared, interactive)
	StudioInfographic       StudioKind = "infographic"        // type code 7
	StudioSlides            StudioKind = "slides"             // type code 8
	StudioDataTable         StudioKind = "data_table"         // type code 9
	StudioInteractiveReport StudioKind = "interactive_report" // type code 11

	// StudioInteractive is what a listing row of type code 4 decodes to:
	// quiz, flashcards and the interactive mind map share the code, and
	// telling them apart needs a live-capture position (rpc-catalog §10 item 29).
	StudioInteractive StudioKind = "interactive"
	// StudioUnknown is an unmapped type code.
	StudioUnknown StudioKind = "unknown"
)

// KindFromTypeCode maps the gArtLc row type code ([2]) to a kind.
func KindFromTypeCode(code int64) StudioKind {
	switch code {
	case 1:
		return StudioAudio
	case 2:
		return StudioReport
	case 3:
		return StudioVideo
	case 4:
		return StudioInteractive
	case 7:
		return StudioInfographic
	case 8:
		return StudioSlides
	case 9:
		return StudioDataTable
	case 11:
		return StudioInteractiveReport
	}
	return StudioUnknown
}

// ArtifactStatus is the generation state of an artifact.
type ArtifactStatus string

const (
	ArtifactPending   ArtifactStatus = "pending" // status 1, 2 or unknown
	ArtifactReady     ArtifactStatus = "ready"   // status 3
	ArtifactFailed    ArtifactStatus = "failed"  // status 4
	artifactSuggested ArtifactStatus = "suggested"
)

func statusFromCode(code int64) ArtifactStatus {
	switch code {
	case 3:
		return ArtifactReady
	case 4:
		return ArtifactFailed
	case 5:
		return artifactSuggested
	}
	return ArtifactPending
}

// Artifact is one Studio artifact in a notebook listing.
type Artifact struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Kind      StudioKind        `json:"kind"`
	Status    ArtifactStatus    `json:"status"`
	MediaURLs map[string]string `json:"media_urls,omitempty"`
	Created   time.Time         `json:"created,omitzero"`
}

// ArtifactContent is the interactive content of one artifact (v9rmvd).
type ArtifactContent struct {
	HTML string         `json:"html,omitempty"` // [0][9][0]
	Tree jsontext.Value `json:"tree,omitempty"` // mind-map tree, [0][9][3]
}

// Live capture 2026-10-13 (gArtLc): [artifactOptions, notebook_id, filter].
// The captured filter string was sanitized; the filter text is the catalog's.
func listArtifactsParams(nb string) []any {
	return []any{artifactOptions(), nb, `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`}
}

// ListArtifacts lists a notebook's Studio artifacts (gArtLc). Only the
// consensus row fields are decoded: id [0], title [1], type [2], status [4]
// (3 ready, 4 failed, 5 suggested) and created [15][0]. Suggested rows are
// dropped.
//
// TODO(rpc-catalog §10 item 29): MediaURLs stays empty. The media positions
// conflict between upstreams (video [8][3] vs [8][4], slides PDF/PPTX, report,
// data table) and need a live capture per type before they are pinned.
func (c *Client) ListArtifacts(ctx context.Context, nb string) ([]Artifact, error) {
	p, err := c.do(ctx, "studio.list", listArtifactsParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	out := []Artifact{}
	for _, r := range wire.Elems(p, 0) {
		id := wire.Str(r, 0)
		if id == "" {
			continue
		}
		code, _ := wire.Int(r, 4)
		st := statusFromCode(code)
		if st == artifactSuggested {
			continue
		}
		out = append(out, Artifact{
			ID: id, Title: wire.Str(r, 1), Kind: rowKind(r),
			Status: st, MediaURLs: map[string]string{}, Created: stamp(r, 15),
		})
	}
	return out, nil
}

// rowKind reads an artifact row's kind: the type code at [2], refined for
// the shared code 4 by the variant at [9][1][0]. Only variant 1
// (flashcards) is live-verified (capture #2: gArtLc, v9rmvd, R7cb6c);
// other variants stay StudioInteractive.
func rowKind(r []byte) StudioKind {
	typ, _ := wire.Int(r, 2)
	k := KindFromTypeCode(typ)
	if k == StudioInteractive {
		if v, _ := wire.Int(r, 9, 1, 0); v == flashcardsVariant {
			return StudioFlashcards
		}
	}
	return k
}

// getArtifactParams is pinned by live capture #2 (2026-10-20, v9rmvd):
// [artifact_id, artifactGetOptions()]. The web app polled it while a
// flashcards artifact generated; the answer is the artifact row (gArtLc
// layout) wrapped in a list.
func getArtifactParams(id string) []any { return []any{id, artifactGetOptions()} }

// GetArtifact reads an artifact's interactive content (v9rmvd). The request
// is live-verified; HTML at [0][9][0] and the mind-map tree at [0][9][3] are
// catalog consensus (the captured row was still generating and had "" at
// [0][9][0]).
func (c *Client) GetArtifact(ctx context.Context, id string) (ArtifactContent, error) {
	if id == "" {
		return ArtifactContent{}, fmt.Errorf("nlm: artifact id is empty")
	}
	p, err := c.do(ctx, "studio.get", getArtifactParams(id), callOpts{idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return ArtifactContent{}, err
	}
	ac := ArtifactContent{HTML: wire.Str(p, 0, 9, 0)}
	if t, ok := wire.At(p, 0, 9, 3); ok {
		ac.Tree = t
	}
	if ac.HTML == "" && ac.Tree == nil {
		return ArtifactContent{}, &transport.Error{Kind: transport.KindNotFound, RPC: "studio.get", Message: "artifact has no interactive content"}
	}
	return ac, nil
}

// CanGenerate is the quota pre-flight: false when any quota window has
// nothing left. The quota summary is returned either way.
func (c *Client) CanGenerate(ctx context.Context) (bool, Quota, error) {
	q, err := c.Quota(ctx)
	if err != nil {
		return false, Quota{}, err
	}
	for _, w := range q.Windows {
		if w.LeftPct == 0 {
			return false, q, nil
		}
	}
	return true, q, nil
}

// StudioRequest describes one artifact generation. Only Kind is used until
// the generate payloads are pinned.
type StudioRequest struct {
	Kind         StudioKind
	SourceIDs    []string
	Instructions string
	Language     string
	Quantity     string // flashcards: number of cards
	Difficulty   string // flashcards
}

// Flashcards options pinned by live capture #2 (2026-10-20): the UI choices
// "Fewer cards" and "Hard" sent 1 and 3. Other values (standard/more cards,
// easy/medium) have no live capture and are refused.
const flashcardsVariant = 1

var (
	flashcardQuantities   = map[string]int{"fewer": 1}
	flashcardDifficulties = map[string]int{"hard": 3}
	// knownFlashcard* are the UI choices; a known but unpinned value is
	// KindUnsupported, anything else KindInvalidArgument.
	knownFlashcardQuantities   = map[string]bool{"fewer": true, "standard": true, "more": true}
	knownFlashcardDifficulties = map[string]bool{"easy": true, "medium": true, "hard": true}
)

// Defaults for empty StudioRequest fields: the pinned flashcards options and
// the language the capture sent.
const (
	defaultFlashcardQuantity   = "fewer"
	defaultFlashcardDifficulty = "hard"
	defaultStudioLanguage      = "en"
)

// flashcardsParams is pinned by live capture #2 (2026-10-20, R7cb6c,
// Flashcards with "Fewer cards", "Hard", "Text only" and a topic):
//
//	[artifactOptions(), nb, [null, null, 4, [[[sid]]..], null×5,
//	  [null, [1, null, topic, lang, null, null, [quantity, difficulty, [1]]]]]]
//
// Type 4 with variant 1 is flashcards; the config sits at slot 9. The
// trailing [1] is the "Text only" choice, the only one captured. The web
// app also listed notes as sources ([null×6,[note_id,2]]); only sources are
// sent here.
func flashcardsParams(nb string, sources []string, topic, lang string, quantity, difficulty int) []any {
	srcs := make([]any, len(sources))
	for i, s := range sources {
		srcs[i] = []any{[]any{s}}
	}
	cfg := []any{nil, []any{flashcardsVariant, nil, topic, lang, nil, nil, []any{quantity, difficulty, []any{1}}}}
	return []any{artifactOptions(), nb, []any{nil, nil, 4, srcs, nil, nil, nil, nil, nil, cfg}}
}

// Generate starts one Studio artifact (R7cb6c, never retried) and returns
// it as pending; poll ListArtifacts for its status. Only flashcards are
// supported: their request is pinned by live capture #2. They need
// Instructions (the topic; a request without one was not captured) and
// accept only the captured options (Quantity "fewer", Difficulty "hard", the
// defaults). Language defaults to "en". Empty SourceIDs means every source
// of the notebook. Other kinds return KindUnsupported: their config slots
// and enums conflict between upstreams and have no live capture.
func (c *Client) Generate(ctx context.Context, nb string, r StudioRequest) (Artifact, error) {
	if r.Kind != StudioFlashcards {
		return Artifact{}, &transport.Error{
			Kind: transport.KindUnsupported, RPC: "studio.create",
			Message: fmt.Sprintf("generating %q artifacts is not supported yet: the request needs a live capture (rpc-catalog §10 items 24-32)", r.Kind),
		}
	}
	q, d := r.Quantity, r.Difficulty
	if q == "" {
		q = defaultFlashcardQuantity
	}
	if d == "" {
		d = defaultFlashcardDifficulty
	}
	if !knownFlashcardQuantities[q] {
		return Artifact{}, invalidArg("studio.create", fmt.Sprintf("flashcards quantity must be fewer, standard or more, got %q", q))
	}
	if !knownFlashcardDifficulties[d] {
		return Artifact{}, invalidArg("studio.create", fmt.Sprintf("flashcards difficulty must be easy, medium or hard, got %q", d))
	}
	qv, qok := flashcardQuantities[q]
	dv, dok := flashcardDifficulties[d]
	if !qok || !dok {
		return Artifact{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "studio.create",
			Message: fmt.Sprintf("flashcards with quantity %q and difficulty %q are not supported yet: only quantity fewer and difficulty hard are pinned by a live capture", q, d)}
	}
	if strings.TrimSpace(r.Instructions) == "" {
		return Artifact{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "studio.create",
			Message: "flashcards without instructions (a topic) are not supported yet: only a request with a topic is pinned by a live capture"}
	}
	lang := r.Language
	if lang == "" {
		lang = defaultStudioLanguage
	}
	sources := r.SourceIDs
	if len(sources) == 0 {
		ids, err := c.notebookSourceIDs(ctx, nb)
		if err != nil {
			return Artifact{}, err
		}
		if len(ids) == 0 {
			return Artifact{}, invalidArg("studio.create", "notebook has no sources")
		}
		sources = ids
	}
	p, err := c.do(ctx, "studio.create", flashcardsParams(nb, sources, r.Instructions, lang, qv, dv), callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed})
	if err != nil {
		return Artifact{}, err
	}
	row, ok := wire.At(p, 0)
	if !ok || wire.Str(row, 0) == "" {
		return Artifact{}, &transport.Error{Kind: transport.KindDrift, RPC: "studio.create", Message: "create answered without an artifact id"}
	}
	code, _ := wire.Int(row, 4)
	return Artifact{ID: wire.Str(row, 0), Title: wire.Str(row, 1), Kind: rowKind(row), Status: statusFromCode(code), MediaURLs: map[string]string{}}, nil
}
