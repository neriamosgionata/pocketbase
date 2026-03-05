package core

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/pocketbase/dbx"
)

var _ DBDialect = (*MySQLDialect)(nil)

// MySQLDialect implements DBDialect for MySQL 8.0+ databases.
//
// Note: requires the MySQL driver to be imported separately
// (e.g. _ "github.com/go-sql-driver/mysql").
type MySQLDialect struct{}

func (d *MySQLDialect) Name() string {
	return "mysql"
}

func (d *MySQLDialect) Connect(dsn string) (*dbx.DB, error) {
	db, err := dbx.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// --- Schema introspection ---

func (d *MySQLDialect) TableColumns(db dbx.Builder, tableName string) ([]string, error) {
	columns := []string{}

	err := db.NewQuery(
		"SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = {:tableName} ORDER BY ORDINAL_POSITION",
	).Bind(dbx.Params{"tableName": tableName}).
		Column(&columns)

	return columns, err
}

func (d *MySQLDialect) TableInfo(db dbx.Builder, tableName string) ([]*TableInfoRow, error) {
	type mysqlColumnInfo struct {
		OrdinalPosition int    `db:"ORDINAL_POSITION"`
		ColumnName      string `db:"COLUMN_NAME"`
		DataType        string `db:"DATA_TYPE"`
		IsNullable      string `db:"IS_NULLABLE"`
		ColumnDefault   *string `db:"COLUMN_DEFAULT"`
		ColumnKey       string `db:"COLUMN_KEY"`
	}

	rawInfo := []*mysqlColumnInfo{}
	err := db.NewQuery(
		"SELECT ORDINAL_POSITION, COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_KEY FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = {:tableName} ORDER BY ORDINAL_POSITION",
	).Bind(dbx.Params{"tableName": tableName}).
		All(&rawInfo)
	if err != nil {
		return nil, err
	}

	if len(rawInfo) == 0 {
		return nil, fmt.Errorf("empty table info probably due to invalid or missing table %s", tableName)
	}

	info := make([]*TableInfoRow, len(rawInfo))
	for i, col := range rawInfo {
		row := &TableInfoRow{
			Index:   col.OrdinalPosition - 1, // 0-based like SQLite
			Name:    col.ColumnName,
			Type:    strings.ToUpper(col.DataType),
			NotNull: col.IsNullable == "NO",
		}
		if col.ColumnDefault != nil {
			row.DefaultValue.Valid = true
			row.DefaultValue.String = *col.ColumnDefault
		}
		if col.ColumnKey == "PRI" {
			row.PK = 1
		}
		info[i] = row
	}

	return info, nil
}

func (d *MySQLDialect) TableIndexes(db dbx.Builder, tableName string) (map[string]string, error) {
	type mysqlIndex struct {
		IndexName  string `db:"INDEX_NAME"`
		NonUnique  int    `db:"NON_UNIQUE"`
		ColumnName string `db:"COLUMN_NAME"`
		SeqInIndex int    `db:"SEQ_IN_INDEX"`
	}

	rawIndexes := []*mysqlIndex{}
	err := db.NewQuery(
		"SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME, SEQ_IN_INDEX FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = {:tableName} AND INDEX_NAME != 'PRIMARY' ORDER BY INDEX_NAME, SEQ_IN_INDEX",
	).Bind(dbx.Params{"tableName": tableName}).
		All(&rawIndexes)
	if err != nil {
		return nil, err
	}

	// group columns by index name
	grouped := make(map[string]*struct {
		columns  []string
		isUnique bool
	})
	for _, idx := range rawIndexes {
		g, ok := grouped[idx.IndexName]
		if !ok {
			g = &struct {
				columns  []string
				isUnique bool
			}{
				isUnique: idx.NonUnique == 0,
			}
			grouped[idx.IndexName] = g
		}
		g.columns = append(g.columns, idx.ColumnName)
	}

	// reconstruct CREATE INDEX statements to match SQLite format
	result := make(map[string]string, len(grouped))
	for name, g := range grouped {
		unique := ""
		if g.isUnique {
			unique = "UNIQUE "
		}
		quotedCols := make([]string, len(g.columns))
		for i, c := range g.columns {
			quotedCols[i] = "`" + c + "`"
		}
		result[name] = fmt.Sprintf(
			"CREATE %sINDEX `%s` ON `%s` (%s)",
			unique, name, tableName, strings.Join(quotedCols, ", "),
		)
	}

	return result, nil
}

func (d *MySQLDialect) HasTable(db dbx.Builder, tableName string) bool {
	var exists int

	err := db.NewQuery(
		"SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND LOWER(TABLE_NAME) = LOWER({:tableName}) LIMIT 1",
	).Bind(dbx.Params{"tableName": tableName}).
		Row(&exists)

	return err == nil && exists > 0
}

func (d *MySQLDialect) Vacuum(db dbx.Builder) error {
	// MySQL doesn't have a direct VACUUM equivalent.
	// OPTIMIZE TABLE requires a specific table name,
	// so this is a no-op. Individual tables can be optimized separately.
	return nil
}

func (d *MySQLDialect) OptimizeAfterDDL(db dbx.Builder, logger *slog.Logger) {
	// MySQL doesn't need explicit optimization after DDL changes.
	// The InnoDB engine handles this automatically.
}

// --- Maintenance ---

func (d *MySQLDialect) PeriodicOptimize(nonconcurrentDB dbx.Builder, auxNonconcurrentDB dbx.Builder, logger *slog.Logger) {
	_, execErr := nonconcurrentDB.NewQuery("ANALYZE TABLE _collections, _params").Execute()
	if execErr != nil {
		logger.Warn("Failed to run periodic ANALYZE TABLE for the main DB", slog.String("error", execErr.Error()))
	}

	_, execErr = auxNonconcurrentDB.NewQuery("ANALYZE TABLE _logs").Execute()
	if execErr != nil {
		logger.Warn("Failed to run periodic ANALYZE TABLE for the auxiliary DB", slog.String("error", execErr.Error()))
	}
}

// --- Column types ---

func (d *MySQLDialect) PrimaryKeyColumnType() string {
	return "VARCHAR(15) PRIMARY KEY NOT NULL"
}

// --- SQL expressions ---

func (d *MySQLDialect) JSONExtract(column string, path string) string {
	if path != "" && !strings.HasPrefix(path, "[") {
		path = "." + path
	}

	return fmt.Sprintf(
		"(CASE WHEN JSON_VALID([[%s]]) THEN JSON_EXTRACT([[%s]], '$%s') ELSE JSON_EXTRACT(JSON_OBJECT('pb', [[%s]]), '$.pb%s') END)",
		column,
		column,
		path,
		column,
		path,
	)
}

func (d *MySQLDialect) JSONEach(column string) string {
	// MySQL 8.0+ JSON_TABLE equivalent of SQLite's json_each().
	// Produces a "value" column matching the SQLite json_each behavior.
	return fmt.Sprintf(
		`JSON_TABLE(CASE WHEN JSON_VALID([[%s]]) AND JSON_TYPE([[%s]]) = 'ARRAY' THEN [[%s]] ELSE JSON_ARRAY([[%s]]) END, '$[*]' COLUMNS(value TEXT PATH '$')) `,
		column, column, column, column,
	)
}

func (d *MySQLDialect) JSONArrayLength(column string) string {
	return fmt.Sprintf(
		`JSON_LENGTH(CASE WHEN JSON_VALID([[%s]]) AND JSON_TYPE([[%s]]) = 'ARRAY' THEN [[%s]] ELSE (CASE WHEN [[%s]] = '' OR [[%s]] IS NULL THEN JSON_ARRAY() ELSE JSON_ARRAY([[%s]]) END) END)`,
		column, column, column, column, column, column,
	)
}

// --- Date expressions ---

func (d *MySQLDialect) DateTruncHour(column string) string {
	return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:00:00')", column)
}

// --- Error detection ---

func (d *MySQLDialect) IsLockError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// MySQL error 1205: Lock wait timeout exceeded
	// MySQL error 1213: Deadlock found
	return strings.Contains(errStr, "Error 1205") ||
		strings.Contains(errStr, "Error 1213") ||
		strings.Contains(errStr, "Lock wait timeout exceeded") ||
		strings.Contains(errStr, "Deadlock found")
}

// --- Migration helpers ---

func (d *MySQLDialect) InitCollectionsSQL() string {
	return `
		CREATE TABLE IF NOT EXISTS _collections (
			id         VARCHAR(15) NOT NULL PRIMARY KEY,
			` + "`system`" + `     TINYINT(1) NOT NULL DEFAULT 0,
			` + "`type`" + `       VARCHAR(50) NOT NULL DEFAULT 'base',
			` + "`name`" + `       VARCHAR(255) NOT NULL,
			` + "`fields`" + `     JSON NOT NULL,
			` + "`indexes`" + `    JSON NOT NULL,
			listRule   TEXT DEFAULT NULL,
			viewRule   TEXT DEFAULT NULL,
			createRule TEXT DEFAULT NULL,
			updateRule TEXT DEFAULT NULL,
			deleteRule TEXT DEFAULT NULL,
			` + "`options`" + `    JSON NOT NULL,
			created    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_collections_name (` + "`name`" + `)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

		CREATE INDEX idx__collections_type ON _collections (` + "`type`" + `);
	`
}

func (d *MySQLDialect) InitParamsSQL() string {
	return `
		CREATE TABLE IF NOT EXISTS _params (
			id      VARCHAR(15) NOT NULL PRIMARY KEY,
			` + "`value`" + `   JSON DEFAULT NULL,
			created DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`
}

func (d *MySQLDialect) InitLogsSQL() string {
	return `
		CREATE TABLE IF NOT EXISTS _logs (
			id      VARCHAR(15) NOT NULL PRIMARY KEY,
			` + "`level`" + `   INT NOT NULL DEFAULT 0,
			message TEXT NOT NULL,
			` + "`data`" + `    JSON NOT NULL,
			created DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

		CREATE INDEX idx_logs_level ON _logs (` + "`level`" + `);
		CREATE INDEX idx_logs_created ON _logs (created);
	`
}
