package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"testing"
)

func TestMosaicTiles(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 25, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 25; x++ {
			v := uint8(100)
			if x%12 >= 6 {
				v = 200
			}
			src.SetNRGBA(x, y, color.NRGBA{R: v, G: 80, B: 40, A: 255})
		}
	}
	out := mosaic(src, 12)
	at := func(x, y int) color.RGBA { return color.RGBAModel.Convert(out.At(x, y)).(color.RGBA) }
	if got := at(5, 5); got.R != 150 || got.G != 80 || got.B != 40 || got.A != 255 {
		t.Fatalf("tile average: %v", got)
	}
	if at(0, 5) != at(12, 5) || at(0, 5).R != 218 {
		t.Fatal("missing grout")
	}
	if at(1, 5).R <= at(5, 5).R || at(11, 5).R >= at(5, 5).R {
		t.Fatal("missing bevel light or shadow")
	}
	if out.Bounds() != src.Bounds() || at(24, 11).A != 255 {
		t.Fatal("incomplete edge")
	}
}
func TestTransform(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 24))
	src.Set(20, 10, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	png.Encode(&buf, src)
	for _, mode := range []string{"mosaic", "kaleidoscope", "stainedglass"} {
		t.Run(mode, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/transform?mode="+mode+"&amount=8", bytes.NewReader(buf.Bytes()))
			w := httptest.NewRecorder()
			transform(w, r)
			if w.Code != 200 {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			img, err := png.Decode(w.Body)
			if err != nil {
				t.Fatal(err)
			}
			wantWidth := 32
			if mode == "kaleidoscope" {
				wantWidth = 24
			}
			if img.Bounds().Dx() != wantWidth || img.Bounds().Dy() != 24 {
				t.Fatal(img.Bounds())
			}
		})
	}
	for _, tc := range []struct {
		method, url string
		data        []byte
		status      int
	}{{"GET", "/api/transform", nil, 405}, {"POST", "/api/transform?mode=mosaic", []byte("invalid"), 400}, {"POST", "/api/transform?mode=bad", buf.Bytes(), 400}, {"POST", "/api/transform?mode=mosaic", make([]byte, (8<<20)+1), 413}} {
		w := httptest.NewRecorder()
		transform(w, httptest.NewRequest(tc.method, tc.url, bytes.NewReader(tc.data)))
		if w.Code != tc.status {
			t.Fatalf("got %d want %d", w.Code, tc.status)
		}
	}
}
func TestKaleidoscopeUniform(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 7, 5))
	want := color.RGBA{R: 23, G: 91, B: 140, A: 255}
	for y := 0; y < 5; y++ {
		for x := 0; x < 7; x++ {
			src.Set(x, y, want)
		}
	}
	out := kaleidoscope(src, 8)
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			if out.At(x, y) != want {
				t.Fatal(out.At(x, y))
			}
		}
	}
}

func TestStainedGlassPanes(t *testing.T) {
	src := image.NewNRGBA(image.Rect(3, 5, 28, 22))
	red := color.NRGBA{220, 70, 40, 255}
	blue := color.NRGBA{40, 90, 220, 255}
	for y := 0; y < 17; y++ {
		for x := 0; x < 25; x++ {
			c := red
			if x > y {
				c = blue
			}
			src.SetNRGBA(x+3, y+5, c)
		}
	}
	out := stainedGlass(src, 16).(*image.RGBA)
	if out.Bounds() != image.Rect(0, 0, 25, 17) {
		t.Fatal(out.Bounds())
	}
	if got := out.RGBAAt(3, 10); got.R != red.R || got.B != red.B {
		t.Fatalf("red pane: %v", got)
	}
	if got := out.RGBAAt(10, 3); got.R != blue.R || got.B != blue.B {
		t.Fatalf("blue pane: %v", got)
	}
	if got := out.RGBAAt(8, 8); got != (color.RGBA{37, 41, 53, 255}) {
		t.Fatalf("lead seam: %v", got)
	}
	for y := 0; y < 17; y++ {
		for x := 0; x < 25; x++ {
			if out.RGBAAt(x, y).A != 255 {
				t.Fatalf("unpainted pixel at %d,%d", x, y)
			}
		}
	}
}
