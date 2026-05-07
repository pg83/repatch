package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// tryApplyFile lands the given patch file (text or procedural) on current b/.
// On success: copies the file into b/patches/, commits, returns true.
// On any failure: reverts b/, returns false. Pure attempt — no retry.
//
// "Cleanly applies" means:
//   - text patch: `git apply --check` passes, then actual apply succeeds.
//   - procedural patch: script runs with rc=0 AND produces a non-empty diff.
func tryApplyFile(ws *Workspace, p *Patch, srcPath string) bool {
	dst := filepath.Join(ws.B, "patches", p.Name)

	if p.Type == PatchText {
		check := exec.Command("git", "apply", "--check", "--directory=b/", srcPath)
		check.Dir = ws.Root

		if err := check.Run(); err != nil {
			return false
		}

		copyFile(srcPath, dst)

		cmd := exec.Command("git", "apply", "--directory=b/", srcPath)
		cmd.Dir = ws.Root

		if _, err := cmd.CombinedOutput(); err != nil {
			revertB(ws)

			return false
		}

		commitAll(ws, "apply text: "+p.Name)

		return true
	}

	copyFile(srcPath, dst)
	Throw(os.Chmod(dst, 0755))

	runner := scriptRunner(dst)

	cmd := exec.Command(runner[0], dst)
	cmd.Dir = ws.B
	cmd.Stderr = os.Stderr

	if _, err := cmd.Output(); err != nil {
		revertB(ws)

		return false
	}

	if len(diffB(ws)) == 0 {
		revertB(ws)

		return false
	}

	commitAll(ws, "apply procedural: "+p.Name)

	return true
}

// loadObsoleteMarker returns (true, reason) if cfg.OutDir contains an
// obsolete marker for p. Markers are sticky: removed only by the operator.
func loadObsoleteMarker(cfg *Config, p *Patch) (bool, string) {
	data, err := os.ReadFile(obsoleteMarkerPath(cfg, p))

	if err != nil {
		return false, ""
	}

	return true, strings.TrimSpace(string(data))
}

func obsoleteMarkerPath(cfg *Config, p *Patch) string {
	return filepath.Join(cfg.OutDir, p.Name+".obsolete")
}

func markObsolete(cfg *Config, p *Patch, reason string) {
	Throw(os.WriteFile(obsoleteMarkerPath(cfg, p), []byte(reason+"\n"), 0644))
}

// invalidateOut removes the existing artifact for p so the next pass
// rebuilds it from scratch. Obsolete markers are left alone — those
// require manual deletion to force re-evaluation.
func invalidateOut(cfg *Config, p *Patch) {
	_ = os.Remove(filepath.Join(cfg.OutDir, p.Name))
}

// syncOut copies b/patches/<name> to cfg.OutDir/<name>. Idempotent: if the
// source matches the dest (we just applied an existing artifact), the file
// is byte-identical. Skipped for obsolete patches — there's no patch file.
func syncOut(cfg *Config, ws *Workspace, p *Patch) {
	src := filepath.Join(ws.B, "patches", p.Name)
	dst := filepath.Join(cfg.OutDir, p.Name)
	copyFile(src, dst)
}
