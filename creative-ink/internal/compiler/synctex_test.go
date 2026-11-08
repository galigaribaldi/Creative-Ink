package compiler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Real `synctex view` output (SyncTeX 1.5), with two matching boxes.
const sampleView = `This is SyncTeX command line utility, version 1.5
SyncTeX result begin
Output:/w/main.pdf
Page:2
x:133.768356
y:134.765778
h:133.768356
v:137.255585
W:343.711060
H:9.962640
before:
offset:0
middle:
after:
Output:/w/main.pdf
Page:2
x:200.0
y:300.0
h:200.0
v:300.0
W:10.0
H:10.0
before:
offset:0
middle:
after:
SyncTeX result end
`

const sampleEdit = `This is SyncTeX command line utility, version 1.5
SyncTeX result begin
Output:/w/main.pdf
Input:./chapters/intro.tex
Line:12
Column:-1
Offset:0
Context:
SyncTeX result end
`

func TestParseSyncTeXViewTakesTheFirstBox(t *testing.T) {
	box, ok := parseSyncTeXView(sampleView)
	if !ok {
		t.Fatal("expected a box")
	}
	if box.Page != 2 || box.X != 133.768356 || box.Width != 343.711060 || box.Height != 9.962640 {
		t.Fatalf("box = %+v", box)
	}
	// Top = baseline (v) - height (H).
	if want := 137.255585 - 9.962640; box.Y < want-1e-9 || box.Y > want+1e-9 {
		t.Fatalf("Y = %v, want %v", box.Y, want)
	}
}

func TestParseSyncTeXEdit(t *testing.T) {
	src, ok := parseSyncTeXEdit(sampleEdit)
	if !ok || src.File != "./chapters/intro.tex" || src.Line != 12 {
		t.Fatalf("src = %+v, ok = %v", src, ok)
	}
}

func TestParseSyncTeXNoResult(t *testing.T) {
	for _, out := range []string{
		"",
		"SyncTeX ERROR: No file?\n",
		"SyncTeX result begin\nSyncTeX result end\n",
		"SyncTeX result begin\nOutput:x.pdf\nPage:0\nSyncTeX result end\n",
	} {
		if _, ok := parseSyncTeXView(out); ok {
			t.Errorf("view %q must not be found", out)
		}
		if _, ok := parseSyncTeXEdit(out); ok {
			t.Errorf("edit %q must not be found", out)
		}
	}
}

func TestSyncTeXWithoutDataIsSilent(t *testing.T) {
	pdf := filepath.Join(t.TempDir(), "main.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No main.synctex.gz: not found, no error, synctex is not even run.
	if _, ok := SyncTeXForward(context.Background(), pdf, "main.tex", 3); ok {
		t.Error("forward without .synctex.gz must not be found")
	}
	if _, ok := SyncTeXBackward(context.Background(), pdf, 1, 100, 100); ok {
		t.Error("backward without .synctex.gz must not be found")
	}
	if _, ok := SyncTeXForward(context.Background(), pdf, "main.tex", 0); ok {
		t.Error("line 0 is invalid")
	}
	if _, ok := SyncTeXBackward(context.Background(), pdf, 0, 1, 1); ok {
		t.Error("page 0 is invalid")
	}
}

func TestSyncTeXDataPath(t *testing.T) {
	if got := SyncTeXDataPath("/w/main.pdf"); got != "/w/main.synctex.gz" {
		t.Fatalf("SyncTeXDataPath = %q", got)
	}
}
