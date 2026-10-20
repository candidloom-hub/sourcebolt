package icon

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The committed SVG must be exactly what the generator writes.
func TestCommittedSVGIsCurrent(t *testing.T) {
	want := SVG()
	got, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "assets", "icon.svg"))
	if err != nil {
		t.Fatal(err)
	}
	got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Fatalf("docs/assets/icon.svg is stale; run: go run ./tools/icon -svg > docs/assets/icon.svg")
	}
}

func TestPNGDeterministicAndSized(t *testing.T) {
	for _, size := range []int{400, 512} {
		a, err := PNG(size)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := PNG(size)
		if !bytes.Equal(a, b) {
			t.Fatalf("PNG(%d) is not deterministic", size)
		}
		img, err := png.Decode(bytes.NewReader(a))
		if err != nil {
			t.Fatal(err)
		}
		if r := img.Bounds(); r.Dx() != size || r.Dy() != size {
			t.Fatalf("PNG(%d) is %v", size, r)
		}
		// Corners are transparent (rounded square), the centre is opaque.
		if _, _, _, al := img.At(0, 0).RGBA(); al != 0 {
			t.Fatalf("corner alpha %d, want 0", al)
		}
		if _, _, _, al := img.At(size/2, size/2).RGBA(); al != 0xffff {
			t.Fatalf("centre alpha %d, want opaque", al)
		}
		if len(a) > 64<<10 {
			t.Fatalf("PNG(%d) is %d bytes; keep the mark simple", size, len(a))
		}
	}
}
