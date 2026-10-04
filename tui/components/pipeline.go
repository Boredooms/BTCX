package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
)

// PipelineStage is one step in the analysis pipeline readout.
type PipelineStage struct {
	Label  string
	Status StageStatus
	Detail string // short honest note (e.g. "anomaly 1.00", "5 items")
}

// StageStatus is the lifecycle of a pipeline stage.
type StageStatus int

const (
	// StagePending: not started.
	StagePending StageStatus = iota
	// StageRunning: in progress (the active stage).
	StageRunning
	// StageDone: completed, with real output present.
	StageDone
	// StageSkipped: not applicable (e.g. report not yet requested).
	StageSkipped
	// StageFailed: errored.
	StageFailed
)

// Pipeline renders a vertical staged checklist for the analysis run (querying →
// features → ML → risk → evidence → report). It is PRESENTATION ONLY: the owning
// screen sets each stage's status from REAL evidence that the step ran (a stage
// is marked done only when its real output is present in the result), so the
// readout never fabricates progress.
type Pipeline struct {
	styles theme.Styles
	stages []PipelineStage
	title  string
}

// NewPipeline builds a pipeline readout with the given title.
func NewPipeline(styles theme.Styles, title string) Pipeline {
	return Pipeline{styles: styles, title: title}
}

// SetStages replaces the stage list.
func (p *Pipeline) SetStages(stages []PipelineStage) { p.stages = stages }

// StagesCopy returns a copy of the current stages so a caller can mutate one
// (e.g. mark the running stage failed) and set it back without aliasing.
func (p Pipeline) StagesCopy() []PipelineStage {
	out := make([]PipelineStage, len(p.stages))
	copy(out, p.stages)
	return out
}

// glyph returns the status glyph (ASCII in degraded mode).
func (p Pipeline) glyph(s StageStatus) (string, theme.Role) {
	degraded := p.styles.Theme().Degraded
	switch s {
	case StageDone:
		if degraded {
			return "[x]", theme.RoleHealthy
		}
		return "✓", theme.RoleHealthy
	case StageRunning:
		if degraded {
			return "[>]", theme.RoleInfo
		}
		return "▶", theme.RoleInfo
	case StageFailed:
		if degraded {
			return "[!]", theme.RoleCritical
		}
		return "✗", theme.RoleCritical
	case StageSkipped:
		if degraded {
			return "[-]", theme.RoleMuted
		}
		return "·", theme.RoleMuted
	default: // pending
		if degraded {
			return "[ ]", theme.RoleMuted
		}
		return "○", theme.RoleMuted
	}
}

// View renders the staged checklist within the frame.
func (p Pipeline) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	if p.title != "" {
		b.WriteString(p.styles.Role(theme.RoleTitle).Render(p.title))
		b.WriteString("\n")
	}
	rows := f.H
	if p.title != "" {
		rows--
	}
	for i, st := range p.stages {
		if i >= rows {
			break
		}
		gl, role := p.glyph(st.Status)
		line := p.styles.Role(role).Render(gl) + " " +
			p.styles.Role(stageLabelRole(st.Status)).Render(st.Label)
		if st.Detail != "" {
			line += "  " + p.styles.Role(theme.RoleLabel).Render(st.Detail)
		}
		b.WriteString(clampLine(line, f.W))
		if i < len(p.stages)-1 && i < rows-1 {
			b.WriteString("\n")
		}
	}
	return clampBlock(b.String(), f)
}

// stageLabelRole dims pending/skipped labels and brightens the active/done ones.
func stageLabelRole(s StageStatus) theme.Role {
	switch s {
	case StageDone:
		return theme.RoleValue
	case StageRunning:
		return theme.RoleInfo
	case StageFailed:
		return theme.RoleCritical
	default:
		return theme.RoleMuted
	}
}
