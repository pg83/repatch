package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Workspace struct {
	Root string
	A    string
	B    string
}

func setupWorkspace(cfg *Config) *Workspace {
	Throw(os.MkdirAll(cfg.Workdir, 0755))

	ws := &Workspace{
		Root: cfg.Workdir,
		A:    filepath.Join(cfg.Workdir, "a"),
		B:    filepath.Join(cfg.Workdir, "b"),
	}

	Throw(os.MkdirAll(ws.A, 0755))
	copyTree(cfg.SrcOld, ws.A)

	Throw(os.MkdirAll(ws.B, 0755))

	fetchSrcNew(cfg.SrcNew, ws.B)

	Throw(os.MkdirAll(filepath.Join(ws.B, "patches"), 0755))

	gitInWS(ws, "init", "-q")
	gitInWS(ws, "config", "user.email", "repatch@local")
	gitInWS(ws, "config", "user.name", "repatch")
	gitInWS(ws, "add", "-A")
	gitInWS(ws, "commit", "-q", "-m", "initial: a/ + b/")

	return ws
}

func copyTree(src, dst string) {
	cmd := exec.Command("cp", "-a", src+"/.", dst+"/")
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		ThrowFmt("cp -a %s/. %s/: %v", src, dst, err)
	}
}

// fetchSrcNew populates dst from src, where src is one of:
//   - http(s)://...    download tarball, pipe into tar
//   - /path/to/dir     local directory, copied with cp -a (contents → dst)
//   - /path/to/file    local tarball file, piped into tar
func fetchSrcNew(src, dst string) {
	switch {
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		fetchHTTPInto(src, dst)
	default:
		info := Throw2(os.Stat(src))

		if info.IsDir() {
			copyTree(src, dst)
		} else {
			extractFileInto(src, dst)
		}
	}

	flattenSingleTopdir(dst)
}

func fetchHTTPInto(url, dst string) {
	resp := Throw2(http.Get(url))

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		ThrowFmt("http %d for %s", resp.StatusCode, url)
	}

	pipeTar(resp.Body, url, dst)
}

func extractFileInto(path, dst string) {
	f := Throw2(os.Open(path))

	defer f.Close()

	pipeTar(f, path, dst)
}

func pipeTar(src io.Reader, hint, dst string) {
	bin, args := tarCmdFor(hint)

	cmd := exec.Command(bin, args...)
	cmd.Dir = dst
	cmd.Stdin = src
	cmd.Stderr = os.Stderr

	Throw(cmd.Run())
}

func tarCmdFor(url string) (string, []string) {
	lower := strings.ToLower(url)

	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar", []string{"-xzf", "-"}
	case strings.HasSuffix(lower, ".tar.zst"), strings.HasSuffix(lower, ".tzst"):
		return "tar", []string{"--use-compress-program=unzstd", "-xf", "-"}
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return "tar", []string{"-xjf", "-"}
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return "tar", []string{"-xJf", "-"}
	case strings.HasSuffix(lower, ".tar"):
		return "tar", []string{"-xf", "-"}
	case strings.HasSuffix(lower, ".zip"):
		return "bsdtar", []string{"-xf", "-"}
	}

	return "tar", []string{"-xf", "-"}
}

// flattenSingleTopdir collapses one-level wrapper dirs (foo-1.2.3/<files>) so the
// extracted tree is flush with dst/, like the user's existing src/ layout.
func flattenSingleTopdir(dst string) {
	entries := Throw2(os.ReadDir(dst))

	if len(entries) != 1 || !entries[0].IsDir() {
		return
	}

	inner := filepath.Join(dst, entries[0].Name())
	sub := Throw2(os.ReadDir(inner))

	for _, e := range sub {
		Throw(os.Rename(filepath.Join(inner, e.Name()), filepath.Join(dst, e.Name())))
	}

	Throw(os.Remove(inner))
}

// gitInWS runs `git <args...>` in ws.Root and throws on non-zero with stderr
// captured. Used for setup-time and per-patch commit/checkout/clean.
func gitInWS(ws *Workspace, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = ws.Root

	out, err := cmd.CombinedOutput()

	if err != nil {
		ThrowFmt("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// gitOutInWS is gitInWS but returns stdout (combined output, not stderr-isolated).
func gitOutInWS(ws *Workspace, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = ws.Root

	out, err := cmd.Output()

	if err != nil {
		ThrowFmt("git %s: %v", strings.Join(args, " "), err)
	}

	return string(out)
}

func commitAll(ws *Workspace, msg string) {
	gitInWS(ws, "add", "-A")
	gitInWS(ws, "commit", "-q", "-m", msg)
}

// revertB drops every change Claude made under b/ since HEAD: tracked file
// reverts via checkout, untracked files via clean. We never touch a/ here —
// a/ is read-only by contract for the whole run.
func revertB(ws *Workspace) {
	gitInWS(ws, "checkout", "HEAD", "--", "b")
	gitInWS(ws, "clean", "-fdq", "b")
}

// dirtyPaths returns paths (relative to ws.Root) that differ from HEAD or are
// untracked. Order matters: the validator inspects this to detect strays.
func dirtyPaths(ws *Workspace) []string {
	out := gitOutInWS(ws, "status", "--porcelain", "--untracked-files=all")

	var paths []string

	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}

		paths = append(paths, strings.TrimSpace(line[3:]))
	}

	return paths
}

// diffB returns `git diff HEAD -- b/` rewritten so the leading b/ is stripped
// from path lines — the resulting unified diff applies inside b/ with -p1,
// matching the convention of the originals.
func diffB(ws *Workspace) []byte {
	out := []byte(gitOutInWS(ws, "diff", "--no-color", "HEAD", "--src-prefix=a/", "--dst-prefix=b/", "--", "b"))

	return rewriteBPaths(out)
}

// rewriteBPaths strips one b/ level from "diff --git", "---", "+++" lines.
func rewriteBPaths(diff []byte) []byte {
	lines := strings.Split(string(diff), "\n")

	for i, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "diff --git a/b/") && strings.Contains(ln, " b/b/"):
			ln = strings.Replace(ln, "a/b/", "a/", 1)
			lines[i] = strings.Replace(ln, "b/b/", "b/", 1)
		case strings.HasPrefix(ln, "--- a/b/"):
			lines[i] = strings.Replace(ln, "--- a/b/", "--- a/", 1)
		case strings.HasPrefix(ln, "+++ b/b/"):
			lines[i] = strings.Replace(ln, "+++ b/b/", "+++ b/", 1)
		}
	}

	return []byte(strings.Join(lines, "\n"))
}

func copyFile(src, dst string) {
	data := Throw2(os.ReadFile(src))
	Throw(os.WriteFile(dst, data, 0644))
}

// extractPatchHeader returns the prelude of a unified-diff file — every byte
// up to and including the newline before the first `diff --git ` or `--- `
// line. Patches written by `git format-patch` or by hand often carry an
// email-style header / commit message / explanatory text that conveys
// WHY the patch exists; `git diff HEAD` produces only hunks, so we splice
// the prelude back on top of our derived diff.
func extractPatchHeader(patchPath string) string {
	data := Throw2(os.ReadFile(patchPath))
	text := string(data)

	for _, marker := range []string{"\ndiff --git ", "\n--- "} {
		if i := strings.Index(text, marker); i >= 0 {
			return text[:i+1]
		}
	}

	return ""
}

// diffFiles returns the unified diff between two files. diff(1) returns
// rc=1 on differences (not an error for us); rc>=2 is a real failure.
func diffFiles(a, b string) (string, error) {
	cmd := exec.Command("diff", "-u", a, b)
	out, err := cmd.Output()

	if ee, ok := err.(*exec.ExitError); ok {
		if ee.ExitCode() == 1 {
			return string(out), nil
		}
	}

	return string(out), err
}

func cleanupWorkspace(ws *Workspace) {
	if err := os.RemoveAll(ws.Root); err != nil {
		fmt.Fprintln(os.Stderr, "repatch: cleanup:", err)
	}
}
