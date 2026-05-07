package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadPatches(cfg *Config) []*Patch {
	entries := Throw2(os.ReadDir(cfg.PatchesDir))

	var patches []*Patch

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()

		patches = append(patches, &Patch{
			Name: name,
			Path: filepath.Join(cfg.PatchesDir, name),
			Type: classifyPatch(name),
		})
	}

	sort.Slice(patches, func(i, j int) bool {
		return patches[i].Name < patches[j].Name
	})

	return patches
}

func classifyPatch(name string) PatchType {
	lower := strings.ToLower(name)

	if strings.HasSuffix(lower, ".patch") || strings.HasSuffix(lower, ".diff") {
		return PatchText
	}

	return PatchProcedural
}
