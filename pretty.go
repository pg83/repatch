package main

import (
	"fmt"
	"os"
)

const (
	clrR    = "\x1b[91m"
	clrG    = "\x1b[92m"
	clrY    = "\x1b[93m"
	clrB    = "\x1b[94m"
	clrM    = "\x1b[95m"
	clrC    = "\x1b[96m"
	clrW    = "\x1b[97m"
	clrBold = "\x1b[1m"
	clrDim  = "\x1b[2m"
	clrRst  = "\x1b[0m"
)

func clr(code, text string) string {
	return code + text + clrRst
}

// say prints to stderr with a leading marker and indent. Indent is in spaces.
func say(indent int, marker, format string, args ...any) {
	pad := ""

	for i := 0; i < indent; i++ {
		pad += " "
	}

	fmt.Fprintln(os.Stderr, pad+marker+" "+fmt.Sprintf(format, args...))
}

func step(indent int, format string, args ...any) {
	say(indent, clr(clrC, "↳"), clr(clrDim, format), args...)
}

func ok(indent int, format string, args ...any) {
	say(indent, clr(clrG, "✓"), format, args...)
}

func bad(indent int, format string, args ...any) {
	say(indent, clr(clrR, "✗"), clr(clrR, format), args...)
}

func retry(indent int, format string, args ...any) {
	say(indent, clr(clrY, "↻"), clr(clrY, format), args...)
}

func note(indent int, format string, args ...any) {
	say(indent, clr(clrDim, "·"), clr(clrDim, format), args...)
}

func header(format string, args ...any) {
	fmt.Fprintln(os.Stderr, clr(clrBold+clrM, fmt.Sprintf(format, args...)))
}

// showPatchDiff prints a colored unified diff between two patch files.
// Used after Claude retargets a patch — operator can eyeball whether
// the rewrite makes sense. No truncation — long diffs scroll, that's the
// project rule.
func showPatchDiff(indent int, oldPath, newPath string) {
	out, _ := diffFiles(oldPath, newPath)

	if len(out) == 0 {
		return
	}

	say(indent, clr(clrM, "Δ"), clr(clrBold, "patch diff (orig → retargeted):"))

	pad := ""

	for i := 0; i < indent+2; i++ {
		pad += " "
	}

	for _, line := range splitKeepEmpty(out) {
		fmt.Fprintln(os.Stderr, pad+colorDiffLine(line))
	}
}

func colorDiffLine(line string) string {
	switch {
	case len(line) >= 3 && (line[:3] == "---" || line[:3] == "+++"):
		return clr(clrBold, line)
	case len(line) > 0 && line[0] == '+':
		return clr(clrG, line)
	case len(line) > 0 && line[0] == '-':
		return clr(clrR, line)
	case len(line) >= 2 && line[:2] == "@@":
		return clr(clrC, line)
	}

	return line
}

func splitKeepEmpty(s string) []string {
	out := []string{}
	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}

	if start < len(s) {
		out = append(out, s[start:])
	}

	return out
}

func banner(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	bar := ""

	for i := 0; i < len(line); i++ {
		bar += "═"
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, clr(clrBold+clrC, bar))
	fmt.Fprintln(os.Stderr, clr(clrBold+clrC, line))
	fmt.Fprintln(os.Stderr, clr(clrBold+clrC, bar))
}
