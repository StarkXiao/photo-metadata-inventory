package main

import (
	"bytes"
	"testing"
	"time"
)

func TestParseTIFF(t *testing.T) {
	b := make([]byte, 220)
	copy(b, "II*\x00\x08\x00\x00\x00")
	b[8] = 4
	put16 := func(i int, v uint16) { b[i], b[i+1] = byte(v), byte(v>>8) }
	put32 := func(i int, v uint32) { b[i], b[i+1], b[i+2], b[i+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24) }
	entry := func(i int, tag uint16, n, v uint32) { put16(i, tag); put16(i+2, 2); put32(i+4, n); put32(i+8, v) }
	entry(10, 0x010f, 5, 80)
	entry(22, 0x0110, 6, 85)
	put16(34, 0x8769)
	put16(36, 4)
	put32(38, 1)
	put32(42, 120)
	put16(46, 0x8825)
	put16(48, 4)
	put32(50, 1)
	put32(54, 200)
	b[120] = 1
	entry(122, 0x9003, 20, 150)
	copy(b[80:], "Acme\x00Model\x00")
	copy(b[150:], "2024:01:02 03:04:05\x00")
	put16(200, 1)
	entry(202, 0x0002, 3, 0)
	taken, camera, gps, err := parseTIFF(b)
	if err != nil {
		t.Fatal(err)
	}
	if camera != "Acme Model" || !gps || taken.Year() != 2024 {
		t.Fatalf("got camera=%q gps=%v date=%v", camera, gps, taken)
	}
}

func TestParseTIFFDoesNotFlagEmptyGPSDirectory(t *testing.T) {
	b := make([]byte, 40)
	copy(b, "II*\x00\x08\x00\x00\x00")
	b[8] = 1
	b[10], b[11] = 0x25, 0x88 // GPSInfoIFDPointer
	b[12], b[13] = 4, 0
	b[14] = 1
	b[18] = 30
	if _, _, gps, err := parseTIFF(b); err != nil || gps {
		t.Fatalf("got gps=%v err=%v", gps, err)
	}
}

func TestParseTIFFHandlesCyclicIFD(t *testing.T) {
	b := make([]byte, 32)
	copy(b, "II*\x00\x08\x00\x00\x00")
	b[8] = 1
	b[10], b[11] = 0x69, 0x87 // ExifIFDPointer
	b[12], b[13] = 4, 0
	b[14] = 1
	b[18] = 8 // points back to the root directory
	if _, _, _, err := parseTIFF(b); err != nil {
		t.Fatal(err)
	}
}

func TestPNGEXIFRejectsTruncatedChunk(t *testing.T) {
	b := append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0, 8, 'e', 'X', 'I', 'f')
	if got := pngEXIF(b); got != nil {
		t.Fatalf("expected no EXIF, got %x", got)
	}
}

func TestJPEGEXIFReadsAPP1Payload(t *testing.T) {
	b := []byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x0a, 'E', 'x', 'i', 'f', 0, 0, 'I', 'I'}
	if got := jpegEXIF(b); !bytes.Equal(got, []byte("II")) {
		t.Fatalf("got %x", got)
	}
}

func TestExtractJPEGStandardEXIF(t *testing.T) {
	tiff := make([]byte, 180)
	copy(tiff, "II*\x00\x08\x00\x00\x00")
	put16 := func(i int, v uint16) { tiff[i], tiff[i+1] = byte(v), byte(v>>8) }
	put32 := func(i int, v uint32) {
		tiff[i], tiff[i+1], tiff[i+2], tiff[i+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	entry := func(i int, tag uint16, count, value uint32) {
		put16(i, tag)
		put16(i+2, 2)
		put32(i+4, count)
		put32(i+8, value)
	}

	// IFD0 contains the usual Make, Model, and Exif IFD pointer entries.
	put16(8, 3)
	entry(10, 0x010f, 6, 80)
	entry(22, 0x0110, 9, 86)
	put16(34, 0x8769)
	put16(36, 4)
	put32(38, 1)
	put32(42, 112)
	put32(46, 0)
	copy(tiff[80:], "Canon\x00EOS R8\x00")

	put16(112, 1)
	entry(114, 0x9003, 20, 130)
	put32(126, 0)
	copy(tiff[130:], "2024:01:02 03:04:05\x00")

	exif := append([]byte("Exif\x00\x00"), tiff...)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x04, 'J', 'F'}
	jpeg = append(jpeg, 0xff, 0xe1, byte((len(exif)+2)>>8), byte(len(exif)+2))
	jpeg = append(jpeg, exif...)
	jpeg = append(jpeg, 0xff, 0xd9)

	taken, camera, _, err := extract("jpeg", jpeg)
	if err != nil {
		t.Fatal(err)
	}
	if camera != "Canon EOS R8" {
		t.Fatalf("camera = %q, want Canon EOS R8", camera)
	}
	wantTaken := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.Local)
	if !taken.Equal(wantTaken) {
		t.Fatalf("taken = %v, want %v", taken, wantTaken)
	}
}
