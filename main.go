package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Photo struct {
	Path       string
	Format     string
	Size       int64
	TakenAt    time.Time
	Camera     string
	HasGPS     bool
	ParseError error
}

type summary struct {
	Count, MissingTime, WithGPS int
	Bytes                       int64
}

func main() {
	root := flag.String("path", ".", "directory to scan recursively")
	csvPath := flag.String("csv", "", "write per-photo results to this CSV file")
	safe := flag.Bool("safe", false, "show privacy and metadata safety recommendations")
	flag.Parse()

	photos, err := scan(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(1)
	}
	printReport(photos)
	if *csvPath != "" {
		if err := writeCSV(*csvPath, photos); err != nil {
			fmt.Fprintln(os.Stderr, "csv:", err)
			os.Exit(1)
		}
		fmt.Printf("\nCSV exported: %s\n", *csvPath)
	}
	if *safe {
		printSafety(photos)
	}
}

func scan(root string) ([]Photo, error) {
	var photos []Photo
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if format == "jpg" {
			format = "jpeg"
		}
		if format != "jpeg" && format != "png" && format != "heic" && format != "heif" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		p := Photo{Path: path, Format: format, Size: info.Size()}
		data, err := os.ReadFile(path)
		if err != nil {
			p.ParseError = err
		} else {
			p.TakenAt, p.Camera, p.HasGPS, p.ParseError = extract(format, data)
		}
		photos = append(photos, p)
		return nil
	})
	sort.Slice(photos, func(i, j int) bool { return photos[i].Path < photos[j].Path })
	return photos, err
}

func printReport(photos []Photo) {
	byMonth, byCamera := map[string]*summary{}, map[string]*summary{}
	for _, p := range photos {
		month := "unknown"
		if !p.TakenAt.IsZero() {
			month = p.TakenAt.Format("2006-01")
		}
		camera := p.Camera
		if camera == "" {
			camera = "unknown"
		}
		for _, m := range []*summary{get(byMonth, month), get(byCamera, camera)} {
			m.Count++
			m.Bytes += p.Size
			if p.TakenAt.IsZero() {
				m.MissingTime++
			}
			if p.HasGPS {
				m.WithGPS++
			}
		}
	}
	fmt.Printf("Photos found: %d\n\nBy month:\n", len(photos))
	printSummaries(byMonth)
	fmt.Println("\nBy device:")
	printSummaries(byCamera)
	fmt.Println("\nNeeds attention:")
	for _, p := range photos {
		if p.TakenAt.IsZero() || p.HasGPS {
			fmt.Printf("- %s [%s]\n", p.Path, issue(p))
		}
	}
}
func get(m map[string]*summary, key string) *summary {
	if m[key] == nil {
		m[key] = &summary{}
	}
	return m[key]
}
func printSummaries(m map[string]*summary) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		fmt.Printf("%-24s %4d files  %9d bytes  missing time: %d  GPS: %d\n", k, v.Count, v.Bytes, v.MissingTime, v.WithGPS)
	}
}
func issue(p Photo) string {
	var s []string
	if p.TakenAt.IsZero() {
		s = append(s, "missing capture time")
	}
	if p.HasGPS {
		s = append(s, "GPS privacy risk")
	}
	return strings.Join(s, "; ")
}

func writeCSV(path string, photos []Photo) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{"path", "format", "size_bytes", "captured_at", "camera", "has_gps", "issues", "metadata_error"}); err != nil {
		_ = f.Close()
		return err
	}
	for _, p := range photos {
		when, parseErr := "", ""
		if !p.TakenAt.IsZero() {
			when = p.TakenAt.Format(time.RFC3339)
		}
		if p.ParseError != nil {
			parseErr = p.ParseError.Error()
		}
		if err := w.Write([]string{p.Path, p.Format, fmt.Sprint(p.Size), when, p.Camera, fmt.Sprint(p.HasGPS), issue(p), parseErr}); err != nil {
			_ = f.Close()
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
func printSafety(photos []Photo) {
	gps, missing := 0, 0
	for _, p := range photos {
		if p.HasGPS {
			gps++
		}
		if p.TakenAt.IsZero() {
			missing++
		}
	}
	fmt.Printf("\nSafety recommendations:\n- %d file(s) contain GPS coordinates. Remove location metadata before sharing publicly.\n- %d file(s) lack capture time. Preserve originals and record dates separately before archiving.\n- Export copies for sharing; keep originals offline and retain a backup.\n", gps, missing)
}

func extract(format string, data []byte) (time.Time, string, bool, error) {
	var tiff []byte
	switch format {
	case "jpeg":
		tiff = jpegEXIF(data)
	case "png":
		tiff = pngEXIF(data)
	case "heic", "heif":
		tiff = heifEXIF(data)
	}
	if len(tiff) == 0 {
		return time.Time{}, "", false, errors.New("EXIF metadata not found")
	}
	return parseTIFF(tiff)
}

func jpegEXIF(b []byte) []byte {
	if len(b) < 4 || b[0] != 0xff || b[1] != 0xd8 {
		return nil
	}
	for i := 2; i+1 < len(b); {
		if b[i] != 0xff {
			i++
			continue
		}
		for i+1 < len(b) && b[i+1] == 0xff {
			i++
		}
		if i+1 >= len(b) {
			break
		}
		marker := b[i+1]
		if marker == 0xda || marker == 0xd9 {
			break
		}
		if marker == 0x00 || marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			i += 2
			continue
		}
		if i+4 > len(b) {
			break
		}
		n := int(b[i+2])<<8 | int(b[i+3])
		if n < 2 || i+2+n > len(b) {
			break
		}
		p := b[i+4 : i+2+n]
		if marker == 0xe1 && len(p) >= 6 && bytes.Equal(p[:6], []byte("Exif\x00\x00")) {
			return p[6:]
		}
		i += n + 2
	}
	return nil
}
func pngEXIF(b []byte) []byte {
	if len(b) < 8 || !bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return nil
	}
	for i := 8; i+12 <= len(b); {
		n := uint64(b[i])<<24 | uint64(b[i+1])<<16 | uint64(b[i+2])<<8 | uint64(b[i+3])
		if n > uint64(len(b)-i-12) {
			return nil
		}
		end := i + 8 + int(n)
		if bytes.Equal(b[i+4:i+8], []byte("eXIf")) {
			return b[i+8 : end]
		}
		i += int(n) + 12
	}
	return nil
}
func heifEXIF(b []byte) []byte {
	for _, mark := range [][]byte{[]byte("Exif\x00\x00"), []byte("Exif")} {
		if i := bytes.Index(b, mark); i >= 0 {
			p := b[i+len(mark):]
			if len(p) >= 4 {
				p = p[4:]
			}
			if t := findTIFF(p); t != nil {
				return t
			}
		}
	}
	return findTIFF(b)
}
func findTIFF(b []byte) []byte {
	for i := 0; i+8 <= len(b); i++ {
		if bytes.Equal(b[i:i+4], []byte("II*\x00")) || bytes.Equal(b[i:i+4], []byte("MM\x00*")) {
			return b[i:]
		}
	}
	return nil
}

type reader struct {
	b  []byte
	le bool
}

func (r reader) u16(i int) (uint16, bool) {
	if i < 0 || i > len(r.b)-2 {
		return 0, false
	}
	if r.le {
		return uint16(r.b[i]) | uint16(r.b[i+1])<<8, true
	}
	return uint16(r.b[i])<<8 | uint16(r.b[i+1]), true
}
func (r reader) u32(i int) (uint32, bool) {
	if i < 0 || i > len(r.b)-4 {
		return 0, false
	}
	if r.le {
		return uint32(r.b[i]) | uint32(r.b[i+1])<<8 | uint32(r.b[i+2])<<16 | uint32(r.b[i+3])<<24, true
	}
	return uint32(r.b[i])<<24 | uint32(r.b[i+1])<<16 | uint32(r.b[i+2])<<8 | uint32(r.b[i+3]), true
}
func parseTIFF(b []byte) (time.Time, string, bool, error) {
	if len(b) < 8 || (string(b[:2]) != "II" && string(b[:2]) != "MM") {
		return time.Time{}, "", false, errors.New("invalid TIFF header")
	}
	r := reader{b, string(b[:2]) == "II"}
	magic, ok := r.u16(2)
	if !ok || magic != 42 {
		return time.Time{}, "", false, errors.New("invalid TIFF magic")
	}
	first, ok := r.u32(4)
	if !ok {
		return time.Time{}, "", false, errors.New("missing TIFF directory")
	}
	var date, makeName, model string
	gps := false
	visited := make(map[uint32]bool)
	var visit func(uint32)
	visit = func(off uint32) {
		if visited[off] || uint64(off)+2 > uint64(len(b)) {
			return
		}
		visited[off] = true
		n, ok := r.u16(int(off))
		if !ok {
			return
		}
		for i := 0; i < int(n); i++ {
			pos := int(off) + 2 + i*12
			tag, ok1 := r.u16(pos)
			typ, ok2 := r.u16(pos + 2)
			count, ok3 := r.u32(pos + 4)
			val, ok4 := r.u32(pos + 8)
			if !ok1 || !ok2 || !ok3 || !ok4 {
				continue
			}
			if tag == 0x8825 && uint64(val)+2 <= uint64(len(b)) {
				gps = hasGPSCoordinates(r, val)
				continue
			}
			if tag == 0x8769 && val < uint32(len(b)) {
				visit(val)
				continue
			}
			if tag == 0x0110 {
				model = ascii(r, typ, count, val)
			}
			if tag == 0x010f {
				makeName = ascii(r, typ, count, val)
			}
			if tag == 0x9003 || tag == 0x9004 || tag == 0x0132 {
				if date == "" {
					date = ascii(r, typ, count, val)
				}
			}
		}
	}
	visit(first)
	camera := strings.TrimSpace(strings.TrimSpace(makeName) + " " + strings.TrimSpace(model))
	var taken time.Time
	if date != "" {
		taken, _ = time.ParseInLocation("2006:01:02 15:04:05", strings.TrimSpace(date), time.Local)
	}
	return taken, camera, gps, nil
}

func hasGPSCoordinates(r reader, off uint32) bool {
	n, ok := r.u16(int(off))
	if !ok {
		return false
	}
	for i := 0; i < int(n); i++ {
		pos := int(off) + 2 + i*12
		tag, ok := r.u16(pos)
		if !ok {
			return false
		}
		if tag == 0x0002 || tag == 0x0004 {
			return true
		}
	}
	return false
}
func ascii(r reader, typ uint16, count, val uint32) string {
	if typ != 2 || count == 0 {
		return ""
	}
	start := int(val)
	if count <= 4 {
		raw := make([]byte, 4)
		if r.le {
			raw[0] = byte(val)
			raw[1] = byte(val >> 8)
			raw[2] = byte(val >> 16)
			raw[3] = byte(val >> 24)
		} else {
			raw[0] = byte(val >> 24)
			raw[1] = byte(val >> 16)
			raw[2] = byte(val >> 8)
			raw[3] = byte(val)
		}
		return strings.TrimRight(string(raw[:count]), "\x00")
	}
	if start < 0 || uint64(count) > uint64(len(r.b)-start) {
		return ""
	}
	return strings.TrimRight(string(r.b[start:start+int(count)]), "\x00")
}
