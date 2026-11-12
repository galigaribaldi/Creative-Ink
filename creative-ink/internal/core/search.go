package core

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Searching the project (docs/MAP.md §4.4, #9): the "Buscar" tool of the
// sidebar, like VS Code's. Go walks the text files of the open folder with
// the explorer's filters (no hidden entries, build artifacts or
// node_modules) and returns the matching lines. Read only, bounded.

// Search limits: a project, not a disk.
const (
	maxSearchFiles   = 20_000
	maxSearchMatches = 2_000
	maxPreviewRunes  = 160
)

// ErrBadPattern: the regular expression does not compile.
var ErrBadPattern = errors.New("expresión regular no válida")

// SearchQuery is what the user typed and the toggles (Aa, palabra, .*).
type SearchQuery struct {
	Text          string `json:"text"`
	CaseSensitive bool   `json:"caseSensitive"`
	WholeWord     bool   `json:"wholeWord"`
	Regex         bool   `json:"regex"`
}

// SearchMatch is one occurrence: line and column from 1 (column in
// characters), and the line trimmed around it.
type SearchMatch struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Length  int    `json:"length"` // in characters
	Preview string `json:"preview"`
	// Start/End locate the match inside Preview (characters).
	Start int `json:"start"`
	End   int `json:"end"`
}

// SearchFileResult groups the matches of a file.
type SearchFileResult struct {
	Path    string        `json:"path"` // absolute
	Rel     string        `json:"rel"`  // with "/" from the folder
	Matches []SearchMatch `json:"matches"`
}

// SearchResult is the whole answer.
type SearchResult struct {
	Files     []SearchFileResult `json:"files"`
	Total     int                `json:"total"`     // matches returned
	Truncated bool               `json:"truncated"` // a limit was hit: refine the search
}

// CompileQuery turns the query into a regular expression (literal text is
// quoted). Whole word uses \b, which in Go only knows ASCII letters: "acción"
// still matches as a word, but a boundary next to "ó" is not detected.
func CompileQuery(q SearchQuery) (*regexp.Regexp, error) {
	pattern := q.Text
	if !q.Regex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if q.WholeWord {
		pattern = `\b(?:` + pattern + `)\b`
	}
	if !q.CaseSensitive {
		pattern = `(?i)` + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadPattern, err)
	}
	return re, nil
}

// SearchFolder searches the text files under root (or only the file only,
// for a loose file). An empty query returns nothing.
func SearchFolder(ctx context.Context, root, only string, q SearchQuery) (SearchResult, error) {
	res := SearchResult{Files: []SearchFileResult{}}
	if strings.TrimSpace(q.Text) == "" {
		return res, nil
	}
	re, err := CompileQuery(q)
	if err != nil {
		return res, err
	}
	files := 0
	visit := func(path string) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if files++; files > maxSearchFiles {
			res.Truncated = true
			return filepath.SkipAll
		}
		if fr := searchFile(path, re, maxSearchMatches-res.Total); len(fr.Matches) > 0 {
			rel, _ := filepath.Rel(root, path)
			fr.Rel = filepath.ToSlash(rel)
			res.Files = append(res.Files, fr)
			res.Total += len(fr.Matches)
			if res.Total >= maxSearchMatches {
				res.Truncated = true
				return filepath.SkipAll
			}
		}
		return nil
	}
	if only != "" {
		return res, visit(only)
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip it
		}
		if path == root {
			return nil
		}
		if isIgnoredEntry(d.Name(), d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() || !IsEditableFile(d.Name()) {
			return nil
		}
		return visit(path)
	})
	if errors.Is(err, filepath.SkipAll) {
		err = nil
	}
	return res, err
}

// searchFile returns up to limit matches of re in a UTF-8 text file (binary
// or too large files are skipped, as the editor would not open them).
func searchFile(path string, re *regexp.Regexp, limit int) SearchFileResult {
	fr := SearchFileResult{Path: path, Matches: []SearchMatch{}}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > MaxEditableBytes || bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return fr
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64<<10), MaxEditableBytes)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		for _, loc := range re.FindAllStringIndex(text, -1) {
			if loc[0] == loc[1] {
				continue // empty match (e.g. "a*"): nothing to show
			}
			fr.Matches = append(fr.Matches, matchAt(text, line, loc[0], loc[1]))
			if len(fr.Matches) >= limit {
				return fr
			}
		}
	}
	return fr
}

// matchAt builds the match with a preview of the line trimmed around it.
func matchAt(text string, line, start, end int) SearchMatch {
	col := utf8.RuneCountInString(text[:start])
	length := utf8.RuneCountInString(text[start:end])
	runes := []rune(text)
	from := max(0, col-40)
	to := min(len(runes), max(col+length+(maxPreviewRunes-40-length), col+length))
	if to-from > maxPreviewRunes {
		to = from + maxPreviewRunes
	}
	prefix := ""
	if from > 0 {
		prefix = "…"
	}
	lead := len([]rune(strings.TrimLeft(string(runes[from:col]), " \t")))
	preview := strings.TrimLeft(string(runes[from:to]), " \t")
	startInPreview := len([]rune(prefix)) + lead
	return SearchMatch{
		Line: line, Column: col + 1, Length: length,
		Preview: prefix + preview,
		Start:   startInPreview,
		End:     min(startInPreview+length, len([]rune(prefix+preview))),
	}
}
