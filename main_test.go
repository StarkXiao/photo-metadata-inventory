package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestParseTIFFPreservesGPSAcrossIFDs(t *testing.T) {
	b := make([]byte, 220)
	copy(b, "II*\x00\x08\x00\x00\x00")
	put16 := func(i int, v uint16) { b[i], b[i+1] = byte(v), byte(v>>8) }
	put32 := func(i int, v uint32) { b[i], b[i+1], b[i+2], b[i+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24) }
	entry := func(i int, tag uint16, typ uint16, n, v uint32) {
		put16(i, tag)
		put16(i+2, typ)
		put32(i+4, n)
		put32(i+8, v)
	}
	// Root IFD: a valid GPS directory and an Exif IFD.
	put16(8, 2)
	entry(10, 0x8825, 4, 1, 100)
	entry(22, 0x8769, 4, 1, 140)
	// The GPS directory contains latitude, while the Exif IFD points at an empty directory.
	put16(100, 1)
	entry(102, 0x0002, 5, 3, 0)
	put16(140, 1)
	entry(142, 0x8825, 4, 1, 180)
	put16(180, 0)

	_, _, gps, err := parseTIFF(b)
	if err != nil {
		t.Fatal(err)
	}
	if !gps {
		t.Fatal("expected GPS flag to remain true across IFDs")
	}

	path := filepath.Join(t.TempDir(), "inventory.csv")
	if err := writeCSV(path, []Photo{{Path: "photo.jpg", Format: "jpeg", HasGPS: gps}}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := records[1][5]; got != "true" {
		t.Fatalf("has_gps = %q, want true", got)
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

func TestWriteCSVPreservesGPSFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.csv")
	if err := writeCSV(path, []Photo{{Path: "photo.jpg", Format: "jpeg", HasGPS: true}}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "photo.jpg,jpeg,0,,,true,") {
		t.Fatalf("unexpected CSV: %s", b)
	}
}
