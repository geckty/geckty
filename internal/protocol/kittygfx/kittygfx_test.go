package kittygfx

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func rgbaBytes(w, h int, c color.NRGBA) []byte {
	out := make([]byte, 0, w*h*4)
	for i := 0; i < w*h; i++ {
		out = append(out, c.R, c.G, c.B, c.A)
	}
	return out
}

func rgbBytes(w, h int, c color.NRGBA) []byte {
	out := make([]byte, 0, w*h*3)
	for i := 0; i < w*h; i++ {
		out = append(out, c.R, c.G, c.B)
	}
	return out
}

func b64(p []byte) string {
	return base64.StdEncoding.EncodeToString(p)
}

func pngBytes(t *testing.T, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestFeedIgnoresNonGraphicsAPC(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Xsomething-else"))
	resp, placement := r.Resp, r.Placement
	if resp != nil || placement != nil {
		t.Fatalf("expected both nil for a non-'G' payload, got resp=%q placement=%v", resp, placement)
	}
}

func TestFeedRGBASingleShot(t *testing.T) {
	d := NewDecoder()
	payload := "Ga=T,f=32,s=2,v=1,i=7;" + b64(rgbaBytes(2, 1, color.NRGBA{R: 10, G: 20, B: 30, A: 255}))

	fr := d.Feed([]byte(payload))
	resp, placement := fr.Resp, fr.Placement
	if placement == nil {
		t.Fatal("expected a completed placement")
	}
	if placement.ID != 7 {
		t.Fatalf("ID = %d, want 7", placement.ID)
	}
	bounds := placement.Image.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 1 {
		t.Fatalf("image size = %v, want 2x1", bounds)
	}
	pr, pg, pb, pa := placement.Image.At(0, 0).RGBA()
	if pr>>8 != 10 || pg>>8 != 20 || pb>>8 != 30 || pa>>8 != 255 {
		t.Fatalf("pixel = %d,%d,%d,%d, want 10,20,30,255", pr>>8, pg>>8, pb>>8, pa>>8)
	}

	if want := "\x1b_Gi=7;OK\x1b\\"; string(resp) != want {
		t.Fatalf("resp = %q, want %q", resp, want)
	}
}

func TestFeedRGBSingleShotDefaultsAlphaOpaque(t *testing.T) {
	d := NewDecoder()
	payload := "Ga=T,f=24,s=1,v=1;" + b64(rgbBytes(1, 1, color.NRGBA{R: 200, G: 100, B: 50}))

	fr := d.Feed([]byte(payload))
	placement := fr.Placement
	if placement == nil {
		t.Fatal("expected a completed placement")
	}
	pr, pg, pb, pa := placement.Image.At(0, 0).RGBA()
	if pr>>8 != 200 || pg>>8 != 100 || pb>>8 != 50 || pa>>8 != 255 {
		t.Fatalf("pixel = %d,%d,%d,%d, want 200,100,50,255", pr>>8, pg>>8, pb>>8, pa>>8)
	}
}

func TestFeedFormatDefaultsToRGBA(t *testing.T) {
	d := NewDecoder()
	// No f= key at all — must default to 32 (RGBA), not error.
	payload := "Ga=T,s=1,v=1;" + b64(rgbaBytes(1, 1, color.NRGBA{R: 1, G: 2, B: 3, A: 4}))

	r := d.Feed([]byte(payload))
	placement := r.Placement
	if placement == nil {
		t.Fatal("expected a completed placement with the default RGBA format")
	}
}

func TestFeedPNG(t *testing.T) {
	d := NewDecoder()
	raw := pngBytes(t, 3, 2, color.NRGBA{R: 9, G: 8, B: 7, A: 255})
	payload := "Ga=T,f=100,i=42;" + b64(raw)

	r := d.Feed([]byte(payload))
	resp, placement := r.Resp, r.Placement
	if placement == nil {
		t.Fatal("expected a completed placement")
	}
	bounds := placement.Image.Bounds()
	if bounds.Dx() != 3 || bounds.Dy() != 2 {
		t.Fatalf("image size = %v, want 3x2", bounds)
	}
	if want := "\x1b_Gi=42;OK\x1b\\"; string(resp) != want {
		t.Fatalf("resp = %q, want %q", resp, want)
	}
}

func TestFeedChunkedTransmission(t *testing.T) {
	d := NewDecoder()
	full := rgbaBytes(2, 2, color.NRGBA{R: 5, G: 6, B: 7, A: 255})
	encoded := b64(full)
	// Split the base64 text into three pieces at 4-byte boundaries
	// (required for all but the last chunk), mirroring the spec's own
	// worked chunking example.
	third := (len(encoded) / 3 / 4) * 4
	if third == 0 {
		third = 4
	}
	c1, rest := encoded[:third], encoded[third:]
	c2, c3 := rest[:third], rest[third:]

	if fr := d.Feed([]byte("Ga=T,f=32,s=2,v=2,i=1,m=1;" + c1)); fr.Resp != nil || fr.Placement != nil {
		t.Fatalf("first chunk: expected no response/placement yet, got resp=%q placement=%v", fr.Resp, fr.Placement)
	}
	if fr := d.Feed([]byte("Gm=1;" + c2)); fr.Resp != nil || fr.Placement != nil {
		t.Fatalf("middle chunk: expected no response/placement yet, got resp=%q placement=%v", fr.Resp, fr.Placement)
	}
	fr := d.Feed([]byte("Gm=0;" + c3))
	resp, p := fr.Resp, fr.Placement
	if p == nil {
		t.Fatal("final chunk: expected a completed placement")
	}
	if want := "\x1b_Gi=1;OK\x1b\\"; string(resp) != want {
		t.Fatalf("resp = %q, want %q", resp, want)
	}
	bounds := p.Image.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Fatalf("image size = %v, want 2x2", bounds)
	}
	pr, pg, pb, pa := p.Image.At(1, 1).RGBA()
	if pr>>8 != 5 || pg>>8 != 6 || pb>>8 != 7 || pa>>8 != 255 {
		t.Fatalf("pixel = %d,%d,%d,%d, want 5,6,7,255", pr>>8, pg>>8, pb>>8, pa>>8)
	}
}

func TestFeedAbandonedTransmissionRecovers(t *testing.T) {
	d := NewDecoder()
	// Start a chunked transfer but never send m=0.
	if fr := d.Feed([]byte("Ga=T,f=32,s=1,v=1,m=1;" + b64(rgbaBytes(1, 1, color.NRGBA{A: 255})))); fr.Placement != nil {
		t.Fatal("unexpected placement from an in-progress chunk")
	}
	// A brand new command's own control data arrives instead of a
	// continuation chunk — the old transfer must be discarded, not
	// misread.
	payload := "Ga=T,f=24,s=1,v=1;" + b64(rgbBytes(1, 1, color.NRGBA{R: 255, G: 255, B: 255}))
	r := d.Feed([]byte(payload))
	placement := r.Placement
	if placement == nil {
		t.Fatal("expected the new command to complete on its own")
	}
	bounds := placement.Image.Bounds()
	if bounds.Dx() != 1 || bounds.Dy() != 1 {
		t.Fatalf("image size = %v, want 1x1 (the new command's own size)", bounds)
	}
}

func TestFeedUnsupportedActionReturnsError(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Ga=p,i=3;"))
	resp, placement := r.Resp, r.Placement
	if placement != nil {
		t.Fatal("expected no placement for an unsupported action")
	}
	if resp == nil {
		t.Fatal("expected an error response so the client doesn't hang")
	}
	if !bytes.HasPrefix(resp, []byte("\x1b_Gi=3;EINVAL:")) {
		t.Fatalf("resp = %q, want an EINVAL error prefixed with i=3", resp)
	}
}

func TestFeedUnsupportedMediumReturnsError(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Ga=T,t=f,s=1,v=1;" + b64([]byte("/etc/passwd"))))
	resp, placement := r.Resp, r.Placement
	if placement != nil {
		t.Fatal("expected no placement for a file-based transmission medium")
	}
	if !bytes.Contains(resp, []byte("EINVAL")) {
		t.Fatalf("resp = %q, want an EINVAL error", resp)
	}
}

func TestFeedUnsupportedFormatReturnsError(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Ga=T,f=7,s=1,v=1;AAAA"))
	resp, placement := r.Resp, r.Placement
	if placement != nil {
		t.Fatal("expected no placement for an unsupported format")
	}
	if !bytes.Contains(resp, []byte("EINVAL")) {
		t.Fatalf("resp = %q, want an EINVAL error", resp)
	}
}

func TestFeedSizeMismatchReturnsError(t *testing.T) {
	d := NewDecoder()
	// Claims 2x2 RGBA (16 bytes) but only supplies 4.
	r := d.Feed([]byte("Ga=T,f=32,s=2,v=2;" + b64([]byte{1, 2, 3, 4})))
	resp, placement := r.Resp, r.Placement
	if placement != nil {
		t.Fatal("expected no placement for a size-mismatched payload")
	}
	if !bytes.Contains(resp, []byte("EINVAL")) {
		t.Fatalf("resp = %q, want an EINVAL error", resp)
	}
}

func TestFeedInvalidBase64ReturnsError(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Ga=T,f=32,s=1,v=1;not-valid-base64!!"))
	resp, placement := r.Resp, r.Placement
	if placement != nil {
		t.Fatal("expected no placement for invalid base64")
	}
	if !bytes.Contains(resp, []byte("EINVAL")) {
		t.Fatalf("resp = %q, want an EINVAL error", resp)
	}
}

func TestFeedQuiet1SuppressesOKOnly(t *testing.T) {
	d := NewDecoder()
	payload := "Ga=T,f=32,s=1,v=1,q=1;" + b64(rgbaBytes(1, 1, color.NRGBA{A: 255}))
	r := d.Feed([]byte(payload))
	resp, placement := r.Resp, r.Placement
	if placement == nil {
		t.Fatal("expected a completed placement")
	}
	if resp != nil {
		t.Fatalf("q=1 must suppress the OK response, got %q", resp)
	}

	d2 := NewDecoder()
	resp2 := d2.Feed([]byte("Ga=p,q=1;")).Resp
	if resp2 == nil {
		t.Fatal("q=1 must NOT suppress error responses")
	}
}

func TestFeedQuiet2SuppressesErrorsToo(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Ga=p,q=2;"))
	resp, placement := r.Resp, r.Placement
	if placement != nil {
		t.Fatal("expected no placement")
	}
	if resp != nil {
		t.Fatalf("q=2 must suppress error responses too, got %q", resp)
	}
}

func TestFeedResponseOmitsIDWhenNotGiven(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Ga=T,f=32,s=1,v=1;" + b64(rgbaBytes(1, 1, color.NRGBA{A: 255}))))
	resp, placement := r.Resp, r.Placement
	if placement == nil {
		t.Fatal("expected a completed placement")
	}
	if want := "\x1b_G;OK\x1b\\"; string(resp) != want {
		t.Fatalf("resp = %q, want %q", resp, want)
	}
}

func TestFeedResponseIncludesPlacementID(t *testing.T) {
	d := NewDecoder()
	payload := "Ga=T,f=32,s=1,v=1,i=5,p=9;" + b64(rgbaBytes(1, 1, color.NRGBA{A: 255}))
	r := d.Feed([]byte(payload))
	resp, placement := r.Resp, r.Placement
	if placement == nil {
		t.Fatal("expected a completed placement")
	}
	if want := "\x1b_Gi=5,p=9;OK\x1b\\"; string(resp) != want {
		t.Fatalf("resp = %q, want %q", resp, want)
	}
}

func TestFeedCarriesRequestedCellSpan(t *testing.T) {
	d := NewDecoder()
	payload := "Ga=T,f=32,s=1,v=1,c=10,r=4;" + b64(rgbaBytes(1, 1, color.NRGBA{A: 255}))
	r := d.Feed([]byte(payload))
	placement := r.Placement
	if placement == nil {
		t.Fatal("expected a completed placement")
	}
	if placement.Cols != 10 || placement.Rows != 4 {
		t.Fatalf("Cols,Rows = %d,%d, want 10,4", placement.Cols, placement.Rows)
	}
}

func TestFeedOversizedRawDimensionsRejected(t *testing.T) {
	d := NewDecoder()
	r := d.Feed([]byte("Ga=T,f=32,s=100000,v=100000;AAAA"))
	resp, placement := r.Resp, r.Placement
	if placement != nil {
		t.Fatal("expected no placement for an absurdly large claimed image")
	}
	if !bytes.Contains(resp, []byte("EINVAL")) {
		t.Fatalf("resp = %q, want an EINVAL error", resp)
	}
}

func TestFeedQueryReturnsOK(t *testing.T) {
	d := NewDecoder()
	fr := d.Feed([]byte("Ga=q,i=42;"))
	if fr.Placement != nil || fr.DeleteAll || fr.DeleteID != 0 {
		t.Fatalf("query must not place/delete, got %+v", fr)
	}
	if want := "\x1b_Gi=42;OK\x1b\\"; string(fr.Resp) != want {
		t.Fatalf("resp = %q, want %q", fr.Resp, want)
	}
}

func TestFeedDeleteAll(t *testing.T) {
	d := NewDecoder()
	fr := d.Feed([]byte("Ga=d;"))
	if !fr.DeleteAll || fr.DeleteID != 0 || fr.Placement != nil {
		t.Fatalf("got %+v, want DeleteAll", fr)
	}
	if want := "\x1b_G;OK\x1b\\"; string(fr.Resp) != want {
		t.Fatalf("resp = %q, want %q", fr.Resp, want)
	}
}

func TestFeedDeleteByID(t *testing.T) {
	d := NewDecoder()
	fr := d.Feed([]byte("Ga=d,d=i,i=10;"))
	if fr.DeleteAll || fr.DeleteID != 10 || fr.Placement != nil {
		t.Fatalf("got %+v, want DeleteID=10", fr)
	}
	if want := "\x1b_Gi=10;OK\x1b\\"; string(fr.Resp) != want {
		t.Fatalf("resp = %q, want %q", fr.Resp, want)
	}
}
