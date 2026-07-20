package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies every pending schema migration in filename order.
// Ordering is plain lexicographic sort.Strings, not numeric -- migration
// filenames (001_initial.sql, 002_..., see internal/store/sqlite/
// migrations/) must keep the same zero-padded digit width for every future
// migration, or a migration numbered 010+ would sort before 002.
//
// 한국어 요약: migrate.go는 sqlite 패키지에서 스키마 마이그레이션만
// 전담하는 파일이다. migrations/*.sql을 go:embed로 바이너리에 포함시킨
// 뒤, 파일명 "사전식(문자열)" 정렬 순서로 아직 schema_migrations 테이블에
// 기록되지 않은 마이그레이션만 골라 각각 트랜잭션 안에서 실행하고
// 기록한다. 숫자 크기 순서가 아니라 문자열 정렬이므로, 새 마이그레이션
// 파일을 추가할 때는 반드시 기존 파일들과 동일한 자리수의 0-패딩 번호를
// 유지해야 한다 -- 자리수가 어긋나면(예: 두 자리 "10_x.sql"이 세 자리
// "002_y.sql"보다 문자열상 앞에 와 버리는 경우) 실제로 의도한 적용
// 순서와 어긋난다. 이 파일이 만든 테이블 스키마 위에서
// project_repository.go 등 나머지 repository 파일들의 SQL이 동작한다.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)

	for _, name := range entries {
		if err := applyMigration(db, name); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(db *sql.DB, name string) error {
	var applied bool
	if err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", name,
	).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	if applied {
		return nil
	}

	contents, err := migrationFiles.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(string(contents)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	return nil
}
