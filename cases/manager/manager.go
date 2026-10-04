// Package manager implements case lifecycle management with filesystem
// isolation. Each case owns a directory under ~/.bctx/cases/<id>/ containing
// its own case.db and evidence/report/export/log subdirectories.
//
// The active case is tracked in a small pointer file so CLI invocations share
// state across runs. All operations are local; no network access occurs.
package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

var slugRe = regexp.MustCompile(`[^a-z0-9_-]+`)

// Manager implements sdk.CaseService over the local filesystem.
type Manager struct {
	layout configs.Layout
}

// New returns a case manager for the given layout.
func New(layout configs.Layout) *Manager {
	return &Manager{layout: layout}
}

// slug normalizes a case name into a filesystem-safe id.
func slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	return s
}

func (m *Manager) metadataPath(id string) string {
	return filepath.Join(m.layout.CaseDir(id), "metadata.json")
}

func (m *Manager) activePointer() string {
	return filepath.Join(m.layout.Cases, ".active")
}

// subdirs are the per-case directories created on case creation.
var subdirs = []string{"imports", "evidence", "reports", "exports", "logs"}

// Create creates a new isolated case. It fails if the case already exists.
func (m *Manager) Create(ctx context.Context, name string) (schema.Case, error) {
	id := slug(name)
	if id == "" {
		return schema.Case{}, fmt.Errorf("invalid case name %q", name)
	}
	dir := m.layout.CaseDir(id)
	if _, err := os.Stat(dir); err == nil {
		return schema.Case{}, fmt.Errorf("case %q already exists", id)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return schema.Case{}, fmt.Errorf("create case dir: %w", err)
	}
	for _, sd := range subdirs {
		if err := os.MkdirAll(filepath.Join(dir, sd), 0o755); err != nil {
			return schema.Case{}, fmt.Errorf("create %s: %w", sd, err)
		}
	}

	now := time.Now().UTC()
	c := schema.Case{
		ID:        id,
		Name:      name,
		Status:    schema.CaseActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := m.writeMetadata(c); err != nil {
		return schema.Case{}, err
	}
	// Initialize the case database (applies migrations) so it is ready.
	repo, err := m.OpenRepository(id)
	if err != nil {
		return schema.Case{}, err
	}
	_ = repo.AppendAudit(ctx, schema.AuditEvent{
		ID: fmt.Sprintf("audit-%d", now.UnixNano()), CaseID: id,
		Action: schema.AuditCaseCreated, Subject: id, Result: "ok", Timestamp: now,
	})
	_ = repo.Close()

	if err := m.setActive(id); err != nil {
		return schema.Case{}, err
	}
	return c, nil
}

// Open marks a case active and returns it.
func (m *Manager) Open(ctx context.Context, name string) (schema.Case, error) {
	id := slug(name)
	c, err := m.read(id)
	if err != nil {
		return schema.Case{}, err
	}
	if err := m.setActive(id); err != nil {
		return schema.Case{}, err
	}
	return c, nil
}

// List returns all cases discovered on disk.
func (m *Manager) List(ctx context.Context) ([]schema.Case, error) {
	entries, err := os.ReadDir(m.layout.Cases)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	var out []schema.Case
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := m.read(e.Name())
		if err != nil {
			continue // skip malformed/non-case dirs
		}
		out = append(out, c)
	}
	return out, nil
}

// Close marks a case closed (does not delete data).
func (m *Manager) Close(ctx context.Context, name string) error {
	id := slug(name)
	c, err := m.read(id)
	if err != nil {
		return err
	}
	c.Status = schema.CaseClosed
	c.UpdatedAt = time.Now().UTC()
	return m.writeMetadata(c)
}

// Active returns the currently active case, if any.
func (m *Manager) Active() (schema.Case, bool) {
	data, err := os.ReadFile(m.activePointer())
	if err != nil {
		return schema.Case{}, false
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return schema.Case{}, false
	}
	c, err := m.read(id)
	if err != nil {
		return schema.Case{}, false
	}
	return c, true
}

// OpenRepository opens the SQLite repository for a case id.
func (m *Manager) OpenRepository(id string) (sdk.Repository, error) {
	return openCaseRepo(filepath.Join(m.layout.CaseDir(id), "case.db"))
}

func (m *Manager) setActive(id string) error {
	return os.WriteFile(m.activePointer(), []byte(id), 0o644)
}

func (m *Manager) writeMetadata(c schema.Case) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.metadataPath(c.ID), data, 0o644)
}

func (m *Manager) read(id string) (schema.Case, error) {
	data, err := os.ReadFile(m.metadataPath(id))
	if err != nil {
		return schema.Case{}, fmt.Errorf("case %q not found", id)
	}
	var c schema.Case
	if err := json.Unmarshal(data, &c); err != nil {
		return schema.Case{}, fmt.Errorf("read case %q: %w", id, err)
	}
	return c, nil
}

var _ sdk.CaseService = (*Manager)(nil)
