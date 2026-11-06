---
name: latex-toolchain-orchestrator
description: Orchestrate LaTeX compilation via latexmk in Go for Creative-ink. Handles TeX binary resolution on macOS/Linux (Finder-launched apps lack the shell PATH), secure process execution without shells, timeouts, engine selection (xelatex/pdflatex/lualatex), and parsing TeX output or .log files into concise errors. Use when building or changing the compilation pipeline, resolving latexmk/xelatex/biber paths, or extracting LaTeX errors.
---

# LaTeX Toolchain Orchestrator (Creative-ink)

You are an expert in integrating the LaTeX ecosystem (TeX Live, MacTeX, latexmk) into Go backends.

Read first: `CLAUDE.md` (rules, status) and `docs/MAP.md` §4 and §4.3 (current pipeline).
Those files win over this skill if they disagree.

## Existing code (reuse it, do not duplicate)

- `creative-ink/internal/compiler/orchestrator.go`: `CompileLaTeXContext(ctx, workdir, filename)` (and
  `CompileLaTeX` without cancellation), `PDFPath`, `summarize(output, texLog)`, `readTeXLog`,
  `teXErrorBlocks` / `extractTeXErrors`, `validateFilename`, global `compileMu`.
- `creative-ink/internal/compiler/scheduler.go`: background `Scheduler` that calls the compiler
  (one run at a time, coalesced). Never call `CompileLaTeX` synchronously from a binding.
- `creative-ink/internal/system/health.go`: `FindBinary(name) (path, ok)` with `fallbackDirs`
  and `fallbackGlobs`. Every binary lookup goes through it.
- Tests: `internal/compiler/orchestrator_test.go` (unit) plus an opt-in integration test:
  `CREATIVE_INK_LATEX_TEST=1 go test ./internal/compiler -run Integration -v` (the user runs it).

## When to use

- Building or changing the compilation pipeline for `.tex` files.
- Resolving system paths for TeX binaries (latexmk, xelatex, pdflatex, lualatex, biber, kpsewhich).
- Parsing and extracting errors from LaTeX `.log` files or latexmk output.

## Principles & execution rules

1. **Strict binary resolution.** Apps opened from the macOS Finder/Dock do NOT inherit the terminal
   `$PATH`. Always use `system.FindBinary`: `exec.LookPath` first, then `/Library/TeX/texbin`,
   `/opt/homebrew/bin`, `/usr/local/bin`, `/usr/bin` and `/usr/local/texlive/*/bin/*`.
   If a binary is missing, return a clear sentinel error (e.g. `ErrLatexmkNotFound`), never a raw
   `exec: "latexmk": executable file not found`.
2. **Secure execution.** Run latexmk directly with `exec.CommandContext(ctx, absPath, args...)`.
   NEVER use `sh -c` / `bash -c` and never build a command line by concatenating strings.
   Validate file names: bare names only (no directories) and no leading `-`, which latexmk would read as an option.
3. **Flags.** Base command:
   `latexmk -<engine> -interaction=nonstopmode -halt-on-error <filename>`
   - The engine is **selectable**: `-xelatex` (default), `-pdf` (pdflatex) or `-lualatex`. Map it from
     an enum/whitelist, never pass user text through as a flag.
   - Do not add or remove flags (e.g. `-synctex=1`, `-file-line-error`, `-outdir`, `-shell-escape`)
     without the user's decision. `-shell-escape` is a security risk: never enable it by default.
4. **Smart log parsing.** TeX output is noisy. On failure, extract the blocks that start with `! `
   (the error) up to the `l.<line>` line (source reference). The concise error goes **first**. A
   truncated tail of the raw output (≤ 32 KB) may follow for diagnosis. Never dump the full output.
   - stdout is hard-wrapped at ~79 columns by TeX (`max_print_line`); `<name>.log` in `workdir` has
     the same wrapping but is complete. Prefer the `.log` when the stdout tail was truncated.
   - Missing `l.<n>` (e.g. `! LaTeX Error: File 'x.sty' not found.` or fatal errors): keep the
     `! ` line plus a few context lines (`errorContextLines`).
5. **Results, not exceptions.** A compilation failure is a normal outcome: return logs + error and
   let the facade turn it into a result object. See the `go-wails-ipc-bridge` skill.

## Gotchas

- **Hangs and loops.** Always use `context.WithTimeout` (60 s today: `compileTimeout`) and set
  `cmd.WaitDelay` so orphaned children (xelatex, biber) cannot keep the pipes open forever.
- **PATH prepend.** latexmk finds xelatex/biber by name: prepend `filepath.Dir(latexmkPath)` to the
  `PATH` in `cmd.Env`. Go uses the last value of a duplicated key, so appending `PATH=` works.
- **Serialise runs.** Two latexmk runs in the same directory corrupt `.aux`/`.fdb_latexmk` files.
  Keep `compileMu` (or a per-workdir lock if multiple documents are supported later).
- **Combined output.** Point `Stdout` and `Stderr` at the same buffer to keep their interleaving.
- **Interaction.** `-interaction=nonstopmode` prevents TeX from waiting on stdin. Never attach a
  terminal or a pipe on stdin.
- **Auxiliary files** (`.aux`, `.log`, `.fls`, `.fdb_latexmk`, `.xdv`, `.pdf`) live next to the
  document in `workspace/` (git-ignored). Do not delete them between runs: latexmk needs them to
  stay incremental.
- **Project rules.** You must not run `latexmk`, `xelatex`, `pdflatex`, `lualatex` or `tlmgr`
  yourself. Use unit tests with sample output (see `sampleFailure` in the tests) and give the user
  the exact command to run the integration test.

## Verification

From `creative-ink/`: `gofmt -l .` (must print nothing), `go vet ./...`, `go test ./internal/compiler/...`.
Then tell the user to restart `wails dev` and describe the expected success and error behaviour.
