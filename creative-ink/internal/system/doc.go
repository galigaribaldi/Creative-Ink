// Package system inspects the host environment: LaTeX health check,
// detection of installed engines (latexmk, pdflatex, xelatex, lualatex)
// and guided installation of missing ones.
//
// It must not import Wails; it receives callbacks/interfaces from App.
package system
