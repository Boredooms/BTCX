package screens

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Block is the Block Detail screen (Phase 8.5). It loads a locally-stored block
// (by hash, or by height when the subject id is numeric) and renders the header
// + its transaction list. All data is local (the block was acquired earlier,
// online; everything here is offline). Selecting a transaction opens the
// Transaction screen. Nothing here touches the network or computes analysis.
type Block struct {
	ctx    *ScreenCtx
	styles theme.Styles

	subject string
	detail  components.DetailPanel
	block   *schema.Block
	loaded  bool
}

// NewBlock builds the Block screen for the current subject (hash or height).
func NewBlock(ctx *ScreenCtx) *Block {
	return &Block{
		ctx:     ctx,
		styles:  ctx.styles(),
		subject: ctx.Subject.ID,
		detail:  components.NewDetailPanel(ctx.styles()),
	}
}

// Init loads the block from the local repository (no network).
func (b *Block) Init() tea.Cmd {
	if b.subject == "" {
		b.detail.SetError("no block selected")
		return nil
	}
	repo := b.ctx.repo()
	if repo == nil {
		b.detail.SetError(errNoCase.Error())
		return nil
	}
	b.detail.SetLoading()
	return blockCmd(b.ctx.bgCtx(), repo, b.subject)
}

// Update folds in the loaded block.
func (b *Block) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if m.Request != "block:"+b.subject {
			return b, nil
		}
		if blk, ok := m.Payload.(*schema.Block); ok {
			b.block, b.loaded = blk, true
			b.rebuild()
		}
	case dataError:
		if m.Request == "block:"+b.subject {
			b.detail.SetError(m.Err.Error())
		}
	case tea.KeyMsg:
		var cmd tea.Cmd
		b.detail, cmd = b.detail.Update(m)
		return b, cmd
	}
	return b, nil
}

func (b *Block) rebuild() {
	if b.block == nil {
		b.detail.SetSections([]components.Section{{
			Title: "block",
			Rows: []components.Row{{Label: "status",
				Value: "not found locally — acquire it first (online), then investigate offline"}},
		}})
		return
	}
	blk := b.block
	ts := "unavailable"
	if !blk.Timestamp.IsZero() {
		ts = blk.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	header := components.Section{Title: "block", Rows: []components.Row{
		{Label: "hash", Value: blk.Hash},
		{Label: "height", Value: strconv.Itoa(blk.Height)},
		{Label: "time", Value: ts},
		{Label: "prev", Value: orUnavail(blk.PrevHash)},
		{Label: "transactions", Value: strconv.Itoa(blk.TxCount)},
		{Label: "size", Value: unavailIfZero(blk.Size)},
		{Label: "weight", Value: unavailIfZero(blk.Weight)},
		{Label: "merkle root", Value: orUnavail(blk.MerkleRoot)},
		{Label: "source", Value: fmt.Sprintf("%s (%s)", blk.Provenance.SourceIdentifier, blk.Provenance.SourceType)},
	}}

	// Transaction list (bounded preview): first N txids, each a drill-down.
	const previewN = 30
	txRows := make([]components.Row, 0, previewN)
	for i, id := range blk.TxIDs {
		if i >= previewN {
			break
		}
		txRows = append(txRows, components.Row{Label: fmt.Sprintf("tx %d", i+1), Value: shortID(id)})
	}
	title := fmt.Sprintf("transactions (%d", len(blk.TxIDs))
	if len(blk.TxIDs) > previewN {
		title += fmt.Sprintf(", showing %d", previewN)
	}
	title += ")"

	b.detail.SetSections([]components.Section{
		header,
		{Title: title, Rows: txRows, Body: "select a transaction and press enter to inspect it (offline)"},
	})
}

// View renders the block detail panel.
func (b *Block) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	if b.subject == "" {
		return placeholderView(b.styles, "BLOCK",
			"no block selected — search a block hash or height, then acquire it", f)
	}
	title := b.styles.Title.Render("BLOCK") + " " + b.styles.Value.Render(shortID(b.subject))
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	return clampBlockLocal(title+"\n"+b.detail.View(components.Frame{W: f.W, H: bodyH}), f)
}

// ShortHelp lists Block's context keys.
func (b *Block) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open transaction")),
		key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
	}
}

func orUnavail(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unavailable"
	}
	return s
}

func unavailIfZero(n int) string {
	if n == 0 {
		return "unavailable"
	}
	return strconv.Itoa(n)
}

// blockCmd loads a block by hash, or by height when the id is all digits. The
// result is tagged "block:<subject>" so Update can match it. Local-only.
func blockCmd(ctx context.Context, repo sdk.Repository, subject string) tea.Cmd {
	return func() tea.Msg {
		var blk *schema.Block
		var err error
		if h, e := strconv.Atoi(subject); e == nil && h >= 0 {
			blk, err = repo.GetBlockByHeight(ctx, h)
		} else {
			blk, err = repo.GetBlock(ctx, subject)
		}
		if err != nil {
			return dataError{Request: "block:" + subject, Err: err}
		}
		return dataLoaded{Request: "block:" + subject, Payload: blk}
	}
}
