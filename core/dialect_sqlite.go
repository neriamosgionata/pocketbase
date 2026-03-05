package core

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/pocketbase/dbx"
)

var _ DBDialect = (*SQLiteDialect)(nil)

// SQLiteDialect implements DBDialect for SQLite databases.
type SQLiteDialect struct{}

func (d *SQLiteDialect) Name() string {
	return "sqlite"
}

func (d *SQLiteDialect) Connect(dsn string) (*dbx.DB, error) {
	// Note: the busy_timeout pragma must be first because
	// the connection needs to be set to block on busy before WAL mode
	// is set in case it hasn't been already set by another connection.
	pragmas := "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=journal_size_limit(200000000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-32000)"

	db, err := dbx.Open("sqlite", dsn+pragmas)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// --- Schema introspection ---

func (d *SQLiteDialect) TableColumns(db dbx.Builder, tableName string) ([]string, error) {
	columns := []string{}

	err := db.NewQuery("SELECT name FROM PRAGMA_TABLE_INFO({:tableName})").
		Bind(dbx.Params{"tableName": tableName}).
		Column(&columns)

	return columns, err
}

func (d *SQLiteDialect) TableInfo(db dbx.Builder, tableName string) ([]*TableInfoRow, error) {
	info := []*TableInfoRow{}

	err := db.NewQuery("SELECT * FROM PRAGMA_TABLE_INFO({:tableName})").
		Bind(dbx.Params{"tableName": tableName}).
		All(&info)
	if err != nil {
		return nil, err
	}

	if len(info) == 0 {
		return nil, fmt.Errorf("empty table info probably due to invalid or missing table %s", tableName)
	}

	return info, nil
}

func (d *SQLiteDialect) TableIndexes(db dbx.Builder, tableName string) (map[string]string, error) {
	indexes := []struct {
		Name string
		Sql  string
	}{}

	err := db.Select("name", "sql").
		From("sqlite_master").
		AndWhere(dbx.NewExp("sql is not null")).
		AndWhere(dbx.HashExp{
			"type":     "index",
			"tbl_name": tableName,
		}).
		All(&indexes)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(indexes))

	for _, idx := range indexes {
		result[idx.Name] = idx.Sql
	}

	return result, nil
}

func (d *SQLiteDialect) HasTable(db dbx.Builder, tableName string) bool {
	var exists int

	err := db.Select("(1)").
		From("sqlite_schema").
		AndWhere(dbx.HashExp{"type": []any{"table", "view"}}).
		AndWhere(dbx.NewExp("LOWER([[name]])=LOWER({:tableName})", dbx.Params{"tableName": tableName})).
		Limit(1).
		Row(&exists)

	return err == nil && exists > 0
}

func (d *SQLiteDialect) Vacuum(db dbx.Builder) error {
	_, err := db.NewQuery("VACUUM").Execute()
	return err
}

func (d *SQLiteDialect) OptimizeAfterDDL(db dbx.Builder, logger *slog.Logger) {
	_, err := db.NewQuery("PRAGMA optimize").Execute()
	if err != nil {
		logger.Warn("Failed to run PRAGMA optimize after DDL", slog.String("error", err.Error()))
	}
}

// --- Maintenance ---

func (d *SQLiteDialect) PeriodicOptimize(nonconcurrentDB dbx.Builder, auxNonconcurrentDB dbx.Builder, logger *slog.Logger) {
	_, execErr := nonconcurrentDB.NewQuery("PRAGMA wal_checkpoint(TRUNCATE)").Execute()
	if execErr != nil {
		logger.Warn("Failed to run periodic PRAGMA wal_checkpoint for the main DB", slog.String("error", execErr.Error()))
	}

	_, execErr = auxNonconcurrentDB.NewQuery("PRAGMA wal_checkpoint(TRUNCATE)").Execute()
	if execErr != nil {
		logger.Warn("Failed to run periodic PRAGMA wal_checkpoint for the auxiliary DB", slog.String("error", execErr.Error()))
	}

	_, execErr = nonconcurrentDB.NewQuery("PRAGMA optimize").Execute()
	if execErr != nil {
		logger.Warn("Failed to run periodic PRAGMA optimize", slog.String("error", execErr.Error()))
	}
}

// --- Column types ---

func (d *SQLiteDialect) PrimaryKeyColumnType() string {
	return "TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL"
}

// --- SQL expressions ---

func (d *SQLiteDialect) JSONExtract(column string, path string) string {
	if path != "" && !strings.HasPrefix(path, "[") {
		path = "." + path
	}

	return fmt.Sprintf(
		"(CASE WHEN json_valid([[%s]]) THEN JSON_EXTRACT([[%s]], '$%s') ELSE JSON_EXTRACT(json_object('pb', [[%s]]), '$.pb%s') END)",
		column,
		column,
		path,
		column,
		path,
	)
}

func (d *SQLiteDialect) JSONEach(column string) string {
	return fmt.Sprintf(
		`json_each(CASE WHEN iif(json_valid([[%s]]), json_type([[%s]])='array', FALSE) THEN [[%s]] ELSE json_array([[%s]]) END)`,
		column, column, column, column,
	)
}

func (d *SQLiteDialect) JSONArrayLength(column string) string {
	return fmt.Sprintf(
		`json_array_length(CASE WHEN iif(json_valid([[%s]]), json_type([[%s]])='array', FALSE) THEN [[%s]] ELSE (CASE WHEN [[%s]] = '' OR [[%s]] IS NULL THEN json_array() ELSE json_array([[%s]]) END) END)`,
		column, column, column, column, column, column,
	)
}

// --- Date expressions ---

func (d *SQLiteDialect) DateTruncHour(column string) string {
	return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00:00', %s)", column)
}

// --- Error detection ---

func (d *SQLiteDialect) IsLockError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "table is locked")
}

// --- Migration helpers ---

func (d *SQLiteDialect) InitCollectionsSQL() string {
	return `
		CREATE TABLE {{_collections}} (
			[[id]]         TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
			[[system]]     BOOLEAN DEFAULT FALSE NOT NULL,
			[[type]]       TEXT DEFAULT "base" NOT NULL,
			[[name]]       TEXT UNIQUE NOT NULL,
			[[fields]]     JSON DEFAULT "[]" NOT NULL,
			[[indexes]]    JSON DEFAULT "[]" NOT NULL,
			[[listRule]]   TEXT DEFAULT NULL,
			[[viewRule]]   TEXT DEFAULT NULL,
			[[createRule]] TEXT DEFAULT NULL,
			[[updateRule]] TEXT DEFAULT NULL,
			[[deleteRule]] TEXT DEFAULT NULL,
			[[options]]    JSON DEFAULT "{}" NOT NULL,
			[[created]]    TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
			[[updated]]    TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx__collections_type on {{_collections}} ([[type]]);
	`
}

func (d *SQLiteDialect) InitParamsSQL() string {
	return `
		CREATE TABLE {{_params}} (
			[[id]]      TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
			[[value]]   JSON DEFAULT NULL,
			[[created]] TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
			[[updated]] TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
		);
	`
}

func (d *SQLiteDialect) InitLogsSQL() string {
	return `
		CREATE TABLE IF NOT EXISTS {{_logs}} (
			[[id]]      TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
			[[level]]   INTEGER DEFAULT 0 NOT NULL,
			[[message]] TEXT DEFAULT "" NOT NULL,
			[[data]]    JSON DEFAULT "{}" NOT NULL,
			[[created]] TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_logs_level on {{_logs}} ([[level]]);
		CREATE INDEX IF NOT EXISTS idx_logs_message on {{_logs}} ([[message]]);
		CREATE INDEX IF NOT EXISTS idx_logs_created_hour on {{_logs}} (strftime('%Y-%m-%d %H:00:00', [[created]]));
	`
}
