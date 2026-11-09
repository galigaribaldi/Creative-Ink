package core

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Large-project detector (docs/MAP.md §4.5): three independent schemes that
// predict slow compilations and a heavy preview. It only INFORMS: the UI
// shows a warning with a tooltip and nothing changes in how the project is
// compiled or shown.
//
// Thresholds are package variables so tests can lower them.
var (
	// Too many files: visible files of the project, or LaTeX sources.
	maxProjectFiles = 1500
	maxTeXFiles     = 100
	// Files that are too full: one source over this size or line count.
	maxSourceBytes = int64(300 * 1024)
	maxSourceLines = 4000
	// Too many images: \includegraphics / \includepdf / \includesvg calls.
	maxImageCalls = 60
)

// Scan limits keep the analysis cheap on any folder (it runs in the
// background after opening a project and after saves).
const (
	healthMaxDepth   = 10
	healthMaxEntries = 20000
	healthMaxReads   = 3000
)

// Warning kinds.
const (
	HealthTooManyFiles = "files"
	HealthLargeFiles   = "size"
	HealthManyImages   = "images"
)

var (
	imageCall  = regexp.MustCompile(`\\(?:includegraphics|includepdf|includesvg)\b`)
	imageExts  = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".pdf": true, ".eps": true, ".svg": true, ".tif": true, ".tiff": true}
	sourceExts = map[string]bool{".tex": true, ".bib": true, ".sty": true, ".cls": true}
)

// ProjectWarning is one triggered scheme.
type ProjectWarning struct {
	Kind    string `json:"kind"` // HealthTooManyFiles | HealthLargeFiles | HealthManyImages
	Message string `json:"message"`
}

// ProjectHealth is the result of AnalyzeProject.
type ProjectHealth struct {
	Folder   string           `json:"folder"`
	Warnings []ProjectWarning `json:"warnings"` // empty: nothing to report
	// Figures behind the warnings (also shown in the tooltip).
	Files        int    `json:"files"`
	TeXFiles     int    `json:"texFiles"`
	LargestFile  string `json:"largestFile"` // relative, "" if none
	LargestBytes int64  `json:"largestBytes"`
	LargestLines int    `json:"largestLines"`
	ImageCalls   int    `json:"imageCalls"`
	ImageBytes   int64  `json:"imageBytes"` // image files in the folder
	Truncated    bool   `json:"truncated"`  // the scan hit its limits
}

// AnalyzeProject measures folder against the three schemes. Errors only
// shorten the analysis (unreadable entries are skipped).
func AnalyzeProject(folder string) ProjectHealth {
	h := ProjectHealth{Folder: folder, Warnings: []ProjectWarning{}}
	var big []string // sources over the size/line limits
	entries, reads := 0, 0

	_ = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == folder {
			return nil
		}
		rel, _ := filepath.Rel(folder, path)
		if d.Type()&fs.ModeSymlink != 0 || isIgnoredEntry(d.Name(), d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entries++; entries > healthMaxEntries {
			h.Truncated = true
			return filepath.SkipAll
		}
		if d.IsDir() {
			if strings.Count(rel, string(filepath.Separator))+1 >= healthMaxDepth {
				h.Truncated = true
				return filepath.SkipDir
			}
			return nil
		}

		h.Files++
		ext := strings.ToLower(filepath.Ext(path))
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if imageExts[ext] && !isOwnPDF(path) {
			h.ImageBytes += info.Size()
		}
		if !sourceExts[ext] {
			return nil
		}
		if ext == ".tex" {
			h.TeXFiles++
		}
		if reads >= healthMaxReads || info.Size() > MaxEditableBytes {
			h.Truncated = h.Truncated || reads >= healthMaxReads
			if info.Size() > maxSourceBytes {
				big = append(big, filepath.ToSlash(rel))
			}
			return nil
		}
		reads++
		lines, calls := scanSource(path, ext == ".tex")
		h.ImageCalls += calls
		if info.Size() > h.LargestBytes {
			h.LargestFile, h.LargestBytes, h.LargestLines = filepath.ToSlash(rel), info.Size(), lines
		}
		if info.Size() > maxSourceBytes || lines > maxSourceLines {
			big = append(big, filepath.ToSlash(rel))
		}
		return nil
	})

	if h.Files > maxProjectFiles || h.TeXFiles > maxTeXFiles {
		h.Warnings = append(h.Warnings, ProjectWarning{HealthTooManyFiles, fmt.Sprintf(
			"Demasiados archivos: %d en el proyecto, %d .tex (avisa a partir de %d archivos o %d .tex).",
			h.Files, h.TeXFiles, maxProjectFiles, maxTeXFiles)})
	}
	if len(big) > 0 {
		slices.Sort(big)
		shown := big
		if len(shown) > 3 {
			shown = append(slices.Clone(big[:3]), fmt.Sprintf("y %d más", len(big)-3))
		}
		h.Warnings = append(h.Warnings, ProjectWarning{HealthLargeFiles, fmt.Sprintf(
			"Archivos muy llenos (más de %d KB o %d líneas): %s. Divídelos con \\input{…}.",
			maxSourceBytes/1024, maxSourceLines, strings.Join(shown, ", "))})
	}
	if h.ImageCalls > maxImageCalls {
		h.Warnings = append(h.Warnings, ProjectWarning{HealthManyImages, fmt.Sprintf(
			"Demasiadas imágenes: %d llamadas a \\includegraphics (avisa a partir de %d); %.0f MB de imágenes en la carpeta.",
			h.ImageCalls, maxImageCalls, float64(h.ImageBytes)/(1<<20))})
	}
	return h
}

// scanSource counts lines and, for .tex, image calls outside % comments.
func scanSource(path string, countImages bool) (lines, calls int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20) // long lines are counted, not split
	for sc.Scan() {
		lines++
		if countImages {
			calls += len(imageCall.FindAllStringIndex(stripTeXComment(sc.Text()), -1))
		}
	}
	return lines, calls
}

// stripTeXComment drops a line's % comment (a \% is a literal percent).
func stripTeXComment(line string) string {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++ // skip the escaped character
		case '%':
			return line[:i]
		}
	}
	return line
}

// isOwnPDF reports whether a PDF is a compiled document of the project
// (next to a .tex with the same name), not an image.
func isOwnPDF(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		return false
	}
	return isRegularFile(strings.TrimSuffix(path, filepath.Ext(path)) + ".tex")
}
