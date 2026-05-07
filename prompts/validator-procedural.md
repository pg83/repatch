# Task: validate a procedural-patch retarget

Workdir: `{{.Workdir}}`.
`a/` is the OLD codebase (target state of the original script).
`b/` is the NEW codebase after a freshly retargeted script ran on it.

Original script:   `a/patches/{{.PatchName}}`
Retargeted script: `b/patches/{{.PatchName}}`

## Question

Are the changes the new script applied to `b/` semantically equivalent to what the original script achieves in `a/`?

Compare INTENT, not byte-equality (paths/identifiers/spellings may differ).

## Output format (REQUIRED)

First line MUST be exactly `VERDICT: YES` or `VERDICT: NO`.
Then a short explanation (1-3 sentences).
