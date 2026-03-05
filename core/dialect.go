package core

import (
	"log/slog"

	"github.com/pocketbase/dbx"
)

// DBDialect defines an interface for database-specific operations,
// allowing PocketBase to support multiple database backends (SQLite, MySQL, etc.).
type DBDialect interface {
	// Name returns the dialect identifier (e.g. "sqlite", "mysql").
	Name() string

	// Connect opens a new database connection using the provided DSN/path.
	Connect(dsn string) (*dbx.DB, error)

	// --- Schema introspection ---

	// TableColumns returns all column names of the specified table.
	TableColumns(db dbx.Builder, tableName string) ([]string, error)

	// TableInfo returns detailed column metadata for the specified table.
	TableInfo(db dbx.Builder, tableName string) ([]*TableInfoRow, error)

	// TableIndexes returns a name-to-SQL map of all non-empty indexes for the specified table.
	TableIndexes(db dbx.Builder, tableName string) (map[string]string, error)

	// HasTable checks if a table or view with the provided name exists.
	HasTable(db dbx.Builder, tableName string) bool

	// Vacuum reclaims unused disk space for the database.
	Vacuum(db dbx.Builder) error

	// OptimizeAfterDDL runs dialect-specific optimization after DDL changes (e.g. table schema sync).
	// This is a best-effort operation and errors should be logged but not fatal.
	OptimizeAfterDDL(db dbx.Builder, logger *slog.Logger)

	// --- Maintenance ---

	// PeriodicOptimize runs periodic database maintenance/optimization.
	// This is called by the cron scheduler.
	PeriodicOptimize(nonconcurrentDB dbx.Builder, auxNonconcurrentDB dbx.Builder, logger *slog.Logger)

	// --- Column types ---

	// PrimaryKeyColumnType returns the column type definition for the
	// auto-generated primary key field.
	PrimaryKeyColumnType() string

	// --- SQL expressions ---

	// JSONExtract returns a SQL expression that extracts a value from a JSON column at the given path.
	JSONExtract(column string, path string) string

	// JSONEach returns a SQL table-valued expression for iterating over a JSON array column.
	// The result should be usable in a FROM/JOIN clause and produce a "value" column.
	JSONEach(column string) string

	// JSONArrayLength returns a SQL expression that returns the length of a JSON array column.
	JSONArrayLength(column string) string

	// --- Date expressions ---

	// DateTruncHour returns a SQL expression that truncates a datetime column to the hour.
	// Used for grouping log entries by hour.
	DateTruncHour(column string) string

	// --- Error detection ---

	// IsLockError reports whether the given error represents a database lock/busy error.
	IsLockError(err error) bool

	// --- Migration helpers ---

	// InitCollectionsSQL returns the SQL to create the _collections system table.
	InitCollectionsSQL() string

	// InitParamsSQL returns the SQL to create the _params system table.
	InitParamsSQL() string

	// InitLogsSQL returns the SQL to create the _logs system table (auxiliary DB).
	InitLogsSQL() string
}
