package cmd

import (
	"database/sql"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

// db.go is the one place every other command in this package goes through
// to find and open the local SQLite database -- see docs/
// libra_integration_contracts.md §7.0 for why .libra.yaml/.libra.db live
// next to each other instead of a fixed OS-standard config directory.
const (
	defaultConfigFilename = ".libra.yaml"
	defaultDBFilename     = ".libra.db"
)

// configFilePath returns the --config path, or the default alongside the
// current directory when it was not set.
func configFilePath() string {
	if cfgPath != "" {
		return cfgPath
	}
	return defaultConfigFilename
}

// dbFilePath returns the SQLite database path, kept next to the config file.
func dbFilePath() string {
	return filepath.Join(filepath.Dir(configFilePath()), defaultDBFilename)
}

// openDatabase opens the local SQLite database and applies any pending
// migrations. The caller owns the returned database and must close it.
func openDatabase() (*sql.DB, error) {
	db, err := sqlite.Open(dbFilePath())
	if err != nil {
		return nil, err
	}
	if err := sqlite.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
