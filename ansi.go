package main

import (
	"io"
	"os"

	"golang.org/x/term"
)

// ansiOK is true when we should emit ANSI escapes to w (interactive terminal, etc.).
func ansiOK(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if t := os.Getenv("TERM"); t == "" || t == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// supportsANSI is used for stdout (help text, access log lines).
func supportsANSI() bool {
	return ansiOK(os.Stdout)
}
