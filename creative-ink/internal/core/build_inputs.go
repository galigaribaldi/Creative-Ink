package core

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// texFamily are the extensions that are part of a LaTeX build by nature:
// saving one of them can change the PDF.
var texFamily = map[string]bool{
	".tex": true, ".sty": true, ".cls": true, ".bib": true, ".bst": true, ".bbx": true, ".cbx": true,
	".lbx": true, ".dbx": true, ".def": true, ".cfg": true, ".clo": true, ".fd": true, ".ltx": true,
	".dtx": true, ".ins": true, ".tikz": true, ".pgf": true,
}

// AffectsBuild reports whether saving file can change the PDF of target:
// a LaTeX-family file, or any file the last build read (the INPUT lines of
// latexmk's <target>.fls: a .csv for pgfplots, a .lua, a listing…). A
// README.md or notes.txt that no document reads is only saved, never
// compiled (docs/MAP.md §4.7, observation 4).
func AffectsBuild(target, file string) bool {
	if texFamily[strings.ToLower(filepath.Ext(file))] {
		return true
	}
	if target == "" {
		return false
	}
	return readByLastBuild(target, file)
}

// readByLastBuild looks for file among the INPUT lines of <target>.fls.
// Relative paths are relative to the PWD line (the folder TeX ran in).
func readByLastBuild(target, file string) bool {
	fls := strings.TrimSuffix(target, filepath.Ext(target)) + ".fls"
	f, err := os.Open(fls)
	if err != nil {
		return false // never compiled (or not by latexmk): nothing known
	}
	defer f.Close()
	want := filepath.Clean(file)
	pwd := filepath.Dir(target)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "PWD "):
			pwd = strings.TrimPrefix(line, "PWD ")
		case strings.HasPrefix(line, "INPUT "):
			p := strings.TrimPrefix(line, "INPUT ")
			if !filepath.IsAbs(p) {
				p = filepath.Join(pwd, p)
			}
			if filepath.Clean(p) == want {
				return true
			}
		}
	}
	return false
}
