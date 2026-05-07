package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Driver struct {
	cfg                 *Config
	sessionStarted      bool
	patchesSinceCompact int
}

type RunResult struct {
	Events    []map[string]any
	FinalText string
	ExitCode  int
	Elapsed   time.Duration
}

func newDriver(cfg *Config) *Driver {
	return &Driver{cfg: cfg}
}

// Run executes one Claude turn with the given prompt. cwd controls where Claude
// reads/writes (typically ws.Root). Continues a prior session iff the policy
// (CompactEvery != 1) allows it.
func (d *Driver) Run(prompt, cwd string) *RunResult {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	}

	if d.shouldContinue() {
		args = append([]string{"--continue"}, args...)
	}

	cmd := exec.Command(d.cfg.ClaudeBin, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	stdout := Throw2(cmd.StdoutPipe())
	cmd.Stderr = os.Stderr

	started := time.Now()

	if err := cmd.Start(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ThrowFatal("claude binary %q not found: %v", d.cfg.ClaudeBin, err)
		}

		ThrowFmt("start claude: %v", err)
	}

	res := &RunResult{}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)

	for scanner.Scan() {
		var ev map[string]any

		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}

		res.Events = append(res.Events, ev)
		liveTrace(ev)

		if t, _ := ev["type"].(string); t == "result" {
			if txt, _ := ev["result"].(string); txt != "" {
				res.FinalText = txt
			}
		}
	}

	err := cmd.Wait()

	if ee, ok := err.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
	} else if err != nil {
		ThrowFmt("claude wait: %v", err)
	}

	res.Elapsed = time.Since(started)

	d.sessionStarted = true
	d.patchesSinceCompact++

	return res
}

// liveTrace prints one line per assistant tool_use as it streams from CC,
// so the user sees what Claude is doing instead of staring at silence.
// We pluck the most relevant arg per tool (file_path, command, pattern).
func liveTrace(ev map[string]any) {
	typ, _ := ev["type"].(string)

	if typ != "assistant" {
		return
	}

	msg, _ := ev["message"].(map[string]any)

	if msg == nil {
		return
	}

	content, _ := msg["content"].([]any)

	for _, c := range content {
		block, _ := c.(map[string]any)

		if block == nil {
			continue
		}

		btyp, _ := block["type"].(string)

		if btyp != "tool_use" {
			continue
		}

		name, _ := block["name"].(string)
		input, _ := block["input"].(map[string]any)
		say(6, clr(clrM, "·"), clr(clrDim, "%s %s"), clr(clrC, name), summarizeInput(name, input))
	}
}

func summarizeInput(toolName string, input map[string]any) string {
	if input == nil {
		return ""
	}

	switch toolName {
	case "Read", "Edit", "Write", "NotebookEdit":
		if p, ok := input["file_path"].(string); ok {
			return p
		}
	case "Bash":
		if c, ok := input["command"].(string); ok {
			return truncate(c, 100)
		}
	case "Grep", "Glob":
		if p, ok := input["pattern"].(string); ok {
			return p
		}
	}

	return ""
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")

	if len(s) > n {
		return s[:n] + "…"
	}

	return s
}

func (d *Driver) shouldContinue() bool {
	if d.cfg.CompactEvery == 1 {
		return false
	}

	return d.sessionStarted
}

// MaybeCompact issues `/compact` to the current --continue session every
// CompactEvery patches. No-op for CompactEvery <= 1 (handled elsewhere) or
// before the first patch.
func (d *Driver) MaybeCompact() {
	if d.cfg.CompactEvery <= 1 || !d.sessionStarted {
		return
	}

	if d.patchesSinceCompact < d.cfg.CompactEvery {
		return
	}

	cmd := exec.Command(d.cfg.ClaudeBin, "--continue", "-p", "/compact")
	cmd.Stderr = os.Stderr

	if out, err := cmd.Output(); err != nil {
		fmt.Fprintf(os.Stderr, "repatch: /compact failed (continuing without): %v: %s\n", err, out)
	}

	d.patchesSinceCompact = 0
}
