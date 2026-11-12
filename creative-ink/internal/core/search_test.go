package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchFolderFindsTextAcrossTheProject(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"main.tex":            "\\documentclass{article}\n\\begin{document}\nLa Movilidad urbana.\n\\end{document}\n",
		"secciones/intro.tex": "Hablemos de movilidad y de inmovilidad.\n",
		"refs.bib":            "@book{movilidad2020, title={Movilidad}}\n",
		"main.aux":            "movilidad\n", // build artifact
		".git/COMMIT_EDITMSG": "movilidad\n", // hidden
		"node_modules/x.js":   "movilidad\n", // skipped folder
		"foto.png":            "movilidad\n", // not a text file
	})
	res, err := SearchFolder(context.Background(), root, "", SearchQuery{Text: "movilidad"})
	require.NoError(t, err)
	var rels []string
	for _, f := range res.Files {
		rels = append(rels, f.Rel)
	}
	assert.ElementsMatch(t, []string{"main.tex", "secciones/intro.tex", "refs.bib"}, rels)
	assert.Equal(t, 5, res.Total, "case-insensitive by default; inmovilidad counts")

	whole, err := SearchFolder(context.Background(), root, "", SearchQuery{Text: "movilidad", WholeWord: true})
	require.NoError(t, err)
	assert.Equal(t, 3, whole.Total, "inmovilidad and movilidad2020 are not the whole word")

	cased, err := SearchFolder(context.Background(), root, "", SearchQuery{Text: "Movilidad", CaseSensitive: true})
	require.NoError(t, err)
	assert.Equal(t, 2, cased.Total)

	re, err := SearchFolder(context.Background(), root, "", SearchQuery{Text: `\\(begin|end)\{document\}`, Regex: true})
	require.NoError(t, err)
	assert.Equal(t, 2, re.Total)
	_, err = SearchFolder(context.Background(), root, "", SearchQuery{Text: "(", Regex: true})
	assert.ErrorIs(t, err, ErrBadPattern)

	empty, err := SearchFolder(context.Background(), root, "", SearchQuery{Text: "  "})
	require.NoError(t, err)
	assert.Zero(t, empty.Total)

	only, err := SearchFolder(context.Background(), root, filepath.Join(root, "refs.bib"), SearchQuery{Text: "movilidad"})
	require.NoError(t, err)
	assert.Equal(t, 2, only.Total, "a loose file: only that file")
}

func TestSearchMatchPositionsAndPreview(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("palabra ", 30) + "acción final"
	writeTree(t, root, map[string]string{"a.tex": "uno\n  dos acción\n" + long + "\n"})
	res, err := SearchFolder(context.Background(), root, "", SearchQuery{Text: "acción"})
	require.NoError(t, err)
	require.Len(t, res.Files, 1)
	m := res.Files[0].Matches
	require.Len(t, m, 2)
	assert.Equal(t, 2, m[0].Line)
	assert.Equal(t, 7, m[0].Column, "columns count characters, from 1")
	assert.Equal(t, "dos acción", m[0].Preview, "leading spaces trimmed")
	assert.Equal(t, "acción", string([]rune(m[0].Preview)[m[0].Start:m[0].End]))
	assert.True(t, strings.HasPrefix(m[1].Preview, "…"), "a long line is trimmed around the match")
	assert.Equal(t, "acción", string([]rune(m[1].Preview)[m[1].Start:m[1].End]))
}
