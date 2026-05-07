package main

import (
	"path/filepath"
	"strings"
)

// noStrayChanges checks that every dirty path is allowed by the per-flow rule:
//   - text writer: changes only inside b/ AND outside b/patches/
//   - procedural writer: only b/patches/<name> exists, nothing else
//   - procedural script-run: changes inside b/, plus the script file itself
func noStrayChanges(paths []string, allow func(string) bool) *Exception {
	for _, p := range paths {
		if !allow(p) {
			return Fmt("stray change at %q", p)
		}
	}

	return nil
}

func allowTextWriter(p string) bool {
	if !strings.HasPrefix(p, "b/") {
		return false
	}

	if strings.HasPrefix(p, "b/patches/") {
		return false
	}

	return true
}

func allowProcWriter(name string) func(string) bool {
	target := filepath.Join("b/patches", name)

	return func(p string) bool {
		return p == target
	}
}

func allowProcScriptRun(name string) func(string) bool {
	target := filepath.Join("b/patches", name)

	return func(p string) bool {
		if p == target {
			return true
		}

		if !strings.HasPrefix(p, "b/") {
			return false
		}

		if strings.HasPrefix(p, "b/patches/") && p != target {
			return false
		}

		return true
	}
}

// scriptRunner picks an interpreter by extension. Falls back to /bin/sh.
func scriptRunner(scriptPath string) []string {
	lower := strings.ToLower(scriptPath)

	switch {
	case strings.HasSuffix(lower, ".py"):
		return []string{"python3"}
	case strings.HasSuffix(lower, ".sh"):
		return []string{"sh"}
	case strings.HasSuffix(lower, ".bash"):
		return []string{"bash"}
	}

	return []string{"sh"}
}

// parseObsolete looks for an `OBSOLETE: <reason>` line in Claude's
// final response. The marker can appear anywhere — Claude usually
// writes it as the last line of the message.
func parseObsolete(text string) (bool, string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "OBSOLETE:") {
			return true, strings.TrimSpace(strings.TrimPrefix(line, "OBSOLETE:"))
		}
	}

	return false, ""
}

// parseVerdict reads `VERDICT: YES|NO` off the first non-empty line of the
// validator's response. Returns (yes, reason).
func parseVerdict(text string) (bool, string) {
	lines := strings.Split(text, "\n")

	var first string

	for _, ln := range lines {
		ln = strings.TrimSpace(ln)

		if ln == "" {
			continue
		}

		first = ln

		break
	}

	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), first))

	switch {
	case strings.HasPrefix(strings.ToUpper(first), "VERDICT: YES"):
		return true, rest
	case strings.HasPrefix(strings.ToUpper(first), "VERDICT: NO"):
		return false, rest
	}

	return false, "no VERDICT line in: " + first
}
