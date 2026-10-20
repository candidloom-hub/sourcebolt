package nlm

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// StudioRequest describes one artifact generation. Which option fields
// apply depends on Kind (see StudioOptions); an option of another kind is
// KindInvalidArgument. Empty options take the defaults in StudioOptions.
type StudioRequest struct {
	Kind      StudioKind
	SourceIDs []string // empty: every source of the notebook
	// Instructions is the free text of the dialog: the focus prompt (audio,
	// video), the report prompt, the slide deck or data table description,
	// the topic (mind map, quiz, flashcards). The infographic was captured
	// without one and refuses it.
	Instructions string
	Language     string // as the web app sends it, for example en or en_in; default en

	Format      string // audio, video, slide_deck
	Length      string // audio, slide_deck
	Quantity    string // quiz, flashcards
	Difficulty  string // quiz, flashcards
	Images      string // flashcards
	Orientation string // infographic
	Detail      string // infographic
	Style       string // infographic visual style
	Title       string // report
	Description string // report
	// GenerateLater is the web app's "Generate later" (deferred) choice:
	// video only, captured with the cinematic format.
	GenerateLater bool
}

// StudioOption is one option of a Studio kind: the values the web app
// offers (Known), the ones a live capture has sent (Proven, in UI order)
// and the value used when the option is empty (Default, always proven).
type StudioOption struct {
	Name    string
	Known   []string
	Proven  []string
	Default string
}

// studioOption pairs an option with its request field and wire values.
type studioOption struct {
	StudioOption
	wire  map[string]any
	field func(*StudioRequest) *string
}

// topicRule says whether a kind's free text (Instructions) is required,
// optional or refused, after what the captures sent.
type topicRule int

const (
	topicRequired topicRule = iota // only captured with free text
	topicOptional                  // captured with and without (sent as null)
	topicRefused                   // only captured without
)

// studioCfg is what a config builder needs besides the option values:
// the free text and language as sent (null when absent).
type studioCfg struct {
	topic, lang        any
	title, description string
}

// studioSpec is one generatable kind: its R7cb6c type code, the slot its
// config goes to, its options and the config builder.
type studioSpec struct {
	typ, slot int
	options   []studioOption
	topic     topicRule
	// build returns the config for the slot; v holds the wire value of
	// every option by name, src2 the sources as [[sid]..].
	build func(c studioCfg, v map[string]any, src2 []any) any
}

func opt(name string, known []string, def string, wire map[string]any, field func(*StudioRequest) *string) studioOption {
	proven := make([]string, 0, len(wire))
	for _, k := range known {
		if _, ok := wire[k]; ok {
			proven = append(proven, k)
		}
	}
	return studioOption{StudioOption{Name: name, Known: known, Proven: proven, Default: def}, wire, field}
}

var (
	fFormat      = func(r *StudioRequest) *string { return &r.Format }
	fLength      = func(r *StudioRequest) *string { return &r.Length }
	fQuantity    = func(r *StudioRequest) *string { return &r.Quantity }
	fDifficulty  = func(r *StudioRequest) *string { return &r.Difficulty }
	fImages      = func(r *StudioRequest) *string { return &r.Images }
	fOrientation = func(r *StudioRequest) *string { return &r.Orientation }
	fDetail      = func(r *StudioRequest) *string { return &r.Detail }
	fStyle       = func(r *StudioRequest) *string { return &r.Style }
)

// videoCinematic is the wire value of the cinematic video format; it was
// captured without a language (null) and with "Generate later".
const videoCinematic = 3

// studioSpecs holds every kind sourcebolt can generate. Each request shape
// and every wire value is pinned by a live capture: capture #3
// (2026-10-20, R7cb6c live3_*_01..09, one per kind, chosen options and
// free text), capture #3b (live3b_*_01..06, the web app's defaults and no
// free text) and, for flashcards "fewer", "hard" and text only, capture #2
// (R7cb6c live_2026-10-20_01). Defaults are the web app's defaults where
// #3b captured them. The option names the web app offers but no capture
// has sent are Known only and refused with KindUnsupported.
var studioSpecs = map[StudioKind]studioSpec{
	// Audio Overview, slot 6: [null, [focus, length, null, src2, lang, null, format]].
	// #3: Critique 3, Short 1, a focus; #3b: Deep dive 1, default length 2,
	// focus null. sqTeoe lists four audio formats (codes 1-4).
	StudioAudio: {typ: 1, slot: 6, topic: topicOptional, options: []studioOption{
		opt("format", []string{"deep_dive", "brief", "critique", "debate"}, "deep_dive", map[string]any{"deep_dive": 1, "critique": 3}, fFormat),
		opt("length", []string{"short", "default", "long"}, "default", map[string]any{"short": 1, "default": 2}, fLength),
	}, build: func(c studioCfg, v map[string]any, src2 []any) any {
		return []any{nil, []any{c.topic, v["length"], nil, src2, c.lang, nil, v["format"]}}
	}},
	// Report (format Document), slot 7: [null, [title, description, null,
	// src2, lang, prompt]]. #3 sent the "Study guide" template's texts; the
	// templates are client-side texts, so the caller passes title,
	// description and prompt (for example from suggest surface report).
	StudioReport: {typ: 2, slot: 7, topic: topicRequired, build: func(c studioCfg, _ map[string]any, src2 []any) any {
		return []any{nil, []any{c.title, c.description, nil, src2, c.lang, c.topic}}
	}},
	// Video Overview, slot 8: [null, null, [src2, lang, focus, null, format]].
	// #3: Short 4 with en_in and a focus; #3b: Cinematic 3 (the default),
	// language and focus null, sent with "Generate later". sqTeoe lists four
	// video formats.
	StudioVideo: {typ: 3, slot: 8, topic: topicOptional, options: []studioOption{
		opt("format", []string{"explainer", "brief", "cinematic", "short"}, "cinematic", map[string]any{"cinematic": videoCinematic, "short": 4}, fFormat),
	}, build: func(c studioCfg, v map[string]any, src2 []any) any {
		return []any{nil, nil, []any{src2, c.lang, c.topic, nil, v["format"]}}
	}},
	// Mind map, slot 9 variant 4: [null, [4, null, topic, lang]] (#3).
	StudioMindMap: {typ: 4, slot: 9, topic: topicRequired, build: func(c studioCfg, _ map[string]any, _ []any) any {
		return []any{nil, []any{mindMapVariant, nil, c.topic, c.lang}}
	}},
	// Quiz, slot 9 variant 2: [null, [2, null, topic, lang, null, null, null,
	// [quantity, difficulty]]]. #3: More 3, Easy 1, a topic; #3b: Standard 2,
	// Medium 2, topic null.
	StudioQuiz: {typ: 4, slot: 9, topic: topicOptional, options: []studioOption{
		opt("quantity", []string{"fewer", "standard", "more"}, "standard", map[string]any{"standard": 2, "more": 3}, fQuantity),
		opt("difficulty", []string{"easy", "medium", "hard"}, "medium", map[string]any{"easy": 1, "medium": 2}, fDifficulty),
	}, build: func(c studioCfg, v map[string]any, _ []any) any {
		return []any{nil, []any{quizVariant, nil, c.topic, c.lang, nil, nil, nil, []any{v["quantity"], v["difficulty"]}}}
	}},
	// Flashcards, slot 9 variant 1: [null, [1, null, topic, lang, null, null,
	// [quantity, difficulty, images]]]. #2: Fewer 1, Hard 3, Text only [1];
	// #3 (the web app's defaults): Standard 2, Medium 2, with images [].
	StudioFlashcards: {typ: 4, slot: 9, topic: topicRequired, options: []studioOption{
		opt("quantity", []string{"fewer", "standard", "more"}, "standard", map[string]any{"fewer": 1, "standard": 2}, fQuantity),
		opt("difficulty", []string{"easy", "medium", "hard"}, "medium", map[string]any{"medium": 2, "hard": 3}, fDifficulty),
		opt("images", []string{"include", "text_only"}, "include", map[string]any{"include": []any{}, "text_only": []any{1}}, fImages),
	}, build: func(c studioCfg, v map[string]any, _ []any) any {
		return []any{nil, []any{flashcardsVariant, nil, c.topic, c.lang, nil, nil, []any{v["quantity"], v["difficulty"], v["images"]}}}
	}},
	// Infographic, slot 14: [[null, lang, null, orientation, detail, style]].
	// #3: Portrait 2, Concise 1, Sketch note 2; #3b: Landscape 1, Standard 2,
	// Auto-select 1. Both without a description.
	StudioInfographic: {typ: 7, slot: 14, topic: topicRefused, options: []studioOption{
		opt("orientation", []string{"landscape", "portrait", "square"}, "landscape", map[string]any{"landscape": 1, "portrait": 2}, fOrientation),
		opt("detail", []string{"concise", "standard", "detailed"}, "standard", map[string]any{"concise": 1, "standard": 2}, fDetail),
		opt("style", []string{"auto", "sketch_note"}, "auto", map[string]any{"auto": 1, "sketch_note": 2}, fStyle),
	}, build: func(c studioCfg, v map[string]any, _ []any) any {
		return []any{[]any{nil, c.lang, nil, v["orientation"], v["detail"], v["style"]}}
	}},
	// Slide deck, slot 16: [[description, lang, format, length]]. #3:
	// Presenter slides 2, Short 2, a description; #3b: Detailed deck 1,
	// default length 3, no description.
	StudioSlides: {typ: 8, slot: 16, topic: topicOptional, options: []studioOption{
		opt("format", []string{"detailed_deck", "presenter_slides"}, "detailed_deck", map[string]any{"detailed_deck": 1, "presenter_slides": 2}, fFormat),
		opt("length", []string{"default", "short"}, "default", map[string]any{"default": 3, "short": 2}, fLength),
	}, build: func(c studioCfg, v map[string]any, _ []any) any {
		return []any{[]any{c.topic, c.lang, v["format"], v["length"]}}
	}},
	// Data table, slot 18: [null, [description, lang]] (#3).
	StudioDataTable: {typ: 9, slot: 18, topic: topicRequired, build: func(c studioCfg, _ map[string]any, _ []any) any {
		return []any{nil, []any{c.topic, c.lang}}
	}},
	// Interactive report (the report dialog's Interactive format), type 11,
	// slot 34: [null, [null, lang]] (#3b, template "Learning overview",
	// which has no wire value of its own; no free text).
	StudioInteractiveReport: {typ: 11, slot: 34, topic: topicRefused, build: func(c studioCfg, _ map[string]any, _ []any) any {
		return []any{nil, []any{nil, c.lang}}
	}},
}

// allStudioOptions names every option field, to refuse those of another kind.
var allStudioOptions = []struct {
	name  string
	field func(*StudioRequest) *string
}{
	{"format", fFormat}, {"length", fLength}, {"quantity", fQuantity}, {"difficulty", fDifficulty},
	{"images", fImages}, {"orientation", fOrientation}, {"detail", fDetail}, {"style", fStyle},
	{"title", func(r *StudioRequest) *string { return &r.Title }},
	{"description", func(r *StudioRequest) *string { return &r.Description }},
}

// StudioOptions returns the options of a generatable kind (nil for a kind
// without options) and whether the kind can be generated at all.
func StudioOptions(k StudioKind) ([]StudioOption, bool) {
	s, ok := studioSpecs[k]
	if !ok {
		return nil, false
	}
	var out []StudioOption
	for _, o := range s.options {
		out = append(out, o.StudioOption)
	}
	return out, true
}

// GeneratableKinds lists the kinds Generate supports, sorted.
func GeneratableKinds() []StudioKind {
	out := make([]StudioKind, 0, len(studioSpecs))
	for k := range studioSpecs {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

const defaultStudioLanguage = "en"

// Variants of type code 4 at config [1][0] and listing row [9][1][0].
const (
	flashcardsVariant = 1 // captures #2 and #3
	quizVariant       = 2 // capture #3
	mindMapVariant    = 4 // capture #3
)

// studioParams validates r against its kind's spec and builds the R7cb6c
// request [ARTIFACT_OPTS, nb, [null, null, type, src3, null.., config]]
// with the config at the kind's slot.
func studioParams(nb string, r StudioRequest) ([]any, error) {
	const rpc = "studio.create"
	spec, ok := studioSpecs[r.Kind]
	if !ok {
		return nil, invalidArg(rpc, fmt.Sprintf("kind %q cannot be generated; use one of %v", r.Kind, GeneratableKinds()))
	}
	own := map[string]bool{}
	for _, o := range spec.options {
		own[o.Name] = true
	}
	if r.Kind == StudioReport {
		own["title"], own["description"] = true, true
	}
	for _, o := range allStudioOptions {
		if !own[o.name] && *o.field(&r) != "" {
			return nil, invalidArg(rpc, fmt.Sprintf("option %s does not apply to %s", o.name, r.Kind))
		}
	}
	v := map[string]any{}
	for _, o := range spec.options {
		val := *o.field(&r)
		if val == "" {
			val = o.Default
		}
		w, ok := o.wire[val]
		switch {
		case ok:
			v[o.Name] = w
		case slices.Contains(o.Known, val):
			return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: rpc, Message: fmt.Sprintf(
				"%s option %s %q is not supported yet: only %s is pinned by a live capture", r.Kind, o.Name, val, strings.Join(o.Proven, ", "))}
		default:
			return nil, invalidArg(rpc, fmt.Sprintf("%s option %s must be one of %s, got %q", r.Kind, o.Name, strings.Join(o.Known, ", "), val))
		}
	}
	hasTopic := strings.TrimSpace(r.Instructions) != ""
	switch {
	case spec.topic == topicRequired && !hasTopic:
		return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: rpc, Message: fmt.Sprintf(
			"%s without instructions is not supported yet: only a request with them is pinned by a live capture", r.Kind)}
	case spec.topic == topicRefused && hasTopic:
		return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: rpc, Message: fmt.Sprintf(
			"%s with instructions is not supported yet: only a request without them is pinned by a live capture", r.Kind)}
	}
	if r.Kind == StudioReport {
		for _, f := range []struct{ name, val string }{{"title", r.Title}, {"description", r.Description}} {
			if strings.TrimSpace(f.val) == "" {
				return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: rpc, Message: fmt.Sprintf(
					"report without a %s is not supported yet: only a request with title, description and instructions is pinned by a live capture", f.name)}
			}
		}
	}
	if r.GenerateLater {
		switch {
		case r.Kind != StudioVideo:
			return nil, invalidArg(rpc, fmt.Sprintf("option generate_later applies to video only, got %s", r.Kind))
		case v["format"] != videoCinematic:
			return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: rpc, Message: "video option generate_later with a format other than cinematic is not supported yet: only cinematic is pinned by a live capture"}
		}
	}
	c := studioCfg{title: r.Title, description: r.Description}
	if hasTopic {
		c.topic = r.Instructions
	}
	switch {
	case r.Kind == StudioVideo && v["format"] == videoCinematic:
		// Captured with a null language; sending one is unproven.
		if r.Language != "" {
			return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: rpc, Message: "video option language with format cinematic is not supported yet: the capture sent no language"}
		}
	case r.Language == "":
		c.lang = defaultStudioLanguage
	default:
		c.lang = r.Language
	}
	src2 := make([]any, len(r.SourceIDs))
	src3 := make([]any, len(r.SourceIDs))
	for i, s := range r.SourceIDs {
		src2[i] = []any{s}
		src3[i] = []any{[]any{s}}
	}
	spec3 := make([]any, spec.slot+1)
	spec3[2], spec3[3] = spec.typ, src3
	spec3[spec.slot] = spec.build(c, v, src2)
	if r.GenerateLater {
		// "Generate later" (capture #3b): [1] at params[6]; the answer row
		// then has status 7.
		return []any{artifactOptions(), nb, spec3, nil, nil, nil, []any{1}}, nil
	}
	return []any{artifactOptions(), nb, spec3}, nil
}

// ValidateStudioRequest reports whether Generate would refuse r before any
// call (kind, options, free text); sources are not checked.
func ValidateStudioRequest(r StudioRequest) error {
	_, err := studioParams("", r)
	return err
}

// Generate starts one Studio artifact (R7cb6c, never retried) and returns
// it as pending; poll ListArtifacts for its status. Every kind in
// studioSpecs is supported with the option values a live capture has
// proven; other values are refused before any call (see studioParams).
// Empty SourceIDs means every source of the notebook; notes are never sent
// as sources (the web app can add them; that is not captured for
// sourcebolt).
func (c *Client) Generate(ctx context.Context, nb string, r StudioRequest) (Artifact, error) {
	if _, err := studioParams(nb, r); err != nil {
		return Artifact{}, err
	}
	if len(r.SourceIDs) == 0 {
		ids, err := c.notebookSourceIDs(ctx, nb)
		if err != nil {
			return Artifact{}, err
		}
		if len(ids) == 0 {
			return Artifact{}, invalidArg("studio.create", "notebook has no sources")
		}
		r.SourceIDs = ids
	}
	params, err := studioParams(nb, r)
	if err != nil {
		return Artifact{}, err
	}
	p, err := c.do(ctx, "studio.create", params, callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed})
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
