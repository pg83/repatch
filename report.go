package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeReport drops a plain-text summary next to the artifacts. Patches are
// already in cfg.OutDir thanks to syncOut after each successful patch — this
// only writes report.txt.
func writeReport(cfg *Config, patches []*Patch) {
	sb := strings.Builder{}

	sb.WriteString("repatch report\n")
	sb.WriteString("==============\n\n")
	sb.WriteString(fmt.Sprintf("src-old: %s\n", cfg.SrcOld))
	sb.WriteString(fmt.Sprintf("src-new: %s\n", cfg.SrcNew))
	sb.WriteString(fmt.Sprintf("out:     %s\n\n", cfg.OutDir))

	counts := map[PatchStatus]int{}

	for _, p := range patches {
		counts[p.Status]++
	}

	sb.WriteString(fmt.Sprintf("applied:    %d\n", counts[StatusApplied]))
	sb.WriteString(fmt.Sprintf("retargeted: %d\n", counts[StatusAppliedClaude]))
	sb.WriteString(fmt.Sprintf("obsolete:   %d\n", counts[StatusObsolete]))
	sb.WriteString(fmt.Sprintf("total:      %d\n\n", len(patches)))

	sb.WriteString("per-patch:\n")

	for _, p := range patches {
		sb.WriteString(fmt.Sprintf("  %-16s %-11s attempts=%d  %s",
			p.Status, p.Type, p.Attempt, p.Name))

		if p.Notes != "" {
			sb.WriteString("  -- " + p.Notes)
		}

		sb.WriteString("\n")
	}

	Throw(os.WriteFile(filepath.Join(cfg.OutDir, "report.txt"), []byte(sb.String()), 0644))
}
