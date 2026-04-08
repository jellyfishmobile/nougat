package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// printCLIHelp prints full usage with ANSI sections; suitable for -h / -help / `nougat help`.
func printCLIHelp() {
	printBanner()
	title := "Command help"
	if supportsANSI() {
		title = ansiBold + ansiWhite + title + ansiReset
	}
	fmt.Fprintf(os.Stdout, "  %s\n\n", title)

	fmt.Fprintln(os.Stdout, helpSection("SYNOPSIS"))
	fmt.Fprintln(os.Stdout, strings.TrimSpace(helpIndent(`
nougat starts an HTTP API that queues tasks and runs your configured LLM CLI
for each job. Stdout/stderr and exit status are stored per job.
`)))

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, helpSection("OPTIONS"))
	flag.CommandLine.SetOutput(os.Stdout)
	flag.PrintDefaults()
	fmt.Fprintln(os.Stdout, notFlag(`  Environment: PORT may set the listen address (e.g. 8080 → ":8080") when -addr is omitted.`))

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, helpSection("HTTP API"))
	fmt.Fprintln(os.Stdout, helpIndent(`
POST /run-task       JSON body: {"prompt":"...","working_directory":"...","task_type":"...","timeout":300}
GET  /jobs/{id}      Job status, stdout, stderr
GET  /health         JSON {"status":"healthy"}
`))

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, helpSection("LLM BACKEND (Claude vs Gemini)"))
	fmt.Fprintln(os.Stdout, helpIndent(`
Nougat runs a subprocess per job. You choose the binary and the argument list that
comes before the final prompt string.

  `+helpCode("NOUGAT_LLM_BIN")+`   Executable name or path.
                      Default: `+helpCode("claude")+` (Anthropic Claude Code CLI)

  `+helpCode("NOUGAT_LLM_EXTRA")+`  Extra arguments placed before the prompt, split on spaces.
                      Default when unset: `+helpCode("--prompt")+` so the command is:
                      `+helpExample("claude --prompt \"<prompt>\"")+`

                      If set to an empty value, no extra args are used; the prompt is the
                      only argument after the binary:
                      `+helpExample("<bin> \"<prompt>\"")+`

                      If set to non-empty text, it is split with `+helpCode("strings.Fields")+` (no quotes);
                      use a wrapper script if you need complex quoting.
`))

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, helpSection("Using Claude (Anthropic)"))
	fmt.Fprintln(os.Stdout, helpIndent(`
1. Install the Claude Code CLI and sign in so `+helpCode("claude")+` works in your shell.
2. Confirm: `+helpExample("claude --help")+` (or your install’s equivalent).
3. Run nougat with defaults — no env vars needed. Each task runs:
   `+helpExample("claude --prompt \"<job prompt>\"")+`
4. If the binary is not named `+helpCode("claude")+`, set:
   `+helpExample("export NOUGAT_LLM_BIN=/path/to/claude")+`
`))

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, helpSection("Using Gemini (Google)"))
	fmt.Fprintln(os.Stdout, helpIndent(`
1. Install whatever Gemini CLI you use (`+helpCode("gemini")+`, npm package, etc.) and ensure
   it runs non-interactively for a one-shot prompt (check that tool’s docs).
2. Discover the exact invocation: some CLIs use `+helpCode("-p")+`, `+helpCode("prompt")+`, or a subcommand.
3. Point nougat at that binary and arguments, for example if your CLI is `+helpCode("gemini -p \"text\"")+`:

   `+helpExample("export NOUGAT_LLM_BIN=gemini")+`
   `+helpExample("export NOUGAT_LLM_EXTRA=-p")+`

   That runs: `+helpExample("gemini -p \"<job prompt>\"")+`

4. If the prompt must be a bare positional with no flag, clear the extras:

   `+helpExample("export NOUGAT_LLM_BIN=gemini")+`
   `+helpExample("export NOUGAT_LLM_EXTRA=")+`

   so nougat runs: `+helpExample("gemini \"<job prompt>\"")+`

5. For subcommands or flags with spaces/quotes, use a small shell script as `+helpCode("NOUGAT_LLM_BIN")+`
   that invokes the real CLI with the right argument shape.
`))

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, helpSection("SEE ALSO"))
	fmt.Fprintln(os.Stdout, helpIndent(`Run without arguments to start the server (see banner + status on stdout).`))
	fmt.Fprintln(os.Stdout)
}

func helpSection(title string) string {
	if !supportsANSI() {
		return title
	}
	return ansiCyan + ansiBold + title + ansiReset
}

func helpCode(s string) string {
	if !supportsANSI() {
		return s
	}
	return ansiGreen + s + ansiReset
}

func helpExample(s string) string {
	if !supportsANSI() {
		return s
	}
	return ansiDim + s + ansiReset
}

func helpIndent(parts ...string) string {
	s := strings.TrimSpace(strings.Join(parts, ""))
	return s
}

func notFlag(line string) string {
	if !supportsANSI() {
		return line
	}
	return ansiDim + line + ansiReset
}
