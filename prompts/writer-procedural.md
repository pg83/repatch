# Task: creatively retarget a PROCEDURAL patch from a/ to b/

Workdir: `{{.Workdir}}` (a git repo).
`a/` is the OLD codebase with this script's effect already baked in.
`b/` is the NEW upstream.

Old script: `{{.PatchPath}}` (treat as INPUT, do not modify).
Write a NEW script to: `{{.OutPath}}`

## Approach

1. Read the old script. Understand WHAT it does at a high level (intent).
2. Look at how `a/` is structured (which files match the script's targets).
3. Look at how `b/` is structured (paths/names may have changed).
4. Write a NEW script that, when run from `b/` cwd, does the semantic equivalent on the new tree.
5. Output ONLY the new script file at the path above. Do NOT touch other files. Do NOT run the script — the harness will run and validate it.

## When the patch is obsolete

If, after reading the script and the relevant code in BOTH `a/` and `b/`, you are CONFIDENT the script's effect is no longer needed — its target files no longer exist, its substitutions are moot, or upstream has already incorporated the equivalent change — DO NOT write any new script. Instead, end your response with one line:

OBSOLETE: <one-line reason>

The harness will skip the patch and mark it obsolete. Use this only when you are confident; when in doubt, retarget.
{{if .PrevError}}
## Previous attempt failed

{{.PrevError}}

Try again, addressing the failure.
{{end}}
