// Package sqlite provides the SQLite persistence foundation for Libra.
package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// sqlite.go는 sqlite 패키지의 진입점이다. 데이터베이스 연결을 여는
// Open과, 연결 직후 항상 적용해야 하는 PRAGMA 설정(외래 키 강제,
// 동시성 대기 타임아웃)을 담당한다. modernc.org/sqlite를 blank
// import해서 cgo 없이 동작하는 순수 Go SQLite 드라이버를 등록한다.
// migrate.go와 project_repository.go/resource_repository.go/
// dependency_repository.go/scan_repository.go/workspace_repository.go는
// 모두 이 Open이 반환한 *sql.DB를 넘겨받아 동작하는 소비자들이다.
// Open opens a SQLite database and configures the connection invariants used
// by Libra. The caller owns the returned database and must close it.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	if err := configure(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func configure(db *sql.DB) error {
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("configure sqlite database: %w", err)
		}
	}

	return nil
}
