// Command ico packs one or more same-format PNG files into a Windows .ico
// container using the PNG-in-ICO extension (valid since Windows Vista —
// each directory entry's image data is a complete, unmodified PNG file,
// rather than the older uncompressed BMP format). Used by
// scripts/gen-icons.sh to build build/windows/icon.ico from the PNGs
// sips produces from assets/icon.png; not a general-purpose ICO tool.
//
// Usage: go run ./scripts/gen-icon/ico -out icon.ico icon-16.png icon-32.png ...
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
)

type icoEntry struct {
	width, height int
	data          []byte
}

func main() {
	out := flag.String("out", "", "output .ico path")
	flag.Parse()
	pngPaths := flag.Args()
	if *out == "" || len(pngPaths) == 0 {
		log.Fatal("usage: ico -out icon.ico icon-16.png icon-32.png ...")
	}

	if err := run(*out, pngPaths); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", *out)
}

func run(out string, pngPaths []string) error {
	entries, err := loadEntries(pngPaths)
	if err != nil {
		return err
	}

	// out is this command's own -out flag value (a build-script-supplied
	// path, see scripts/gen-icons.sh), not user/network-controlled input.
	f, err := os.Create(out) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return writeICO(f, entries)
}

func loadEntries(pngPaths []string) ([]icoEntry, error) {
	entries := make([]icoEntry, 0, len(pngPaths))
	for _, p := range pngPaths {
		// p is one of this command's own CLI arguments (a build-script-
		// supplied path, see scripts/gen-icons.sh), not user/network-
		// controlled input.
		data, err := os.ReadFile(p) //nolint:gosec
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decoding %s: %w", p, err)
		}
		if cfg.Width > 256 || cfg.Height > 256 {
			return nil, fmt.Errorf("%s is %dx%d, ICO entries must be at most 256x256", p, cfg.Width, cfg.Height)
		}
		entries = append(entries, icoEntry{width: cfg.Width, height: cfg.Height, data: data})
	}
	return entries, nil
}

func writeICO(f *os.File, entries []icoEntry) error {
	// ICONDIR header: reserved(2)=0, type(2)=1 (icon), count(2).
	header := make([]byte, 6)
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], uint16(len(entries)))
	if _, err := f.Write(header); err != nil {
		return err
	}

	offset := uint32(6 + 16*len(entries))
	for _, e := range entries {
		dirEntry := make([]byte, 16)
		dirEntry[0] = byte(e.width % 256) // 0 means 256, per the ICO format
		dirEntry[1] = byte(e.height % 256)
		dirEntry[2] = 0                                 // no palette
		dirEntry[3] = 0                                 // reserved
		binary.LittleEndian.PutUint16(dirEntry[4:], 1)  // color planes
		binary.LittleEndian.PutUint16(dirEntry[6:], 32) // bits per pixel
		binary.LittleEndian.PutUint32(dirEntry[8:], uint32(len(e.data)))
		binary.LittleEndian.PutUint32(dirEntry[12:], offset)
		if _, err := f.Write(dirEntry); err != nil {
			return err
		}
		offset += uint32(len(e.data))
	}
	for _, e := range entries {
		if _, err := f.Write(e.data); err != nil {
			return err
		}
	}
	return nil
}
