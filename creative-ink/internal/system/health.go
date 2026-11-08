package system

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// Hardware tier labels shown to the user.
const (
	TierBasic    = "Básico (Solo API Nube)"
	TierAdvanced = "Avanzado (IA Local Soportada)"
	TierUnknown  = "Desconocido (no se pudo leer la RAM)"
)

// Stable tier identifiers for program logic (the frontend picks colours
// from these instead of parsing the human-readable labels).
const (
	TierIDBasic    = "basic"
	TierIDAdvanced = "advanced"
	TierIDUnknown  = "unknown"
)

// localAIMinRAMGB is the RAM threshold to run local models (e.g. Ollama).
const localAIMinRAMGB = 8.0

// diskRoot is the filesystem whose free space is reported.
const diskRoot = "/"

const (
	gib = 1 << 30 // RAM is reported in binary GB (what the OS and vendors show).
	gb  = 1e9     // Disk is reported in decimal GB (what Finder/Nautilus show).
)

// HealthStatus reports the LaTeX toolchain and hardware profile of the host.
// The json tags define the property names the frontend receives
// (Wails serialises bound return values through encoding/json).
//
// Probes are independent: if one fails, its field keeps the zero value and a
// human-readable message is appended to Warnings, while the rest still report.
type HealthStatus struct {
	// LaTeX toolchain
	HasXeLaTeX bool `json:"hasXeLaTeX"`
	HasLatexmk bool `json:"hasLatexmk"`
	HasBiber   bool `json:"hasBiber"`

	// Hardware profile
	TotalRAMGB      float64 `json:"totalRAMGB"`
	CPUCores        int     `json:"cpuCores"`
	FreeDiskSpaceGB float64 `json:"freeDiskSpaceGB"`
	HardwareTier    string  `json:"hardwareTier"`
	HardwareTierID  string  `json:"hardwareTierId"`

	// Operating system and proposed location of the central repository.
	Repository RepositoryOptions `json:"repository"`

	// Non-fatal probe failures. Always non-nil so JS receives [] and not null.
	Warnings []string `json:"warnings"`
}

// probes groups the system calls so tests can replace them.
type probes struct {
	lookup     func(name string) bool
	numCPU     func() int
	totalRAM   func() (uint64, error)
	freeDiskOn func(path string) (uint64, error)
	repository func() RepositoryOptions
}

var hostProbes = probes{
	lookup: binaryExists,
	numCPU: runtime.NumCPU,
	totalRAM: func() (uint64, error) {
		v, err := mem.VirtualMemory()
		if err != nil {
			return 0, err
		}
		return v.Total, nil
	},
	freeDiskOn: func(path string) (uint64, error) {
		u, err := disk.Usage(path)
		if err != nil {
			return 0, err
		}
		return u.Free, nil
	},
	repository: DetectRepositoryOptions,
}

// CheckHealth audits the LaTeX toolchain and the hardware of the host system.
func CheckHealth() HealthStatus {
	return checkHealth(hostProbes)
}

// checkHealth is the testable core.
func checkHealth(p probes) HealthStatus {
	s := HealthStatus{
		HasXeLaTeX: p.lookup("xelatex"),
		HasLatexmk: p.lookup("latexmk"),
		HasBiber:   p.lookup("biber"),
		CPUCores:   p.numCPU(),
		Repository: p.repository(),
		Warnings:   []string{},
	}

	ramOK := false
	if total, err := p.totalRAM(); err != nil {
		s.Warnings = append(s.Warnings, fmt.Sprintf("No se pudo leer la RAM: %v", err))
	} else {
		s.TotalRAMGB = round1(float64(total) / gib)
		ramOK = true
	}

	if free, err := p.freeDiskOn(diskRoot); err != nil {
		s.Warnings = append(s.Warnings, fmt.Sprintf("No se pudo leer el disco %q: %v", diskRoot, err))
	} else {
		s.FreeDiskSpaceGB = round1(float64(free) / gb)
	}

	s.HardwareTierID, s.HardwareTier = classifyTier(s.TotalRAMGB, ramOK)
	return s
}

// classifyTier decides whether the machine can run local AI models.
// Without a RAM reading it refuses to guess instead of defaulting to "basic".
func classifyTier(ramGB float64, ramOK bool) (id, label string) {
	switch {
	case !ramOK:
		return TierIDUnknown, TierUnknown
	case ramGB < localAIMinRAMGB:
		return TierIDBasic, TierBasic
	default:
		return TierIDAdvanced, TierAdvanced
	}
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// fallbackDirs are well-known TeX locations searched when a binary is not
// on PATH. GUI apps on macOS launched from Finder/Dock do not inherit the
// shell PATH, so /Library/TeX/texbin would otherwise be missed.
var fallbackDirs = []string{
	"/Library/TeX/texbin", // MacTeX / BasicTeX
	"/opt/homebrew/bin",   // Homebrew (Apple Silicon)
	"/usr/local/bin",      // Homebrew (Intel) / manual installs
	"/usr/bin",            // Linux distro packages
}

// fallbackGlobs cover vanilla TeX Live installs (e.g. /usr/local/texlive/2026/bin/x86_64-linux).
var fallbackGlobs = []string{
	"/usr/local/texlive/*/bin/*",
}

// FindBinary returns the absolute path of an executable, looking it up on
// PATH first and then in well-known TeX/Homebrew dirs (see fallbackDirs).
func FindBinary(name string) (string, bool) {
	if path, err := exec.LookPath(name); err == nil {
		return path, true
	}
	for _, dir := range candidateDirs() {
		path := filepath.Join(dir, name)
		if isExecutable(path) {
			return path, true
		}
	}
	return "", false
}

func binaryExists(name string) bool {
	_, ok := FindBinary(name)
	return ok
}

// ToolPATH returns the process PATH extended with the TeX and Homebrew
// directories it lacks (appended, so the user's own order wins). Apps
// launched from the macOS Finder get a minimal PATH; commands run from the
// integrated terminal (tlmgr, git from Homebrew) need these.
func ToolPATH() string {
	current := os.Getenv("PATH")
	parts := filepath.SplitList(current)
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		seen[p] = true
	}
	for _, dir := range candidateDirs() {
		if !seen[dir] {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				parts = append(parts, dir)
				seen[dir] = true
			}
		}
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func candidateDirs() []string {
	dirs := append([]string{}, fallbackDirs...)
	for _, pattern := range fallbackGlobs {
		if matches, err := filepath.Glob(pattern); err == nil {
			dirs = append(dirs, matches...)
		}
	}
	return dirs
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}
