package compiler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"creative-ink/internal/system"
)

// synctexTimeout bounds one synctex query (they normally take milliseconds).
const synctexTimeout = 5 * time.Second

// PDFBox is where a source line lands in the PDF: a 1-based page and a box
// in PDF points (1/72 in), origin at the top-left corner of the page.
type PDFBox struct {
	Page   int
	X      float64 // left
	Y      float64 // top
	Width  float64
	Height float64
}

// SourceLine is where a point of the PDF comes from.
type SourceLine struct {
	File string // absolute, cleaned
	Line int    // 1-based
}

// SyncTeXForward (source → PDF) runs
//
//	synctex view -i <line>:0:<texFile> -o <pdfPath>
//
// found is false, without error, when there is nothing to report: no
// .synctex.gz yet (never compiled with -synctex=1), synctex not installed,
// or no box for that line. Callers show a hint instead of an error.
func SyncTeXForward(ctx context.Context, pdfPath, texFile string, line int) (box PDFBox, found bool) {
	if line < 1 || !hasSyncTeXData(pdfPath) {
		return PDFBox{}, false
	}
	out, ok := runSyncTeX(ctx, pdfPath, "view", "-i", fmt.Sprintf("%d:0:%s", line, texFile), "-o", pdfPath)
	if !ok {
		return PDFBox{}, false
	}
	return parseSyncTeXView(out)
}

// SyncTeXBackward (PDF → source) runs
//
//	synctex edit -o <page>:<x>:<y>:<pdfPath>
//
// with x, y in PDF points from the top-left corner of the page. The returned
// file is absolute (synctex may report it relative to the PDF's folder).
func SyncTeXBackward(ctx context.Context, pdfPath string, page int, x, y float64) (src SourceLine, found bool) {
	if page < 1 || x < 0 || y < 0 || !hasSyncTeXData(pdfPath) {
		return SourceLine{}, false
	}
	query := fmt.Sprintf("%d:%s:%s:%s", page, formatPoint(x), formatPoint(y), pdfPath)
	out, ok := runSyncTeX(ctx, pdfPath, "edit", "-o", query)
	if !ok {
		return SourceLine{}, false
	}
	src, found = parseSyncTeXEdit(out)
	if found && !filepath.IsAbs(src.File) {
		src.File = filepath.Join(filepath.Dir(pdfPath), src.File)
	}
	src.File = filepath.Clean(src.File)
	return src, found
}

// SyncTeXDataPath returns the .synctex.gz written next to the PDF.
func SyncTeXDataPath(pdfPath string) string {
	return strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + ".synctex.gz"
}

func hasSyncTeXData(pdfPath string) bool {
	for _, p := range []string{SyncTeXDataPath(pdfPath), strings.TrimSuffix(SyncTeXDataPath(pdfPath), ".gz")} {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

// runSyncTeX executes synctex directly (no shell), next to the PDF. Any
// failure is logged and reported as ok=false: SyncTeX is best effort.
func runSyncTeX(parent context.Context, pdfPath string, args ...string) (string, bool) {
	bin, found := system.FindBinary("synctex")
	if !found {
		log.Printf("[synctex] no se encontró el binario synctex")
		return "", false
	}
	ctx, cancel := context.WithTimeout(parent, synctexTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = filepath.Dir(pdfPath)
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.WaitDelay = time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		log.Printf("[synctex] %s: %v", args[0], err)
		return "", false
	}
	return out.String(), true
}

// syncTeXRecord returns the key/value pairs of the first record between
// "SyncTeX result begin" and "SyncTeX result end". synctex may print several
// records (one per matching box); the first is the best match.
func syncTeXRecord(out string, firstKey string) map[string]string {
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	inResult := false
	record := map[string]string{}
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case line == "SyncTeX result begin":
			inResult = true
			continue
		case line == "SyncTeX result end":
			return record
		case !inResult:
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if key == firstKey && len(record) > 0 {
			return record // a second record starts: keep the first one
		}
		record[key] = value
	}
	return record
}

// parseSyncTeXView reads the output of `synctex view`. h/v are the box's
// left edge and baseline, W/H its width and height (all in points, from the
// page's top-left corner), so the box top is v - H.
func parseSyncTeXView(out string) (PDFBox, bool) {
	r := syncTeXRecord(out, "Output")
	page, err := strconv.Atoi(r["Page"])
	if err != nil || page < 1 {
		return PDFBox{}, false
	}
	h, okH := parseFloat(r["h"])
	v, okV := parseFloat(r["v"])
	if !okH || !okV { // older versions: only x/y (the box's reference point)
		if h, okH = parseFloat(r["x"]); !okH {
			return PDFBox{}, false
		}
		if v, okV = parseFloat(r["y"]); !okV {
			return PDFBox{}, false
		}
	}
	w, _ := parseFloat(r["W"])
	ht, _ := parseFloat(r["H"])
	return PDFBox{Page: page, X: h, Y: v - ht, Width: w, Height: ht}, true
}

// parseSyncTeXEdit reads the output of `synctex edit`.
func parseSyncTeXEdit(out string) (SourceLine, bool) {
	r := syncTeXRecord(out, "Output")
	file := strings.TrimSpace(r["Input"])
	line, err := strconv.Atoi(r["Line"])
	if file == "" || err != nil || line < 1 {
		return SourceLine{}, false
	}
	return SourceLine{File: file, Line: line}, true
}

func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f, err == nil
}

func formatPoint(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
