package system

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fakeProbes returns deterministic probes; override fields per test.
func fakeProbes() probes {
	return probes{
		lookup:     func(string) bool { return true },
		numCPU:     func() int { return 8 },
		totalRAM:   func() (uint64, error) { return 16 * gib, nil },
		freeDiskOn: func(string) (uint64, error) { return 250 * gb, nil },
	}
}

func TestCheckHealthMapsEachTool(t *testing.T) {
	installed := map[string]bool{"xelatex": true, "biber": true}
	p := fakeProbes()
	p.lookup = func(name string) bool { return installed[name] }

	got := checkHealth(p)

	if !got.HasXeLaTeX || got.HasLatexmk || !got.HasBiber {
		t.Fatalf("tools = xelatex:%v latexmk:%v biber:%v, want true/false/true",
			got.HasXeLaTeX, got.HasLatexmk, got.HasBiber)
	}
}

func TestCheckHealthHardwareProfile(t *testing.T) {
	got := checkHealth(fakeProbes())

	want := HealthStatus{
		HasXeLaTeX: true, HasLatexmk: true, HasBiber: true,
		TotalRAMGB: 16, CPUCores: 8, FreeDiskSpaceGB: 250,
		HardwareTier: TierAdvanced, HardwareTierID: TierIDAdvanced,
		Warnings: []string{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("checkHealth() = %+v\nwant %+v", got, want)
	}
}

func TestClassifyTier(t *testing.T) {
	cases := []struct {
		ram    float64
		ramOK  bool
		wantID string
	}{
		{4, true, TierIDBasic},
		{7.9, true, TierIDBasic},
		{8, true, TierIDAdvanced},
		{48, true, TierIDAdvanced},
		{0, false, TierIDUnknown},
	}
	for _, c := range cases {
		if id, _ := classifyTier(c.ram, c.ramOK); id != c.wantID {
			t.Errorf("classifyTier(%v, %v) = %q, want %q", c.ram, c.ramOK, id, c.wantID)
		}
	}
}

func TestCheckHealthDegradesGracefully(t *testing.T) {
	p := fakeProbes()
	p.totalRAM = func() (uint64, error) { return 0, errors.New("sysctl failed") }
	p.freeDiskOn = func(string) (uint64, error) { return 0, errors.New("statfs failed") }

	got := checkHealth(p)

	if got.TotalRAMGB != 0 || got.FreeDiskSpaceGB != 0 {
		t.Errorf("failed probes must report 0, got RAM=%v disk=%v", got.TotalRAMGB, got.FreeDiskSpaceGB)
	}
	if got.HardwareTierID != TierIDUnknown {
		t.Errorf("tier = %q, want %q when RAM is unreadable", got.HardwareTierID, TierIDUnknown)
	}
	if len(got.Warnings) != 2 {
		t.Errorf("warnings = %v, want 2 entries", got.Warnings)
	}
	if !got.HasXeLaTeX || got.CPUCores != 8 {
		t.Error("independent probes must still report when others fail")
	}
}

func TestIsExecutable(t *testing.T) {
	dir := t.TempDir()

	exe := filepath.Join(dir, "tool")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := map[string]bool{
		exe:                          true,
		plain:                        false,
		dir:                          false,
		filepath.Join(dir, "absent"): false,
	}
	for path, want := range cases {
		if got := isExecutable(path); got != want {
			t.Errorf("isExecutable(%q) = %v, want %v", path, got, want)
		}
	}
}
