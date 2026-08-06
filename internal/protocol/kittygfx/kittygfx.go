// Package kittygfx decodes the Kitty graphics protocol
// (https://sw.kovidgoyal.net/kitty/graphics-protocol/), fed complete APC
// payloads by internal/protocol.Sniffer (emu has no APC handling at all —
// confirmed by the project's M0 spike — so this, unlike OSC 52, genuinely
// needs the sniffer pattern rather than an emu hook).
//
// Scope, deliberately narrow for the MVP:
//
//   - Actions a=T (transmit-and-display), a=d (delete), and a=q (query)
//     are supported. a=p (place a previously-transmitted image) and
//     animation frames (a=f/a=a) are declined with an error response rather
//     than silently ignored, so a well-behaved client doesn't hang waiting
//     for a reply that will never come.
//   - Only transmission medium t=d (direct, escape-code-embedded base64)
//     is supported. File-based mediums (t=f local file, t=t temp file,
//     t=s shared memory) are refused: the spec's own security section
//     requires refusing special files, and even limited to regular files,
//     honoring a file path sent by the *program running in the terminal*
//     is a real risk in a remote/SSH scenario (a compromised remote
//     process could otherwise direct geckty, running locally, to read an
//     arbitrary local file by path). Direct transmission carries the
//     image bytes themselves, so it has no equivalent risk.
//   - Formats f=24 (RGB), f=32 (RGBA, the default when f is omitted), and
//     f=100 (PNG, via the standard library's image/png) are supported.
//   - Chunked transmission (m=1 / m=0) is reassembled.
//
// Not implemented: multiple placements per image / placement reuse by id
// (needs a=p), animation, non-zero pixel offsets within a cell (x/y), and
// z-index compositing (images always draw in front of text, negative z
// "behind text" is not distinguished). All are clean extensions on top of
// this package's Decoder if a future milestone needs them.
package kittygfx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"strings"
)

// Format is a Kitty graphics protocol pixel format (the 'f' control key).
type Format int

// Supported formats.
const (
	FormatRGB  Format = 24
	FormatRGBA Format = 32
	FormatPNG  Format = 100
)

// maxPayloadBytes bounds how much base64 text a single (possibly chunked)
// transmission can accumulate before Decoder gives up on it — defensive
// against a runaway or malicious sender that never sends a final (m=0)
// chunk. 200MiB of base64 comfortably covers any real terminal image.
const maxPayloadBytes = 200 << 20

// maxPixels bounds width*height for both raw and PNG payloads — defensive
// against a hostile s/v or PNG header claiming an enormous image and
// forcing a huge allocation.
const maxPixels = 40_000_000

// Placement is one fully decoded image, ready to display. It carries no
// position — the caller (internal/session) knows the terminal cursor
// position kittygfx has no visibility into, and is responsible for
// anchoring it.
type Placement struct {
	// ID is the control data's 'i' key, or 0 if the client didn't send
	// one.
	ID uint32
	// Image is the decoded picture.
	Image image.Image
	// Cols, Rows are the client's requested display size in terminal
	// cells (the 'c'/'r' keys). 0 means "not specified" — the caller
	// should compute a size from Image's pixel dimensions and the
	// current cell size instead.
	Cols, Rows int
}

// FeedResult is the outcome of one Decoder.Feed call.
type FeedResult struct {
	Resp      []byte
	Placement *Placement
	// DeleteID is the image id to delete when non-zero (a=d,d=i).
	DeleteID uint32
	// DeleteAll is set for a=d with d=a (or default) / no targeted id.
	DeleteAll bool
}

// controlData is the parsed form of an APC payload's key=value control
// data (the part before the first ';').
type controlData struct {
	action      byte // 'a'; 0 = unset
	format      int  // 'f'; 0 = unset (defaults to 32)
	medium      byte // 't'; 0 = unset (defaults to 'd')
	width       int  // 's'
	height      int  // 'v'
	id          uint32
	placementID uint32
	more        bool // 'm' == 1
	quiet       int  // 'q': 0, 1, or 2
	cols        int  // 'c'
	rows        int  // 'r'
	deleteSel   byte // 'd' delete selector (a=d only); 0 = unset
}

func parseControlData(s string) controlData {
	var cd controlData
	if s == "" {
		return cd
	}
	for _, pair := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "" {
			continue
		}
		switch k {
		case "a":
			if v != "" {
				cd.action = v[0]
			}
		case "f":
			cd.format, _ = strconv.Atoi(v)
		case "t":
			if v != "" {
				cd.medium = v[0]
			}
		case "s":
			cd.width, _ = strconv.Atoi(v)
		case "v":
			cd.height, _ = strconv.Atoi(v)
		case "i":
			n, _ := strconv.ParseUint(v, 10, 32)
			cd.id = uint32(n)
		case "p":
			n, _ := strconv.ParseUint(v, 10, 32)
			cd.placementID = uint32(n)
		case "m":
			cd.more = v == "1"
		case "q":
			cd.quiet, _ = strconv.Atoi(v)
		case "c":
			cd.cols, _ = strconv.Atoi(v)
		case "r":
			cd.rows, _ = strconv.Atoi(v)
		case "d":
			if v != "" {
				cd.deleteSel = v[0]
			}
		}
	}
	return cd
}

// Decoder assembles a (possibly chunked) direct-transmission Kitty
// graphics command into a decoded Placement. One Decoder tracks at most
// one in-flight transmission at a time, matching the protocol's own rule
// that a complete image must be sent before any other graphics command.
type Decoder struct {
	inFlight bool
	meta     controlData
	data     []byte // accumulated base64 text across chunks, undecoded
}

// NewDecoder returns a ready-to-use Decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Feed processes one complete APC payload (as internal/protocol.Sniffer
// delivers it: everything between "ESC _" and "ESC \", including the
// leading marker byte). Payloads not starting with 'G' aren't a Kitty
// graphics command at all and are silently ignored.
//
// Resp, if non-nil, is the terminal's reply and must be written back to
// the shell. Placement, if non-nil, is a newly completed image. DeleteAll /
// DeleteID signal placement removal for a=d.
func (d *Decoder) Feed(payload []byte) FeedResult {
	if len(payload) == 0 || payload[0] != 'G' {
		return FeedResult{}
	}
	ctrlStr, b64 := splitControlAndPayload(payload[1:])
	chunk := parseControlData(ctrlStr)

	if d.inFlight && chunk.action != 0 {
		// This chunk carries its own action, meaning it's not a
		// continuation of the in-flight transmission (which, per
		// spec, must be sent before any other graphics command) —
		// the previous transfer was abandoned (interrupted, or a
		// buggy sender that never sent m=0). Discard it and start
		// fresh rather than misreading this as one of its chunks.
		d.reset()
	}

	var cd controlData
	if !d.inFlight {
		cd = chunk
		d.meta = cd
		d.inFlight = true
		d.data = d.data[:0]
	} else {
		cd = d.meta
		cd.more = chunk.more
		if chunk.quiet != 0 {
			cd.quiet = chunk.quiet
		}
	}

	if cd.action == 'q' {
		// Query: we don't retain a separate image store in the decoder,
		// so answer OK (clients tolerate ENOENT for missing; OK is fine).
		resp := d.okResponse(cd)
		d.reset()
		return FeedResult{Resp: resp}
	}
	if cd.action == 'd' {
		result := FeedResult{Resp: d.okResponse(cd)}
		sel := cd.deleteSel
		if sel == 0 {
			sel = 'a'
		}
		switch sel {
		case 'i', 'I':
			if cd.id != 0 {
				result.DeleteID = cd.id
			} else {
				result.DeleteAll = true
			}
		default:
			result.DeleteAll = true
		}
		d.reset()
		return result
	}

	if cd.action != 'T' {
		resp := d.errorResponse(cd, "only action=T (transmit+display) is supported")
		d.reset()
		return FeedResult{Resp: resp}
	}
	medium := cd.medium
	if medium == 0 {
		medium = 'd'
	}
	if medium != 'd' {
		resp := d.errorResponse(cd, "only direct (t=d) transmission is supported")
		d.reset()
		return FeedResult{Resp: resp}
	}
	format := cd.format
	if format == 0 {
		format = int(FormatRGBA)
	}
	if format != int(FormatRGB) && format != int(FormatRGBA) && format != int(FormatPNG) {
		resp := d.errorResponse(cd, fmt.Sprintf("unsupported format f=%d", format))
		d.reset()
		return FeedResult{Resp: resp}
	}

	d.data = append(d.data, b64...)
	if len(d.data) > maxPayloadBytes {
		resp := d.errorResponse(cd, "payload too large")
		d.reset()
		return FeedResult{Resp: resp}
	}

	if cd.more {
		return FeedResult{}
	}

	raw := make([]byte, base64.StdEncoding.DecodedLen(len(d.data)))
	n, err := base64.StdEncoding.Decode(raw, d.data)
	if err != nil {
		resp := d.errorResponse(cd, "invalid base64 payload")
		d.reset()
		return FeedResult{Resp: resp}
	}
	raw = raw[:n]

	img, err := decodeImage(format, cd.width, cd.height, raw)
	if err != nil {
		resp := d.errorResponse(cd, err.Error())
		d.reset()
		return FeedResult{Resp: resp}
	}

	p := &Placement{ID: cd.id, Image: img, Cols: cd.cols, Rows: cd.rows}
	resp := d.okResponse(cd)
	d.reset()
	return FeedResult{Resp: resp, Placement: p}
}

func (d *Decoder) reset() {
	d.inFlight = false
	d.data = nil
	d.meta = controlData{}
}

func splitControlAndPayload(body []byte) (ctrl string, payload []byte) {
	i := bytes.IndexByte(body, ';')
	if i < 0 {
		return string(body), nil
	}
	return string(body[:i]), body[i+1:]
}

func decodeImage(format, width, height int, raw []byte) (image.Image, error) {
	switch format {
	case int(FormatPNG):
		return decodePNG(raw)
	case int(FormatRGB):
		return decodeRaw(width, height, raw, 3)
	case int(FormatRGBA):
		return decodeRaw(width, height, raw, 4)
	default:
		return nil, fmt.Errorf("unsupported format %d", format)
	}
}

func decodePNG(raw []byte) (image.Image, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid PNG payload: %w", err)
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("PNG image too large: %dx%d exceeds the %d pixel limit", cfg.Width, cfg.Height, maxPixels)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid PNG payload: %w", err)
	}
	return img, nil
}

func decodeRaw(width, height int, raw []byte, channels int) (image.Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("missing or invalid width/height (s=%d,v=%d) for a raw pixel format", width, height)
	}
	if width*height > maxPixels {
		return nil, fmt.Errorf("image too large: %dx%d exceeds the %d pixel limit", width, height, maxPixels)
	}
	want := width * height * channels
	if len(raw) != want {
		return nil, fmt.Errorf("raw pixel payload size mismatch: got %d bytes, want %d (%dx%d, %d channels)", len(raw), want, width, height, channels)
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	if channels == 4 {
		copy(img.Pix, raw)
	} else {
		for i := 0; i < width*height; i++ {
			img.Pix[i*4+0] = raw[i*3+0]
			img.Pix[i*4+1] = raw[i*3+1]
			img.Pix[i*4+2] = raw[i*3+2]
			img.Pix[i*4+3] = 255
		}
	}
	return img, nil
}

// okResponse builds the terminal's success reply, suppressed entirely
// when cd.quiet >= 1.
func (d *Decoder) okResponse(cd controlData) []byte {
	if cd.quiet >= 1 {
		return nil
	}
	return formatResponse(cd.id, cd.placementID, "OK")
}

// errorResponse builds the terminal's error reply, suppressed entirely
// when cd.quiet >= 2 (quiet level 1 only suppresses OK, per spec). Every
// error this package currently returns is a client-input problem (bad
// control data, unsupported scope, malformed payload), so the code is
// always "EINVAL" rather than a parameter — if a future milestone adds a
// distinct error class (e.g. an internal decode failure), give it one
// then.
func (d *Decoder) errorResponse(cd controlData, msg string) []byte {
	if cd.quiet >= 2 {
		return nil
	}
	return formatResponse(cd.id, cd.placementID, "EINVAL:"+msg)
}

func formatResponse(id, placementID uint32, payload string) []byte {
	var b strings.Builder
	b.WriteString("\x1b_G")
	if id != 0 {
		fmt.Fprintf(&b, "i=%d", id)
		if placementID != 0 {
			fmt.Fprintf(&b, ",p=%d", placementID)
		}
	}
	b.WriteByte(';')
	b.WriteString(payload)
	b.WriteString("\x1b\\")
	return []byte(b.String())
}
