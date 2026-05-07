package main

import (
	"bytes"
	"embed"
	"path/filepath"
	"text/template"
)

//go:embed prompts/*.md
var promptFS embed.FS

var prompts = map[string]*template.Template{
	"writer-text":          mustTemplate("writer-text.md"),
	"writer-procedural":    mustTemplate("writer-procedural.md"),
	"validator-procedural": mustTemplate("validator-procedural.md"),
}

func mustTemplate(name string) *template.Template {
	data := Throw2(promptFS.ReadFile("prompts/" + name))

	return Throw2(template.New(name).Parse(string(data)))
}

type promptData struct {
	Workdir   string
	PatchPath string
	OutPath   string
	PatchName string
	PrevError string
}

func render(name string, data promptData) string {
	var buf bytes.Buffer
	Throw(prompts[name].Execute(&buf, data))

	return buf.String()
}

func writerPromptText(p *Patch, ws *Workspace, prevError string) string {
	relPatch := Throw2(filepath.Rel(ws.Root, p.Path))

	return render("writer-text", promptData{
		Workdir:   ws.Root,
		PatchPath: relPatch,
		PatchName: p.Name,
		PrevError: prevError,
	})
}

func writerPromptProcedural(p *Patch, ws *Workspace, prevError string) string {
	relPatch := Throw2(filepath.Rel(ws.Root, p.Path))
	relOut := Throw2(filepath.Rel(ws.Root, filepath.Join(ws.B, "patches", p.Name)))

	return render("writer-procedural", promptData{
		Workdir:   ws.Root,
		PatchPath: relPatch,
		OutPath:   relOut,
		PatchName: p.Name,
		PrevError: prevError,
	})
}

func validatorPromptProcedural(p *Patch, ws *Workspace) string {
	return render("validator-procedural", promptData{
		Workdir:   ws.Root,
		PatchName: p.Name,
	})
}
