# Task: creatively retarget a TEXT patch from a/ to b/

Workdir: `{{.Workdir}}` (a git repo).
`a/` is the OLD codebase with the patch already applied.
`b/` is the NEW upstream where the patch needs to land.

Patch to retarget: `{{.PatchPath}}`

## Approach

1. Read the patch.
2. Read the surrounding code in BOTH `a/` and `b/` to understand WHAT the patch does and WHY.
3. Modify files inside `b/` to achieve the semantic equivalent.
4. Do NOT touch anything outside `b/`. Do NOT modify `b/patches/`. Do NOT touch `a/`. The harness will diff and save the new patch.

Be creative — the upstream may have moved code around, renamed identifiers, restructured. Keep the INTENT of the original patch.

## When the patch is obsolete

If, after reading the patch and the relevant code in BOTH `a/` and `b/`, you are CONFIDENT the patch is no longer needed — the upstream now does the equivalent natively, the patched code no longer exists with no replacement, or the patch's intent is moot for the new version — DO NOT modify any files. Instead, end your response with one line:

OBSOLETE: <one-line reason>

The harness will skip the patch and mark it obsolete. Use this only when you are confident; when in doubt, retarget.
{{if .PrevError}}
## Previous attempt failed

{{.PrevError}}

Try again, addressing the failure.
{{end}}
