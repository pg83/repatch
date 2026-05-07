package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	Try(func() {
		run()
	}).Catch(func(e *Exception) {
		bad(0, "abort: %s", e.Error())
		os.Exit(1)
	})
}

func run() {
	cfg := parseConfig()

	header("repatch")
	note(0, "workdir = %s", cfg.Workdir)
	note(0, "src-old = %s", cfg.SrcOld)
	note(0, "src-new = %s", cfg.SrcNew)
	note(0, "out     = %s", cfg.OutDir)

	step(0, "setting up workspace")
	ws := setupWorkspace(cfg)
	ok(0, "workspace ready (a/, b/, git initialized)")

	if !cfg.KeepWorkdir {
		defer cleanupWorkspace(ws)
	}

	patches := loadPatches(cfg)
	ok(0, "loaded %d patches", len(patches))

	Throw(os.MkdirAll(cfg.OutDir, 0755))

	drv := newDriver(cfg)

	for i, p := range patches {
		header("[%d/%d] · %s · %s", i+1, len(patches), clr(clrB, p.Type.String()), clr(clrW, p.Name))
		processPatch(cfg, ws, drv, p)

		if p.Status != StatusObsolete {
			syncOut(cfg, ws, p)
		}
	}

	writeReport(cfg, patches)
	printSummary(cfg, patches)
}

// processPatch lands p on b/. Strategy is uniform regardless of state:
//
//  1. Sticky obsolete marker → honor and return.
//  2. Existing artifact in --out → try; if applies cleanly, done.
//     Doesn't apply → drop from --out, fall through.
//  3. Original from a/patches/ → try as-is. Most patches that haven't
//     drifted will land here.
//  4. Otherwise → claude retargeter (with retries, fatal on exhaustion).
//
// On success p.Status is set; main loop syncs to --out (skip on obsolete).
func processPatch(cfg *Config, ws *Workspace, drv *Driver, p *Patch) {
	if obs, reason := loadObsoleteMarker(cfg, p); obs {
		p.Status = StatusObsolete
		p.Notes = reason
		say(2, clr(clrY, "⊘"), clr(clrY, "obsolete (marker present): %s"), reason)

		return
	}

	out := filepath.Join(cfg.OutDir, p.Name)

	if _, err := os.Stat(out); err == nil {
		step(2, "trying existing from --out")

		if tryApplyFile(ws, p, out) {
			ok(2, "existing applied cleanly")
			p.Status = StatusApplied

			return
		}

		bad(2, "existing rejected; deleting and rebuilding")
		invalidateOut(cfg, p)
	}

	if p.Type == PatchText {
		step(2, "trying original")

		if tryApplyFile(ws, p, p.Path) {
			ok(2, "original applied cleanly")
			p.Status = StatusApplied

			return
		}

		bad(2, "original rejected; handing off to claude")
	}

	drv.MaybeCompact()

	if p.Type == PatchText {
		retargetTextWithClaude(cfg, ws, drv, p)
	} else {
		retargetProceduralWithClaude(cfg, ws, drv, p)
	}
}

func retargetTextWithClaude(cfg *Config, ws *Workspace, drv *Driver, p *Patch) {
	var lastErr string

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		p.Attempt = attempt
		say(2, clr(clrM, "★"), clr(clrBold, "attempt %d/%d"), attempt, cfg.MaxRetries)

		exc := Try(func() {
			runTextWriter(cfg, ws, drv, p, lastErr)
		})

		if exc == nil {
			if p.Status == StatusObsolete {
				say(2, clr(clrY, "⊘"), clr(clrY, "marked obsolete: %s"), p.Notes)
			} else {
				ok(2, "committed retargeted patch")
			}

			return
		}

		if IsFatal(exc) {
			Rethrow(exc)
		}

		lastErr = exc.Error()
		bad(4, "%s", lastErr)

		Try(func() { revertB(ws) })

		if attempt < cfg.MaxRetries {
			retry(2, "retrying with feedback")
		}
	}

	ThrowFatal("text patch %s: out of retries: %s", p.Name, lastErr)
}

func runTextWriter(cfg *Config, ws *Workspace, drv *Driver, p *Patch, prevError string) {
	step(4, "writer: spawning claude")

	prompt := writerPromptText(p, ws, prevError)
	res := drv.Run(prompt, ws.Root)

	if res.ExitCode != 0 {
		ThrowFmt("claude exited %d", res.ExitCode)
	}

	ok(4, "writer returned in %s (%d events)", res.Elapsed.Truncate(time.Second), len(res.Events))

	if obs, reason := parseObsolete(res.FinalText); obs {
		dirty := dirtyPaths(ws)

		if len(dirty) > 0 {
			ThrowFmt("OBSOLETE declared but %d files dirty (claude must not touch anything)", len(dirty))
		}

		markObsolete(cfg, p, reason)
		p.Status = StatusObsolete
		p.Notes = reason

		return
	}

	dirty := dirtyPaths(ws)
	step(4, "validating: %d dirty paths under git", len(dirty))

	if exc := noStrayChanges(dirty, allowTextWriter); exc != nil {
		Throw(exc.AsError())
	}

	diff := diffB(ws)

	if len(strings.TrimSpace(string(diff))) == 0 {
		ThrowFmt("no source changes in b/")
	}

	final := append([]byte(extractPatchHeader(p.Path)), diff...)

	out := filepath.Join(ws.B, "patches", p.Name)
	Throw(os.WriteFile(out, final, 0644))

	ok(4, "wrote retargeted patch (%d bytes)", len(final))

	commitAll(ws, "retarget text: "+p.Name)

	showPatchDiff(4, p.Path, out)

	p.Status = StatusAppliedClaude
}

func retargetProceduralWithClaude(cfg *Config, ws *Workspace, drv *Driver, p *Patch) {
	var lastErr string

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		p.Attempt = attempt
		say(2, clr(clrM, "★"), clr(clrBold, "attempt %d/%d"), attempt, cfg.MaxRetries)

		exc := Try(func() {
			runProceduralCycle(cfg, ws, drv, p, lastErr)
		})

		if exc == nil {
			if p.Status == StatusObsolete {
				say(2, clr(clrY, "⊘"), clr(clrY, "marked obsolete: %s"), p.Notes)
			} else {
				ok(2, "committed retargeted procedural patch")
			}

			return
		}

		if IsFatal(exc) {
			Rethrow(exc)
		}

		lastErr = exc.Error()
		bad(4, "%s", lastErr)

		Try(func() { revertB(ws) })

		if attempt < cfg.MaxRetries {
			retry(2, "retrying with feedback")
		}
	}

	ThrowFatal("procedural patch %s: out of retries: %s", p.Name, lastErr)
}

func runProceduralCycle(cfg *Config, ws *Workspace, drv *Driver, p *Patch, prevError string) {
	step(4, "writer: spawning claude")

	prompt := writerPromptProcedural(p, ws, prevError)
	res := drv.Run(prompt, ws.Root)

	if res.ExitCode != 0 {
		ThrowFmt("writer claude exited %d", res.ExitCode)
	}

	ok(4, "writer returned in %s (%d events)", res.Elapsed.Truncate(time.Second), len(res.Events))

	if obs, reason := parseObsolete(res.FinalText); obs {
		dirty := dirtyPaths(ws)

		if len(dirty) > 0 {
			ThrowFmt("OBSOLETE declared but %d files dirty (claude must not touch anything)", len(dirty))
		}

		markObsolete(cfg, p, reason)
		p.Status = StatusObsolete
		p.Notes = reason

		return
	}

	scriptPath := filepath.Join(ws.B, "patches", p.Name)

	if _, err := os.Stat(scriptPath); err != nil {
		ThrowFmt("writer didn't produce %s", scriptPath)
	}

	dirty := dirtyPaths(ws)

	if exc := noStrayChanges(dirty, allowProcWriter(p.Name)); exc != nil {
		ThrowFmt("after writer: %v", exc.Error())
	}

	ok(4, "writer produced %s (clean)", filepath.Base(scriptPath))

	step(4, "running new script in b/")

	Throw(os.Chmod(scriptPath, 0755))

	runner := scriptRunner(scriptPath)

	cmd := exec.Command(runner[0], scriptPath)
	cmd.Dir = ws.B
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()

	if err != nil {
		ThrowFmt("script %s failed: %v: %s", p.Name, err, out)
	}

	dirty = dirtyPaths(ws)

	if exc := noStrayChanges(dirty, allowProcScriptRun(p.Name)); exc != nil {
		ThrowFmt("after script run: %v", exc.Error())
	}

	if len(diffB(ws)) == 0 {
		ThrowFmt("script ran but produced no changes in b/")
	}

	ok(4, "script ran, %d paths changed", len(dirty)-1)

	step(4, "validator: spawning claude")

	vRes := drv.Run(validatorPromptProcedural(p, ws), ws.Root)

	if vRes.ExitCode != 0 {
		ThrowFmt("validator claude exited %d", vRes.ExitCode)
	}

	verdict, reason := parseVerdict(vRes.FinalText)

	if !verdict {
		ThrowFmt("validator NO: %s", reason)
	}

	ok(4, "validator YES")

	commitAll(ws, "retarget procedural: "+p.Name)

	showPatchDiff(4, p.Path, scriptPath)

	p.Status = StatusAppliedClaude
}

func printSummary(cfg *Config, patches []*Patch) {
	counts := map[PatchStatus]int{}

	for _, p := range patches {
		counts[p.Status]++
	}

	banner("SUMMARY")

	fmt.Fprintln(os.Stderr, "  "+clr(clrG, "✓ applied:           ")+fmt.Sprintf("%d", counts[StatusApplied]))
	fmt.Fprintln(os.Stderr, "  "+clr(clrG, "✓ claude retarget:   ")+fmt.Sprintf("%d", counts[StatusAppliedClaude]))
	fmt.Fprintln(os.Stderr, "  "+clr(clrY, "⊘ obsolete:          ")+fmt.Sprintf("%d", counts[StatusObsolete]))
	fmt.Fprintln(os.Stderr, "  "+clr(clrBold, "  total:              ")+fmt.Sprintf("%d", len(patches)))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  "+clr(clrDim, "artifacts: ")+cfg.OutDir)
	fmt.Fprintln(os.Stderr, "  "+clr(clrDim, "report:    ")+filepath.Join(cfg.OutDir, "report.txt"))
}
