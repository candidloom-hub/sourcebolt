// Package icon draws the sourcebolt mark: a notebook page with a lightning
// bolt, on a rounded square. The geometry is defined once here and rendered
// both as SVG (docs/assets/icon.svg) and as PNG at any size, so the two
// never drift. Only the standard library is used and the output is
// deterministic (same bytes for the same size).
package icon

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
)

// Canvas is the design grid; every coordinate below is on a 512x512 canvas.
const Canvas = 512

type pt struct{ X, Y float64 }

type shape struct {
	fill color.NRGBA
	// Exactly one of the following is set.
	rect   *rrect
	circle *circ
	poly   []pt
}

type rrect struct{ X, Y, W, H, R float64 }
type circ struct{ X, Y, R float64 }

var (
	navy  = color.NRGBA{0x1B, 0x26, 0x44, 0xFF}
	paper = color.NRGBA{0xF4, 0xF1, 0xEA, 0xFF}
	spine = color.NRGBA{0xC9, 0xC2, 0xB0, 0xFF}
	amber = color.NRGBA{0xF5, 0xB7, 0x00, 0xFF}
)

// shapes are painted in order (later on top).
func shapes() []shape {
	s := []shape{
		{fill: navy, rect: &rrect{0, 0, 512, 512, 104}},
		{fill: paper, rect: &rrect{116, 92, 280, 328, 22}},
		{fill: spine, rect: &rrect{116, 92, 52, 328, 0}},
	}
	for _, y := range []float64{150, 222, 294, 366} {
		s = append(s, shape{fill: navy, circle: &circ{142, y, 11}})
	}
	// The bolt: a classic seven-point zigzag, slightly right of centre.
	s = append(s, shape{fill: amber, poly: []pt{
		{314, 104}, {204, 284}, {266, 284}, {232, 412}, {350, 222}, {288, 222}, {334, 104},
	}})
	return s
}

func hex(c color.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

func num(f float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), ".")
}

// SVG returns the mark as a standalone SVG document.
func SVG() []byte {
	var b bytes.Buffer
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512" role="img" aria-label="sourcebolt">` + "\n")
	b.WriteString("  <title>sourcebolt</title>\n")
	for _, s := range shapes() {
		switch {
		case s.rect != nil:
			r := s.rect
			rx := ""
			if r.R > 0 {
				rx = ` rx="` + num(r.R) + `"`
			}
			fmt.Fprintf(&b, `  <rect x="%s" y="%s" width="%s" height="%s"%s fill="%s"/>`+"\n", num(r.X), num(r.Y), num(r.W), num(r.H), rx, hex(s.fill))
		case s.circle != nil:
			c := s.circle
			fmt.Fprintf(&b, `  <circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", num(c.X), num(c.Y), num(c.R), hex(s.fill))
		default:
			var p []string
			for _, q := range s.poly {
				p = append(p, num(q.X)+","+num(q.Y))
			}
			fmt.Fprintf(&b, `  <polygon points="%s" fill="%s"/>`+"\n", strings.Join(p, " "), hex(s.fill))
		}
	}
	b.WriteString("</svg>\n")
	return b.Bytes()
}

// inside reports whether canvas point (x, y) is covered by s.
func (s shape) inside(x, y float64) bool {
	switch {
	case s.rect != nil:
		r := s.rect
		if x < r.X || x > r.X+r.W || y < r.Y || y > r.Y+r.H {
			return false
		}
		// Distance to the nearest corner centre when in a corner zone.
		cx := math.Max(r.X+r.R, math.Min(x, r.X+r.W-r.R))
		cy := math.Max(r.Y+r.R, math.Min(y, r.Y+r.H-r.R))
		dx, dy := x-cx, y-cy
		return dx*dx+dy*dy <= r.R*r.R
	case s.circle != nil:
		c := s.circle
		dx, dy := x-c.X, y-c.Y
		return dx*dx+dy*dy <= c.R*c.R
	default:
		in := false
		p := s.poly
		for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
			if (p[i].Y > y) != (p[j].Y > y) && x < (p[j].X-p[i].X)*(y-p[i].Y)/(p[j].Y-p[i].Y)+p[i].X {
				in = !in
			}
		}
		return in
	}
}

// Image renders the mark at size x size pixels with 4x4 supersampling.
// Pixels outside the rounded square are transparent.
func Image(size int) *image.NRGBA {
	const ss = 4
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	sh := shapes()
	scale := float64(Canvas) / float64(size)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, bl, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) * scale
					y := (float64(py) + (float64(sy)+0.5)/ss) * scale
					var c color.NRGBA
					for _, s := range sh {
						if s.inside(x, y) {
							c = s.fill
						}
					}
					if c.A != 0 {
						r += float64(c.R)
						g += float64(c.G)
						bl += float64(c.B)
						a++
					}
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(r / a)),
				G: uint8(math.Round(g / a)),
				B: uint8(math.Round(bl / a)),
				A: uint8(math.Round(a * 255 / ss / ss)),
			})
		}
	}
	return img
}

// PNG encodes Image(size) with best compression.
func PNG(size int) ([]byte, error) {
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&b, Image(size)); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
