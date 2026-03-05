package core_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// -------------------------------------------------------------------
// SQLiteDialect tests
// -------------------------------------------------------------------

func TestSQLiteDialectName(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}
	if d.Name() != "sqlite" {
		t.Fatalf("Expected 'sqlite', got %q", d.Name())
	}
}

func TestSQLiteDialectPrimaryKeyColumnType(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}
	result := d.PrimaryKeyColumnType()

	if !strings.Contains(result, "TEXT PRIMARY KEY") {
		t.Fatalf("Expected TEXT PRIMARY KEY in result, got %q", result)
	}
	if !strings.Contains(result, "randomblob") {
		t.Fatalf("Expected randomblob in result, got %q", result)
	}
}

func TestSQLiteDialectJSONExtract(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	scenarios := []struct {
		column   string
		path     string
		expected []string // substrings that must appear
	}{
		{"col", "", []string{"json_valid", "JSON_EXTRACT", "[[col]]"}},
		{"col", "key", []string{"JSON_EXTRACT", "$.key", "$.pb.key"}},
		{"col", "[0]", []string{"JSON_EXTRACT", "$[0]", "$.pb[0]"}},
	}

	for _, s := range scenarios {
		t.Run(s.column+"_"+s.path, func(t *testing.T) {
			result := d.JSONExtract(s.column, s.path)
			for _, sub := range s.expected {
				if !strings.Contains(result, sub) {
					t.Errorf("Expected %q to contain %q", result, sub)
				}
			}
		})
	}
}

func TestSQLiteDialectJSONEach(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	result := d.JSONEach("myCol")
	if !strings.Contains(result, "json_each") {
		t.Fatalf("Expected json_each in result, got %q", result)
	}
	if !strings.Contains(result, "[[myCol]]") {
		t.Fatalf("Expected [[myCol]] in result, got %q", result)
	}
	if !strings.Contains(result, "json_valid") {
		t.Fatalf("Expected json_valid in result, got %q", result)
	}
}

func TestSQLiteDialectJSONArrayLength(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	result := d.JSONArrayLength("myCol")
	if !strings.Contains(result, "json_array_length") {
		t.Fatalf("Expected json_array_length in result, got %q", result)
	}
	if !strings.Contains(result, "[[myCol]]") {
		t.Fatalf("Expected [[myCol]] in result, got %q", result)
	}
}

func TestSQLiteDialectDateTruncHour(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	result := d.DateTruncHour("created")
	if !strings.Contains(result, "strftime") {
		t.Fatalf("Expected strftime in result, got %q", result)
	}
	if !strings.Contains(result, "created") {
		t.Fatalf("Expected 'created' in result, got %q", result)
	}
	if !strings.Contains(result, "%H:00:00") {
		t.Fatalf("Expected hour truncation format in result, got %q", result)
	}
}

func TestSQLiteDialectIsLockError(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	scenarios := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("some error"), false},
		{errors.New("database is locked"), true},
		{errors.New("table is locked"), true},
		{errors.New("something database is locked something"), true},
	}

	for _, s := range scenarios {
		name := "nil"
		if s.err != nil {
			name = s.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			result := d.IsLockError(s.err)
			if result != s.expected {
				t.Fatalf("Expected %v, got %v", s.expected, result)
			}
		})
	}
}

func TestSQLiteDialectInitCollectionsSQL(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	sql := d.InitCollectionsSQL()
	for _, sub := range []string{"_collections", "randomblob", "strftime", "CREATE INDEX"} {
		if !strings.Contains(sql, sub) {
			t.Errorf("Expected InitCollectionsSQL to contain %q", sub)
		}
	}
}

func TestSQLiteDialectInitParamsSQL(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	sql := d.InitParamsSQL()
	for _, sub := range []string{"_params", "randomblob", "strftime"} {
		if !strings.Contains(sql, sub) {
			t.Errorf("Expected InitParamsSQL to contain %q", sub)
		}
	}
}

func TestSQLiteDialectInitLogsSQL(t *testing.T) {
	t.Parallel()
	d := &core.SQLiteDialect{}

	sql := d.InitLogsSQL()
	for _, sub := range []string{"_logs", "randomblob", "strftime", "idx_logs_level"} {
		if !strings.Contains(sql, sub) {
			t.Errorf("Expected InitLogsSQL to contain %q", sub)
		}
	}
}

// -------------------------------------------------------------------
// MySQLDialect tests
// -------------------------------------------------------------------

func TestMySQLDialectName(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}
	if d.Name() != "mysql" {
		t.Fatalf("Expected 'mysql', got %q", d.Name())
	}
}

func TestMySQLDialectPrimaryKeyColumnType(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}
	result := d.PrimaryKeyColumnType()

	if !strings.Contains(result, "VARCHAR") {
		t.Fatalf("Expected VARCHAR in result, got %q", result)
	}
	if !strings.Contains(result, "PRIMARY KEY") {
		t.Fatalf("Expected PRIMARY KEY in result, got %q", result)
	}
	// MySQL should NOT contain randomblob (SQLite-specific)
	if strings.Contains(result, "randomblob") {
		t.Fatalf("MySQL PK type should not contain randomblob, got %q", result)
	}
}

func TestMySQLDialectJSONExtract(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	scenarios := []struct {
		column   string
		path     string
		expected []string // substrings that must appear
		absent   []string // substrings that must NOT appear
	}{
		{
			"col", "",
			[]string{"JSON_VALID", "JSON_EXTRACT", "[[col]]"},
			nil,
		},
		{
			"col", "key",
			[]string{"JSON_EXTRACT", "$.key", "$.pb.key"},
			nil,
		},
		{
			"col", "[0]",
			[]string{"JSON_EXTRACT", "$[0]", "$.pb[0]"},
			nil,
		},
	}

	for _, s := range scenarios {
		t.Run(s.column+"_"+s.path, func(t *testing.T) {
			result := d.JSONExtract(s.column, s.path)
			for _, sub := range s.expected {
				if !strings.Contains(result, sub) {
					t.Errorf("Expected %q to contain %q", result, sub)
				}
			}
			for _, sub := range s.absent {
				if strings.Contains(result, sub) {
					t.Errorf("Expected %q to NOT contain %q", result, sub)
				}
			}
		})
	}
}

func TestMySQLDialectJSONEach(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	result := d.JSONEach("myCol")

	// MySQL uses JSON_TABLE instead of json_each
	if !strings.Contains(result, "JSON_TABLE") {
		t.Fatalf("Expected JSON_TABLE in result, got %q", result)
	}
	if !strings.Contains(result, "[[myCol]]") {
		t.Fatalf("Expected [[myCol]] in result, got %q", result)
	}
	// Must produce a "value" column
	if !strings.Contains(result, "value") {
		t.Fatalf("Expected 'value' column in result, got %q", result)
	}
	if !strings.Contains(result, "COLUMNS") {
		t.Fatalf("Expected COLUMNS clause in result, got %q", result)
	}
	// Must NOT contain SQLite-specific json_each
	if strings.Contains(result, "json_each") {
		t.Fatalf("MySQL JSONEach should not contain 'json_each', got %q", result)
	}
}

func TestMySQLDialectJSONArrayLength(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	result := d.JSONArrayLength("myCol")

	// MySQL uses JSON_LENGTH instead of json_array_length
	if !strings.Contains(result, "JSON_LENGTH") {
		t.Fatalf("Expected JSON_LENGTH in result, got %q", result)
	}
	if !strings.Contains(result, "[[myCol]]") {
		t.Fatalf("Expected [[myCol]] in result, got %q", result)
	}
	// Must NOT contain SQLite-specific json_array_length
	if strings.Contains(result, "json_array_length") {
		t.Fatalf("MySQL should not contain 'json_array_length', got %q", result)
	}
}

func TestMySQLDialectDateTruncHour(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	result := d.DateTruncHour("created")
	if !strings.Contains(result, "DATE_FORMAT") {
		t.Fatalf("Expected DATE_FORMAT in result, got %q", result)
	}
	if !strings.Contains(result, "created") {
		t.Fatalf("Expected 'created' in result, got %q", result)
	}
	if !strings.Contains(result, "%H:00:00") {
		t.Fatalf("Expected hour truncation format in result, got %q", result)
	}
	// Must NOT contain SQLite-specific strftime
	if strings.Contains(result, "strftime") {
		t.Fatalf("MySQL DateTruncHour should not contain 'strftime', got %q", result)
	}
}

func TestMySQLDialectIsLockError(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	scenarios := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("some error"), false},
		{errors.New("database is locked"), false}, // SQLite error, not MySQL
		{errors.New("Error 1205: Lock wait timeout exceeded"), true},
		{errors.New("Error 1213: Deadlock found"), true},
		{errors.New("Lock wait timeout exceeded; try restarting transaction"), true},
		{errors.New("Deadlock found when trying to get lock"), true},
	}

	for _, s := range scenarios {
		name := "nil"
		if s.err != nil {
			name = s.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			result := d.IsLockError(s.err)
			if result != s.expected {
				t.Fatalf("Expected %v, got %v for error %q", s.expected, result, name)
			}
		})
	}
}

func TestMySQLDialectInitCollectionsSQL(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	sql := d.InitCollectionsSQL()

	expected := []string{"_collections", "VARCHAR", "InnoDB", "utf8mb4", "CURRENT_TIMESTAMP", "CREATE INDEX"}
	for _, sub := range expected {
		if !strings.Contains(sql, sub) {
			t.Errorf("Expected InitCollectionsSQL to contain %q", sub)
		}
	}

	// Must NOT contain SQLite-specific syntax
	absent := []string{"randomblob", "strftime"}
	for _, sub := range absent {
		if strings.Contains(sql, sub) {
			t.Errorf("Expected InitCollectionsSQL to NOT contain %q", sub)
		}
	}
}

func TestMySQLDialectInitParamsSQL(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	sql := d.InitParamsSQL()

	expected := []string{"_params", "VARCHAR", "InnoDB", "CURRENT_TIMESTAMP"}
	for _, sub := range expected {
		if !strings.Contains(sql, sub) {
			t.Errorf("Expected InitParamsSQL to contain %q", sub)
		}
	}

	absent := []string{"randomblob", "strftime"}
	for _, sub := range absent {
		if strings.Contains(sql, sub) {
			t.Errorf("Expected InitParamsSQL to NOT contain %q", sub)
		}
	}
}

func TestMySQLDialectInitLogsSQL(t *testing.T) {
	t.Parallel()
	d := &core.MySQLDialect{}

	sql := d.InitLogsSQL()

	expected := []string{"_logs", "VARCHAR", "InnoDB", "CURRENT_TIMESTAMP", "idx_logs_level"}
	for _, sub := range expected {
		if !strings.Contains(sql, sub) {
			t.Errorf("Expected InitLogsSQL to contain %q", sub)
		}
	}

	absent := []string{"randomblob", "strftime"}
	for _, sub := range absent {
		if strings.Contains(sql, sub) {
			t.Errorf("Expected InitLogsSQL to NOT contain %q", sub)
		}
	}
}

// -------------------------------------------------------------------
// Cross-dialect consistency tests
// -------------------------------------------------------------------

func TestDialectInterfaceCompleteness(t *testing.T) {
	t.Parallel()

	// Verify both dialects implement the interface (compile-time check is in the files,
	// but this confirms they're usable at runtime)
	dialects := []core.DBDialect{
		&core.SQLiteDialect{},
		&core.MySQLDialect{},
	}

	for _, d := range dialects {
		t.Run(d.Name(), func(t *testing.T) {
			if d.Name() == "" {
				t.Fatal("Name() should not be empty")
			}
			if d.PrimaryKeyColumnType() == "" {
				t.Fatal("PrimaryKeyColumnType() should not be empty")
			}
			if d.JSONExtract("col", "path") == "" {
				t.Fatal("JSONExtract() should not be empty")
			}
			if d.JSONEach("col") == "" {
				t.Fatal("JSONEach() should not be empty")
			}
			if d.JSONArrayLength("col") == "" {
				t.Fatal("JSONArrayLength() should not be empty")
			}
			if d.DateTruncHour("col") == "" {
				t.Fatal("DateTruncHour() should not be empty")
			}
			if d.InitCollectionsSQL() == "" {
				t.Fatal("InitCollectionsSQL() should not be empty")
			}
			if d.InitParamsSQL() == "" {
				t.Fatal("InitParamsSQL() should not be empty")
			}
			if d.InitLogsSQL() == "" {
				t.Fatal("InitLogsSQL() should not be empty")
			}
		})
	}
}

func TestDialectsProduceDifferentSQL(t *testing.T) {
	t.Parallel()

	sqlite := &core.SQLiteDialect{}
	mysql := &core.MySQLDialect{}

	// These methods should produce different SQL for each dialect
	checks := []struct {
		name   string
		sqlite string
		mysql  string
	}{
		{"PrimaryKeyColumnType", sqlite.PrimaryKeyColumnType(), mysql.PrimaryKeyColumnType()},
		{"JSONEach", sqlite.JSONEach("col"), mysql.JSONEach("col")},
		{"JSONArrayLength", sqlite.JSONArrayLength("col"), mysql.JSONArrayLength("col")},
		{"DateTruncHour", sqlite.DateTruncHour("col"), mysql.DateTruncHour("col")},
		{"InitCollectionsSQL", sqlite.InitCollectionsSQL(), mysql.InitCollectionsSQL()},
		{"InitParamsSQL", sqlite.InitParamsSQL(), mysql.InitParamsSQL()},
		{"InitLogsSQL", sqlite.InitLogsSQL(), mysql.InitLogsSQL()},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if c.sqlite == c.mysql {
				t.Errorf("Expected different SQL for %s between SQLite and MySQL, but both produced:\n%s", c.name, c.sqlite)
			}
		})
	}
}

func TestDialectsLockErrorsAreMutuallyExclusive(t *testing.T) {
	t.Parallel()

	sqlite := &core.SQLiteDialect{}
	mysql := &core.MySQLDialect{}

	// SQLite-specific lock errors should NOT trigger MySQL lock detection
	sqliteErrors := []error{
		errors.New("database is locked"),
		errors.New("table is locked"),
	}
	for _, err := range sqliteErrors {
		if mysql.IsLockError(err) {
			t.Errorf("MySQL should not detect SQLite error %q as a lock error", err)
		}
		if !sqlite.IsLockError(err) {
			t.Errorf("SQLite should detect %q as a lock error", err)
		}
	}

	// MySQL-specific lock errors should NOT trigger SQLite lock detection
	mysqlErrors := []error{
		errors.New("Error 1205: Lock wait timeout exceeded"),
		errors.New("Error 1213: Deadlock found"),
	}
	for _, err := range mysqlErrors {
		if sqlite.IsLockError(err) {
			t.Errorf("SQLite should not detect MySQL error %q as a lock error", err)
		}
		if !mysql.IsLockError(err) {
			t.Errorf("MySQL should detect %q as a lock error", err)
		}
	}
}
