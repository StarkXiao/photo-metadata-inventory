package main

import (
	"bytes"
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

func TestScanReportsAndExportsDateFromLinkedIFD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "linked-ifd.jpg")
	tiff := make([]byte, 100)
	copy(tiff, "II*\x00\x08\x00\x00\x00")
	tiff[8] = 0   // no entries in IFD0
	tiff[10] = 16 // IFD0's next-directory pointer
	tiff[16] = 1
	put16 := func(i int, v uint16) { tiff[i], tiff[i+1] = byte(v), byte(v>>8) }
	put32 := func(i int, v uint32) {
		tiff[i], tiff[i+1], tiff[i+2], tiff[i+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	put16(18, 0x9003)
	put16(20, 2)
	put32(22, 20)
	put32(26, 40)
	copy(tiff[40:], "2024:01:02 03:04:05\x00")
	jpeg := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(tiff) + 8)}, []byte("Exif\x00\x00")...)
	jpeg = append(jpeg, tiff...)
	jpeg = append(jpeg, 0xff, 0xd9)
	if err := os.WriteFile(path, jpeg, 0o600); err != nil {
		t.Fatal(err)
	}

	photos, err := scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 1 || photos[0].TakenAt.IsZero() || photos[0].TakenAt.Year() != 2024 {
		t.Fatalf("got photos=%+v", photos)
	}

	csvPath := filepath.Join(dir, "inventory.csv")
	if err := writeCSV(csvPath, photos); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := records[1][3]; !strings.HasPrefix(got, "2024-01-02T03:04:05") {
		t.Fatalf("captured_at = %q", got)
	}

	output := captureStdout(t, func() { printReport(photos) })
	if !strings.Contains(output, "2024-01") || !strings.Contains(output, "missing time: 0") {
		t.Fatalf("report did not group photo by capture month:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	var output bytes.Buffer
	if _, err := output.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestPNGEXIFRejectsTruncatedChunk(t *testing.T) {
	b := append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0, 8, 'e', 'X', 'I', 'f')
	if got := pngEXIF(b); got != nil {
		t.Fatalf("expected no EXIF, got %x", got)
	}
}
