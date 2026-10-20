package wire

import (
	"bufio"
	"bytes"
	"encoding/json/v2"
	"io"
)

// Snapshot is one cumulative state of a streamed chat answer. Every snapshot
// holds the whole text so far, so the last one wins; there is no diffing.
type Snapshot struct {
	Text     string
	Thinking bool // type flag 2: reasoning chunk, not part of the answer
	// Final is the server's own end-of-answer marker (isFinalResponse,
	// payload position 4). It is set on the snapshot that carries it, which
	// is the last answer snapshot before the trailers, and on a rejection.
	// A stream cut short, or one from a server that omits the flag, never
	// sees Final; LastAnswer then falls back to the last non-thinking
	// snapshot.
	Final     bool
	Citations []Citation // re-derived from this snapshot only; ranges may be rewritten later
	// ConversationID is the id the stream carries. Live captures say this is
	// a per-stream session id; resolve the real conversation through hPTbtc.
	ConversationID string
	ResponseID     string
	Status         *Status // google.rpc.Status of a rejection (null answer + status)
}

// Citation ties a "[N]" marker to a passage of a source. Start and End are
// offsets into the answer text in UTF-16 code units (use SliceUTF16).
type Citation struct {
	N        int // marker number, 1-based; passage N-1 backs it
	SourceID string
	Text     string // cited passage text
	Start    int
	End      int
}

const (
	maxSegment       = 64 << 20 // one snapshot is ~200 KB live; this is a hostile-input cap
	maxCitationRange = 1024     // passage indices read per citation marker
	readBlock        = 32 << 10
)

type streamItem struct {
	snap Snapshot
	err  error
}

// StreamDecoder parses a GenerateFreeFormStreamed body incrementally. It holds
// one top-level segment at a time (about one snapshot), never the whole body.
type StreamDecoder struct {
	br  *bufio.Reader
	blk []byte
	pos int
	end int

	seg   []byte
	ubuf  [2][]byte
	depth int
	inStr bool
	esc   bool

	head     []byte // first bytes outside any segment: numeric-body check
	headSeen bool
	sawSeg   bool
	bad      int
	total    int
	tr       Trailers
	queue    []streamItem
	qi       int
	err      error
}

// NewStreamDecoder reads the stream from r.
func NewStreamDecoder(r io.Reader) *StreamDecoder {
	return &StreamDecoder{br: bufio.NewReaderSize(r, readBlock), blk: make([]byte, readBlock)}
}

// Trailers returns metadata seen so far (rotated f.sid). Complete after io.EOF.
func (d *StreamDecoder) Trailers() Trailers { return d.tr }

// Next returns the next snapshot. It returns io.EOF after the last one. A
// *Error ends the stream (truncated body, server error frame, sign-in page);
// snapshots returned before it stay valid. Heartbeat chunks are skipped.
func (d *StreamDecoder) Next() (Snapshot, error) {
	for {
		if d.qi < len(d.queue) {
			it := d.queue[d.qi]
			d.qi++
			if d.qi == len(d.queue) {
				d.queue, d.qi = d.queue[:0], 0
			}
			if it.err != nil {
				d.err = it.err
				d.queue, d.qi = nil, 0
				return Snapshot{}, it.err
			}
			return it.snap, nil
		}
		if d.err != nil {
			return Snapshot{}, d.err
		}
		d.fill()
	}
}

// fill reads one segment and queues its snapshots, or records the terminal error.
func (d *StreamDecoder) fill() {
	seg, err := d.nextSegment()
	if err != nil {
		d.err = d.finish(err)
		return
	}
	d.total++
	d.sawSeg = true
	rows, ok := splitElems(seg)
	if !ok {
		d.bad++
		return
	}
	for _, r := range rows {
		d.row(r)
	}
}

// finish turns the reader's terminal condition into the sticky decoder error.
func (d *StreamDecoder) finish(err error) error {
	if err != io.EOF {
		return err
	}
	switch {
	case d.depth > 0:
		return &Error{Malformed: true, Msg: "unterminated segment"}
	case d.bad > 0 && d.bad*10 > d.total:
		return &Error{Malformed: true, Msg: "unparseable chunks"}
	case !d.sawSeg:
		if n, ok := numericBody(d.head); ok {
			return &Error{Code: n, Msg: "numeric error body"}
		}
		return &Error{SignedOut: true, Msg: "no frames"}
	}
	return io.EOF
}

// nextSegment returns the next balanced top-level [...] segment with raw
// control characters escaped. The returned slice is valid until the next call.
func (d *StreamDecoder) nextSegment() ([]byte, error) {
	for {
		if d.pos == d.end {
			n, err := d.br.Read(d.blk)
			d.pos, d.end = 0, n
			if n == 0 {
				if err == nil {
					continue
				}
				return nil, err
			}
		}
		b := d.blk[d.pos:d.end]
		from := -1
		if d.depth > 0 {
			from = 0
		}
		for i := 0; i < len(b); i++ {
			c := b[i]
			if d.inStr {
				switch {
				case d.esc:
					d.esc = false
				case c == '\\':
					d.esc = true
				case c == '"':
					d.inStr = false
				}
				continue
			}
			switch c {
			case '"':
				if d.depth > 0 {
					d.inStr = true
				}
			case '[':
				if d.depth == 0 {
					d.seg = d.seg[:0]
					from = i
				}
				d.depth++
			case ']':
				if d.depth > 0 {
					d.depth--
					if d.depth == 0 {
						d.seg = append(d.seg, b[from:i+1]...)
						d.pos += i + 1
						return escapeControl(d.seg), nil
					}
				}
			default:
				if d.depth == 0 {
					if herr := d.outside(c); herr != nil {
						return nil, herr
					}
				}
			}
		}
		if from >= 0 {
			d.seg = append(d.seg, b[from:]...)
			if len(d.seg) > maxSegment {
				return nil, &Error{Malformed: true, Msg: "segment too large"}
			}
		}
		d.pos = d.end
	}
}

// outside inspects bytes between segments: an HTML page is a sign-in wall;
// a bare number is an error code.
func (d *StreamDecoder) outside(c byte) error {
	if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
		return nil
	}
	if !d.headSeen {
		d.headSeen = true
		if c == '<' {
			return &Error{SignedOut: true, Msg: "html page"}
		}
	}
	if len(d.head) < 32 && !d.sawSeg && c != ')' && c != '}' && c != '\x27' {
		d.head = append(d.head, c)
	}
	return nil
}

func (d *StreamDecoder) row(r []byte) {
	cols, ok := splitElems(r)
	if !ok || len(cols) == 0 {
		return
	}
	var tag string
	if json.Unmarshal(cols[0], &tag) != nil {
		return
	}
	switch tag {
	case "er":
		n, _ := Int(r, 2)
		d.queue = append(d.queue, streamItem{err: &Error{ID: Str(r, 1), ServerErr: true, Code: int(n)}})
	case "af.httprm":
		if s := Str(r, 2); s != "" {
			d.tr.FSID = s
		}
	case "wrb.fr":
		var payload []byte
		if len(cols) > 2 {
			payload = d.unquote(cols[2])
		}
		if payload == nil {
			if len(cols) > 5 {
				if st := parseStatus(cols[5]); st != nil {
					d.queue = append(d.queue, streamItem{snap: Snapshot{Status: st, Final: true}})
				}
			}
			return
		}
		if s, ok := parseSnapshot(payload); ok {
			d.queue = append(d.queue, streamItem{snap: s})
		}
	}
}

// parseSnapshot reads the answer inner payload:
//
//	[row, null, markers, followUps, isFinal, ...]
//	row = [text, null, [conv, resp, n], null, typeBlock]
//	typeBlock = [rich, null, null, passages, flag]   flag 1 answer, 2 thinking
//
// Heartbeats (`[]`) and payloads without a row report ok=false.
func parseSnapshot(p []byte) (Snapshot, bool) {
	top, ok := splitElems(p)
	if !ok || len(top) == 0 {
		return Snapshot{}, false
	}
	row, ok := splitElems(top[0])
	if !ok || len(row) == 0 {
		return Snapshot{}, false
	}
	var s Snapshot
	s.Text = rawString(row[0])
	if len(row) > 2 {
		if ids, ok := splitElems(row[2]); ok {
			if len(ids) > 0 {
				s.ConversationID = rawString(ids[0])
			}
			if len(ids) > 1 {
				s.ResponseID = rawString(ids[1])
			}
		}
	}
	var passages [][]byte
	if len(row) > 4 {
		if tb, ok := splitElems(row[4]); ok && len(tb) > 0 {
			var flag float64
			if json.Unmarshal(tb[len(tb)-1], &flag) == nil && flag == 2 {
				s.Thinking = true
			}
			if len(tb) > 3 {
				passages, _ = splitElems(tb[3])
			}
		}
	}
	if len(top) > 4 {
		var fin bool
		_ = json.Unmarshal(top[4], &fin)
		s.Final = fin
	}
	if len(top) > 2 {
		if markers, ok := splitElems(top[2]); ok {
			s.Citations = citations(markers, passages)
		}
	}
	return s, true
}

// citations expands markers `[[null, start, end], [idx...]]` into one
// Citation per passage index; citation N reads passage N-1. The second
// element lists passage indices (live captures: [0,1] and [1,2] at two
// markers match the type block's own list of passages 0,1 and 1,2; a
// single-passage marker is [0]); it is not a [from, to) range. The same N
// at another offset is a separate citation; exact duplicates (same N, same
// offsets) are dropped. At most
// maxCitationRange indices are read per marker.
func citations(markers, passages [][]byte) []Citation {
	var out []Citation
	type key struct{ n, start, end int }
	seen := map[key]bool{}
	for _, m := range markers {
		mc, ok := splitElems(m)
		if !ok || len(mc) < 2 {
			continue
		}
		pos, _ := splitElems(mc[0])
		idxs, ok := splitElems(mc[1])
		if !ok {
			continue
		}
		var start, end int
		if len(pos) > 2 {
			start, end = rawInt(pos[1]), rawInt(pos[2])
		}
		if len(idxs) > maxCitationRange {
			idxs = idxs[:maxCitationRange]
		}
		for _, raw := range idxs {
			idx, ok := passageIndex(raw)
			if !ok {
				continue
			}
			k := key{idx + 1, start, end}
			if seen[k] { // exact duplicate: same passage at the same offsets
				continue
			}
			seen[k] = true
			c := Citation{N: idx + 1, Start: start, End: end}
			if idx < len(passages) {
				c.SourceID = Str(passages[idx], 1, 5, 0, 0, 0)
				c.Text = Str(passages[idx], 1, 4)
			}
			out = append(out, c)
		}
	}
	return out
}

// passageIndex parses one marker index: a whole number from 0 to 2^31.
// Anything else (negative, fractional, not a number) is rejected so that it
// is skipped, not read as passage 0. JSON null would unmarshal to 0, so
// only a number literal is accepted.
func passageIndex(v []byte) (int, bool) {
	v = bytes.TrimSpace(v)
	if len(v) == 0 || (v[0] != '-' && (v[0] < '0' || v[0] > '9')) {
		return 0, false
	}
	var f float64
	if json.Unmarshal(v, &f) != nil || f < 0 || f > 1<<31 || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

func rawString(v []byte) string {
	var s string
	if json.Unmarshal(v, &s, lenient) != nil {
		return ""
	}
	return s
}

func rawInt(v []byte) int {
	var f float64
	if json.Unmarshal(bytes.TrimSpace(v), &f) != nil || f < 0 || f > 1<<31 {
		return 0
	}
	return int(f)
}

// LastAnswer returns the answer to use from a collected stream: the last
// snapshot with Final set, else the last non-thinking snapshot without a
// rejection status, else the zero Snapshot. Because snapshots are cumulative,
// that one snapshot is the whole answer.
func LastAnswer(snaps []Snapshot) Snapshot {
	for i := len(snaps) - 1; i >= 0; i-- {
		if s := snaps[i]; s.Final && !s.Thinking && s.Status == nil {
			return s
		}
	}
	for i := len(snaps) - 1; i >= 0; i-- {
		if s := snaps[i]; !s.Thinking && s.Status == nil {
			return s
		}
	}
	return Snapshot{}
}

// SliceUTF16 returns the part of s between UTF-16 code-unit offsets start and
// end, the unit citation offsets use. Offsets are clamped; a rune is kept when
// its first unit lies in [start, end).
func SliceUTF16(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end <= start {
		return ""
	}
	u, from, to := 0, -1, len(s)
	for i, r := range s {
		if from < 0 && u >= start {
			from = i
		}
		if u >= end {
			to = i
			break
		}
		if r > 0xFFFF {
			u += 2
		} else {
			u++
		}
	}
	if from < 0 {
		return ""
	}
	return s[from:to]
}
