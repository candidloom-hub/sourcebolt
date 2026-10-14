package wire

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func FuzzStreamDecoder(f *testing.F) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "fixtures", "_stream", "*.txt"))
	for _, p := range files {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(b)
		}
	}
	f.Add(liveStreamBody(f))
	f.Add([]byte(")]}'\n16"))
	f.Add([]byte(")\n]\n}\n'\n[[\"wrb.fr\",null,\"[[[\\\"a\\\"]]]\"]]"))
	f.Add([]byte("<html>"))
	f.Fuzz(func(t *testing.T, body []byte) {
		d := NewStreamDecoder(bytes.NewReader(body))
		for i := 0; i < 1<<16; i++ {
			s, err := d.Next()
			if err != nil {
				// Once ended it must stay ended.
				if _, err2 := d.Next(); err2 == nil {
					t.Fatalf("Next succeeded after %v", err)
				}
				return
			}
			_ = SliceUTF16(s.Text, 0, 3)
		}
		t.Fatal("decoder did not terminate")
	})
}

func TestStreamErrorsAreStickyAndTyped(t *testing.T) {
	d := NewStreamDecoder(bytes.NewReader([]byte(")]}'\n\n5\n[[\"wrb")))
	_, err := d.Next()
	var we *Error
	if !errors.As(err, &we) {
		t.Fatalf("err = %v", err)
	}
	if _, err := d.Next(); errors.Is(err, io.EOF) {
		t.Fatal("error became EOF")
	}
}
