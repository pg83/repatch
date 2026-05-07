package main

type PatchType int

const (
	PatchText PatchType = iota
	PatchProcedural
)

func (t PatchType) String() string {
	switch t {
	case PatchText:
		return "text"
	case PatchProcedural:
		return "procedural"
	}

	return "unknown"
}

type PatchStatus int

const (
	StatusQueued PatchStatus = iota
	StatusApplied
	StatusAppliedClaude
	StatusObsolete
)

func (s PatchStatus) String() string {
	switch s {
	case StatusQueued:
		return "queued"
	case StatusApplied:
		return "applied"
	case StatusAppliedClaude:
		return "applied-claude"
	case StatusObsolete:
		return "obsolete"
	}

	return "unknown"
}

type Patch struct {
	Name    string
	Path    string
	Type    PatchType
	Status  PatchStatus
	Notes   string
	Attempt int
}
