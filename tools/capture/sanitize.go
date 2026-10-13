package main

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Fixture matches the golden capture format read by internal/nlm tests.
type Fixture struct {
	RPC          string         `json:"rpc"`
	Synthetic    bool           `json:"synthetic"`
	Source       string         `json:"source"`
	Request      jsontext.Value `json:"request"`
	ResponseBody string         `json:"response_body"`
	Captured     string         `json:"captured"`
	Host         string         `json:"host"`
}

type Options struct {
	KeepTitles bool // keep free text (titles, notes, answers); emails and ids are always replaced
}

// FakeUUID is the n-th stable replacement id (1-based).
func FakeUUID(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }

type har struct {
	Log struct {
		Entries []struct {
			Started string `json:"startedDateTime"`
			Request struct {
				Method   string `json:"method"`
				URL      string `json:"url"`
				PostData struct {
					Text string `json:"text"`
				} `json:"postData"`
			} `json:"request"`
			Response struct {
				Status  int `json:"status"`
				Content struct {
					Text     string `json:"text"`
					Encoding string `json:"encoding"`
				} `json:"content"`
			} `json:"response"`
		} `json:"entries"`
	} `json:"log"`
}

const streamRPC = "GenerateFreeFormStreamed"

// Convert turns a HAR into sanitized fixtures, one per RPC call. Only the
// RPC id, the decoded params and the response body are kept: headers,
// cookies and the CSRF token are never read into the output.
func Convert(harJSON []byte, o Options) ([]Fixture, error) {
	var h har
	if err := json.Unmarshal(harJSON, &h, json.RejectUnknownMembers(false)); err != nil {
		return nil, fmt.Errorf("capture: HAR: %w", err)
	}
	s := newSanitizer(o)
	var out []Fixture
	for _, e := range h.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || e.Request.Method != "POST" {
			continue
		}
		isBatch := strings.HasSuffix(u.Path, "/data/batchexecute")
		isStream := strings.HasSuffix(u.Path, "/"+streamRPC)
		if !isBatch && !isStream {
			continue
		}
		body := e.Response.Content.Text
		if e.Response.Content.Encoding == "base64" {
			b, err := base64.StdEncoding.DecodeString(body)
			if err != nil {
				continue
			}
			body = string(b)
		}
		form, err := url.ParseQuery(e.Request.PostData.Text)
		if err != nil {
			continue
		}
		cleanBody, err := s.body(body)
		if err != nil {
			return nil, fmt.Errorf("capture: %s: %w", u.Path, err)
		}
		date, _, _ := strings.Cut(e.Started, "T")
		base := Fixture{Source: "live capture", ResponseBody: cleanBody, Captured: date, Host: u.Hostname()}
		for rpc, params := range s.requests(form.Get("f.req"), isStream) {
			f := base
			f.RPC, f.Request = rpc, params
			out = append(out, f)
		}
	}
	return out, nil
}

type sanitizer struct {
	o     Options
	ids   map[string]string
	texts map[string]string
}

func newSanitizer(o Options) *sanitizer {
	return &sanitizer{o: o, ids: map[string]string{}, texts: map[string]string{}}
}

// requests decodes f.req into rpc id -> sanitized params, in request order.
func (s *sanitizer) requests(freq string, stream bool) func(yield func(string, jsontext.Value) bool) {
	return func(yield func(string, jsontext.Value) bool) {
		var top []any
		if json.Unmarshal([]byte(freq), &top) != nil {
			return
		}
		if stream {
			if len(top) > 1 {
				if p, ok := top[1].(string); ok {
					yield(streamRPC, s.params(p))
				}
			}
			return
		}
		if len(top) == 0 {
			return
		}
		items, _ := top[0].([]any)
		for _, it := range items {
			row, _ := it.([]any)
			if len(row) < 2 {
				continue
			}
			id, _ := row[0].(string)
			p, _ := row[1].(string)
			if !yield(id, s.params(p)) {
				return
			}
		}
	}
}

func (s *sanitizer) params(p string) jsontext.Value {
	var v any
	if json.Unmarshal([]byte(p), &v) != nil {
		b, _ := json.Marshal(s.str(p))
		return b
	}
	b, _ := json.Marshal(s.walk(v))
	return b
}

// body rewrites every wrb.fr payload and re-frames the chunks.
func (s *sanitizer) body(body string) (string, error) {
	segs, err := wire.Segments([]byte(body))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(")]}'\n\n")
	for _, seg := range segs {
		var rows []any
		if json.Unmarshal(seg, &rows) != nil {
			continue
		}
		for _, r := range rows {
			row, ok := r.([]any)
			if !ok || len(row) < 3 {
				continue
			}
			if tag, _ := row[0].(string); tag == "wrb.fr" {
				if p, ok := row[2].(string); ok {
					row[2] = s.payloadString(p)
				}
			}
		}
		out, err := json.Marshal(rows)
		if err != nil {
			return "", err
		}
		b.WriteString(strconv.Itoa(len(out)))
		b.WriteByte('\n')
		b.Write(out)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func (s *sanitizer) payloadString(p string) string {
	var v any
	if json.Unmarshal([]byte(p), &v) != nil {
		return s.str(p)
	}
	b, _ := json.Marshal(s.walk(v))
	return string(b)
}

func (s *sanitizer) walk(v any) any {
	switch x := v.(type) {
	case string:
		t := strings.TrimSpace(x)
		if strings.HasPrefix(t, "[") || strings.HasPrefix(t, "{") {
			var inner any
			if json.Unmarshal([]byte(t), &inner) == nil {
				b, _ := json.Marshal(s.walk(inner))
				return string(b)
			}
		}
		return s.str(x)
	case []any:
		for i := range x {
			x[i] = s.walk(x[i])
		}
		return x
	case map[string]any:
		for k := range x {
			x[k] = s.walk(x[k])
		}
		return x
	}
	return v
}

var (
	reUUID  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reEmail = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

func (s *sanitizer) str(x string) string {
	x = reUUID.ReplaceAllStringFunc(x, func(id string) string {
		id = strings.ToLower(id)
		if f, ok := s.ids[id]; ok {
			return f
		}
		f := FakeUUID(len(s.ids) + 1)
		s.ids[id] = f
		return f
	})
	x = reEmail.ReplaceAllString(x, "user@example.com")
	if u, err := url.Parse(x); err == nil && u.Scheme != "" && u.Host != "" {
		if strings.HasSuffix(u.Hostname(), "google.com") || strings.HasSuffix(u.Hostname(), "googleusercontent.com") {
			u.RawQuery = ""
		}
		return u.String()
	}
	if !s.o.KeepTitles && freeText(x) {
		if f, ok := s.texts[x]; ok {
			return f
		}
		f := "text-" + strconv.Itoa(len(s.texts)+1)
		s.texts[x] = f
		return f
	}
	return x
}

// freeText guesses whether a string is user content (title, note, answer)
// rather than an id, enum, MIME type or token: it has whitespace, or
// letters with an uppercase or non-ASCII rune and no _ / . separators.
func freeText(x string) bool {
	if x == "" || x == "user@example.com" || strings.HasPrefix(x, "00000000-0000-4000-8000-") {
		return false
	}
	if strings.ContainsFunc(x, unicode.IsSpace) {
		return true
	}
	if strings.ContainsAny(x, "_/.") {
		return false
	}
	for _, r := range x {
		if unicode.IsUpper(r) || r > unicode.MaxASCII {
			return true
		}
	}
	return false
}
