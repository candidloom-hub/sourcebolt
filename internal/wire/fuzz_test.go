package wire

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzDecode(f *testing.F) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "fixtures", "_wire", "*.txt"))
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err == nil {
			f.Add(b)
		}
	}
	f.Add([]byte(")]}'\n16"))
	f.Fuzz(func(t *testing.T, body []byte) {
		frames, _, err := Decode(body)
		if err == nil {
			_, _ = Pick(frames, "wXbhsf", NullAllowedUnlessRejected)
		}
	})
}
