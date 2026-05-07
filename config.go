package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	SrcOld       string
	SrcNew       string
	PatchesDir   string
	OutDir       string
	Workdir      string
	CompactEvery int
	MaxRetries   int
	KeepWorkdir  bool
	ClaudeBin    string
}

func parseConfig() *Config {
	c := &Config{}

	flag.StringVar(&c.SrcOld, "src-old", "", "path to existing src dir (with patches/ subdir)")
	flag.StringVar(&c.SrcNew, "src-new", "", "new upstream: http(s)://... tarball, /path/to/file.tar.*, or /path/to/dir")
	flag.StringVar(&c.PatchesDir, "patches-dir", "", "patches dir override; default <src-old>/patches")
	flag.StringVar(&c.OutDir, "out", "", "output dir for retargeted patches")
	flag.StringVar(&c.Workdir, "workdir", "", "scratch workspace dir; default ./repatch-work-<ts>")
	flag.IntVar(&c.CompactEvery, "compact-every", 4, "0=one --continue session never compacted; 1=fresh per patch; N>1=/compact every N")
	flag.IntVar(&c.MaxRetries, "max-retries", 3, "max writer retries per patch")
	flag.BoolVar(&c.KeepWorkdir, "keep-workdir", false, "don't remove workdir on exit")
	flag.StringVar(&c.ClaudeBin, "claude", "claude", "claude binary path")

	flag.Parse()

	if c.SrcOld == "" || c.SrcNew == "" || c.OutDir == "" {
		fmt.Fprintln(os.Stderr, "usage: repatch --src-old PATH --src-new URL --out DIR [opts]")
		flag.PrintDefaults()
		os.Exit(2)
	}

	if c.PatchesDir == "" {
		c.PatchesDir = filepath.Join(c.SrcOld, "patches")
	}

	if c.Workdir == "" {
		c.Workdir = fmt.Sprintf("./repatch-work-%d", time.Now().Unix())
	}

	c.SrcOld = Throw2(filepath.Abs(c.SrcOld))
	c.PatchesDir = Throw2(filepath.Abs(c.PatchesDir))
	c.OutDir = Throw2(filepath.Abs(c.OutDir))
	c.Workdir = Throw2(filepath.Abs(c.Workdir))
	c.ClaudeBin = resolveBin(c.ClaudeBin, "--claude")

	return c
}

// resolveBin turns a CLI-supplied binary spec into an absolute path. A spec
// containing `/` is treated as a path (abs + stat); a bare name is looked up
// in PATH. Either way the returned path is one we know exists right now.
func resolveBin(spec, flagName string) string {
	if strings.Contains(spec, "/") {
		abs := Throw2(filepath.Abs(spec))

		if _, err := os.Stat(abs); err != nil {
			ThrowFatal("%s %s: not found at %s (%v)", flagName, spec, abs, err)
		}

		return abs
	}

	p, err := exec.LookPath(spec)

	if err != nil {
		ThrowFatal("%s %s: not in PATH: %v", flagName, spec, err)
	}

	return p
}
