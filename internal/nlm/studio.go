package nlm

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
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
		typ, _ := wire.Int(r, 2)
		out = append(out, Artifact{
			ID: id, Title: wire.Str(r, 1), Kind: KindFromTypeCode(typ),
			Status: st, MediaURLs: map[string]string{}, Created: stamp(r, 15),
		})
	}
	return out, nil
}

// GetArtifact reads an artifact's interactive content (v9rmvd; catalog
// consensus, not live-verified): params [artifact_id], HTML at [0][9][0],
// mind-map tree at [0][9][3].
func (c *Client) GetArtifact(ctx context.Context, id string) (ArtifactContent, error) {
	if id == "" {
		return ArtifactContent{}, fmt.Errorf("nlm: artifact id is empty")
	}
	p, err := c.do(ctx, "studio.get", []any{id}, callOpts{idempotent: true, null: wire.NullAllowed})
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
}

// Generate is blocked: R7cb6c has no live fixture and its first element,
// per-kind config slots and enum values conflict between upstreams.
func (c *Client) Generate(_ context.Context, _ string, r StudioRequest) (Artifact, error) {
	return Artifact{}, &transport.Error{
		Kind: transport.KindUnsupported, RPC: "studio.create",
		Message: fmt.Sprintf("generating %q artifacts is not supported yet: the request needs a live capture (rpc-catalog §10 items 24-32)", r.Kind),
	}
}
