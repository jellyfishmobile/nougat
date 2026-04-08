package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	bolt "go.etcd.io/bbolt"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiWhite  = "\033[97m"
)

const nougatBannerArt = `    ███╗   ██╗ ██████╗ ██╗   ██╗ ██████╗  █████╗ ████████╗
    ████╗  ██║██╔═══██╗██║   ██║██╔════╝ ██╔══██╗╚══██╔══╝
    ██╔██╗ ██║██║   ██║██║   ██║██║  ███╗███████║   ██║
    ██║╚██╗██║██║   ██║██║   ██║██║   ██║██╔══██║   ██║
    ██║ ╚████║╚██████╔╝╚██████╔╝╚██████╔╝██║  ██║   ██║
    ╚═╝  ╚═══╝ ╚═════╝  ╚═════╝  ╚═════╝ ╚═╝  ╚═╝   ╚═╝`

// printBanner writes the NOUGAT ASCII art (no tagline).
func printBanner(w io.Writer) {
	color := ansiOK(w)
	banner := nougatBannerArt
	if color {
		lines := strings.Split(banner, "\n")
		for i := range lines {
			lines[i] = ansiCyan + ansiBold + lines[i] + ansiReset
		}
		banner = strings.Join(lines, "\n")
	}
	fmt.Fprintf(w, "\n%s\n", banner)
}

// printWelcome writes banner, build line, and tagline (server startup or help header).
func printWelcome(w io.Writer) {
	color := ansiOK(w)
	// Plain first line so something always appears when ANSI is stripped or broken.
	fmt.Fprintf(w, "nougat %s\n", versionLine())
	printBanner(w)
	tagline := "Claude task runner · HTTP API"
	if color {
		tagline = ansiDim + "Claude task runner" + ansiReset + " · " + ansiGray + "HTTP API" + ansiReset
	}
	fmt.Fprintf(w, "  %s\n\n", tagline)
}

func printServerStatus(w io.Writer, db *bolt.DB, addr, dbPath, logDir, configPath string) {
	color := ansiOK(w)
	title := "Status"
	if color {
		title = ansiBold + ansiWhite + "Status" + ansiReset
	}
	fmt.Fprintf(w, "  %s\n", title)
	addrDisp := addr
	if color {
		addrDisp = ansiGreen + addr + ansiReset
	}
	fmt.Fprintf(w, "  %s Listen     %s\n", dimLabel(color, "│"), addrDisp)
	fmt.Fprintf(w, "  %s Database   %s\n", dimLabel(color, "│"), dbPath)
	fmt.Fprintf(w, "  %s Job logs   %s\n", dimLabel(color, "│"), logDir)
	if configPath != "" {
		fmt.Fprintf(w, "  %s Config     %s\n", dimLabel(color, "│"), configPath)
	}
	acc := accessLogFile(logDir)
	if acc != "" {
		fmt.Fprintf(w, "  %s Access log %s\n", dimLabel(color, "│"), acc)
	}

	total, byStatus, err := jobStats(db)
	if err != nil {
		errMsg := "(error reading stats)"
		if color {
			errMsg = ansiYellow + errMsg + ansiReset
		}
		fmt.Fprintf(w, "  %s Jobs       %s\n", dimLabel(color, "│"), errMsg)
	} else {
		fmt.Fprintf(w, "  %s Jobs       %d total", dimLabel(color, "│"), total)
		order := []JobStatus{StatusPending, StatusRunning, StatusCompleted, StatusFailed, StatusTimeout}
		parts := []string{}
		for _, st := range order {
			if n := byStatus[st]; n > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", st, n))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(w, " (%s)", strings.Join(parts, " "))
		}
		fmt.Fprintln(w)
	}

	ep := "Endpoints"
	if color {
		ep = ansiBold + ansiWhite + "Endpoints" + ansiReset
	}
	fmt.Fprintf(w, "\n  %s\n", ep)
	fmt.Fprintf(w, "  %s POST  /run-task           native async task\n", dimLabel(color, "│"))
	fmt.Fprintf(w, "  %s GET   /jobs/{id}          job status & output\n", dimLabel(color, "│"))
	fmt.Fprintf(w, "  %s POST  /v1/chat/completions OpenAI-compatible chat\n", dimLabel(color, "│"))
	fmt.Fprintf(w, "  %s GET   /v1/models           OpenAI model list\n", dimLabel(color, "│"))
	fmt.Fprintf(w, "  %s GET   /dashboard           usage UI (Bearer key)\n", dimLabel(color, "│"))
	fmt.Fprintf(w, "  %s GET   /health              liveness\n", dimLabel(color, "│"))

	fmt.Fprintln(w)
	if color {
		fmt.Fprintf(w, "  %sHTTP access log%s → stdout (below when attached to a TTY)\n\n", ansiDim, ansiReset)
	} else {
		fmt.Fprintln(w, "  HTTP access log → stdout")
		fmt.Fprintln(w)
	}
	flushWriter(w)
}

// printServerRunningHint is a short reminder after the status block.
func printServerRunningHint(listenAddr string) {
	base := listenAddrToBaseURL(listenAddr)
	fmt.Fprintf(os.Stderr, "nougat: serving HTTP on %s — Ctrl+C to stop\n", listenAddr)
	fmt.Fprintf(os.Stderr, "        %s/health\n", base)
	_ = os.Stderr.Sync()
}

func listenAddrToBaseURL(listenAddr string) string {
	a := strings.TrimSpace(listenAddr)
	if a == "" {
		return "http://127.0.0.1:8080"
	}
	if len(a) > 0 && a[0] == ':' {
		return "http://127.0.0.1" + a
	}
	return "http://" + a
}

func flushWriter(w io.Writer) {
	if f, ok := w.(*os.File); ok {
		_ = f.Sync()
	}
}

func dimLabel(color bool, s string) string {
	if color {
		return ansiGray + s + ansiReset
	}
	return s
}
