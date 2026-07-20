// [한국어 설명] db.go는 cmd 패키지의 모든 명령이 공유하는 로컬 SQLite
// 데이터베이스 접근 지점을 한 곳에 모아둔 파일이다. 설정 파일(.libra.yaml)
// 경로 계산(configFilePath), DB 파일 경로 계산(dbFilePath), DB 열기 및
// 마이그레이션 적용(openDatabase)을 제공한다. scan.go, init.go,
// projects.go, summary.go 등 DB를 사용하는 모든 명령 파일이 이 파일의
// openDatabase()를 호출하며, 각자 별도의 DB 연결/마이그레이션 로직을
// 두지 않는다 -- 이렇게 분리해 둔 덕분에 DB 위치 규칙이 바뀌어도 이
// 파일 하나만 고치면 된다.
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
