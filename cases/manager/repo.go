package manager

import (
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/storage/sqlite"
)

// openCaseRepo opens a SQLite-backed repository for a case database path.
// Isolated here so the manager depends on the storage package in one place.
func openCaseRepo(path string) (sdk.Repository, error) {
	return sqlite.NewRepository(path)
}
