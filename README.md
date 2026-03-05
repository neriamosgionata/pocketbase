<p align="center">
    <a href="https://pocketbase.io" target="_blank" rel="noopener">
        <img src="https://i.imgur.com/5qimnm5.png" alt="PocketBase - open source backend in 1 file" />
    </a>
</p>

<p align="center">
    <a href="https://github.com/pocketbase/pocketbase/actions/workflows/release.yaml" target="_blank" rel="noopener"><img src="https://github.com/pocketbase/pocketbase/actions/workflows/release.yaml/badge.svg" alt="build" /></a>
    <a href="https://github.com/pocketbase/pocketbase/releases" target="_blank" rel="noopener"><img src="https://img.shields.io/github/release/pocketbase/pocketbase.svg" alt="Latest releases" /></a>
    <a href="https://pkg.go.dev/github.com/pocketbase/pocketbase" target="_blank" rel="noopener"><img src="https://godoc.org/github.com/pocketbase/pocketbase?status.svg" alt="Go package documentation" /></a>
</p>

[PocketBase](https://pocketbase.io) is an open source Go backend that includes:

- embedded database (_SQLite_) with **realtime subscriptions**
- **MySQL 8.0+ support** via the database dialect abstraction layer
- built-in **files and users management**
- convenient **Admin dashboard UI**
- and simple **REST-ish API**

**For documentation and examples, please visit https://pocketbase.io/docs.**

> [!WARNING]
> Please keep in mind that PocketBase is still under active development
> and therefore full backward compatibility is not guaranteed before reaching v1.0.0.

## API SDK clients

The easiest way to interact with the PocketBase Web APIs is to use one of the official SDK clients:

- **JavaScript - [pocketbase/js-sdk](https://github.com/pocketbase/js-sdk)** (_Browser, Node.js, React Native_)
- **Dart - [pocketbase/dart-sdk](https://github.com/pocketbase/dart-sdk)** (_Web, Mobile, Desktop, CLI_)

You could also check the recommendations in https://pocketbase.io/docs/how-to-use/.


## Overview

### Use as standalone app

You could download the prebuilt executable for your platform from the [Releases page](https://github.com/pocketbase/pocketbase/releases).
Once downloaded, extract the archive and run `./pocketbase serve` in the extracted directory.

The prebuilt executables are based on the [`examples/base/main.go` file](https://github.com/pocketbase/pocketbase/blob/master/examples/base/main.go) and comes with the JS VM plugin enabled by default which allows to extend PocketBase with JavaScript (_for more details please refer to [Extend with JavaScript](https://pocketbase.io/docs/js-overview/)_).

### Use as a Go framework/toolkit

PocketBase is distributed as a regular Go library package which allows you to build
your own custom app specific business logic and still have a single portable executable at the end.

Here is a minimal example:

0. [Install Go 1.23+](https://go.dev/doc/install) (_if you haven't already_)

1. Create a new project directory with the following `main.go` file inside it:
    ```go
    package main

    import (
        "log"

        "github.com/pocketbase/pocketbase"
        "github.com/pocketbase/pocketbase/core"
    )

    func main() {
        app := pocketbase.New()

        app.OnServe().BindFunc(func(se *core.ServeEvent) error {
            // registers new "GET /hello" route
            se.Router.GET("/hello", func(re *core.RequestEvent) error {
                return re.String(200, "Hello world!")
            })

            return se.Next()
        })

        if err := app.Start(); err != nil {
            log.Fatal(err)
        }
    }
    ```

2. To init the dependencies, run `go mod init myapp && go mod tidy`.

3. To start the application, run `go run main.go serve`.

4. To build a statically linked executable, you can run `CGO_ENABLED=0 go build` and then start the created executable with `./myapp serve`.

_For more details please refer to [Extend with Go](https://pocketbase.io/docs/go-overview/)._

### Building and running the repo main.go example

To build the minimal standalone executable, like the prebuilt ones in the releases page, you can simply run `go build` inside the `examples/base` directory:

0. [Install Go 1.24+](https://go.dev/doc/install) (_if you haven't already_)
1. Clone/download the repo
2. Navigate to `examples/base`
3. Run `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build`
   (_https://go.dev/doc/install/source#environment_)
4. Start the created executable by running `./base serve`.

Note that the supported build targets by the pure Go SQLite driver at the moment are:

```
darwin  amd64
darwin  arm64
freebsd amd64
freebsd arm64
linux   386
linux   amd64
linux   arm
linux   arm64
linux   loong64
linux   ppc64le
linux   riscv64
linux   s390x
windows 386
windows amd64
windows arm64
```

### Using MySQL instead of SQLite

PocketBase ships with SQLite as the default database, but includes a **database dialect abstraction layer** that enables MySQL 8.0+ support.

To use MySQL, provide a `DBDialect` and a custom `DBConnect` function in your configuration:

```go
package main

import (
    "log"

    _ "github.com/go-sql-driver/mysql"
    "github.com/pocketbase/dbx"
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/core"
)

func main() {
    app := pocketbase.NewWithConfig(pocketbase.Config{
        DBDialect: &core.MySQLDialect{},
        DBConnect: func(dsn string) (*dbx.DB, error) {
            // dsn receives the default file path (e.g. "pb_data/data.db");
            // for MySQL, ignore it and use your own connection string.
            return dbx.Open("mysql", "user:pass@tcp(localhost:3306)/pocketbase?parseTime=true")
        },
        DataDir: "pb_data", // still used for file storage
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

The dialect abstraction covers:

- **Schema introspection** -- `INFORMATION_SCHEMA` queries instead of SQLite PRAGMAs
- **JSON functions** -- `JSON_TABLE` / `JSON_LENGTH` instead of `json_each` / `json_array_length`
- **Date functions** -- `DATE_FORMAT` instead of `strftime`
- **Column types** -- `VARCHAR(15) PRIMARY KEY` instead of `TEXT PRIMARY KEY DEFAULT (randomblob(...))`
- **Lock/error detection** -- MySQL error codes 1205/1213 instead of "database is locked"
- **DDL migrations** -- InnoDB tables with `utf8mb4` charset and `CURRENT_TIMESTAMP(3)` defaults
- **Maintenance** -- `ANALYZE TABLE` instead of `PRAGMA wal_checkpoint` / `PRAGMA optimize`

#### Database dialect interface

The `core.DBDialect` interface can also be implemented for other database backends. The following methods must be provided:

| Method | Purpose |
|--------|---------|
| `Name()` | Dialect identifier (e.g. `"sqlite"`, `"mysql"`) |
| `Connect(dsn)` | Open a new DB connection |
| `TableColumns(db, table)` | List column names |
| `TableInfo(db, table)` | Column metadata |
| `TableIndexes(db, table)` | Index name-to-SQL map |
| `HasTable(db, table)` | Check table/view existence |
| `Vacuum(db)` | Reclaim disk space |
| `OptimizeAfterDDL(db, logger)` | Post-DDL optimization |
| `PeriodicOptimize(db, auxDB, logger)` | Cron-scheduled maintenance |
| `PrimaryKeyColumnType()` | PK column definition |
| `JSONExtract(column, path)` | JSON value extraction expression |
| `JSONEach(column)` | JSON array iteration table expression |
| `JSONArrayLength(column)` | JSON array length expression |
| `DateTruncHour(column)` | Truncate datetime to hour |
| `IsLockError(err)` | Detect lock/busy errors |
| `InitCollectionsSQL()` | DDL for `_collections` table |
| `InitParamsSQL()` | DDL for `_params` table |
| `InitLogsSQL()` | DDL for `_logs` table |

#### Known Limitations / Future Work

- **`strftime` filter function** -- The `strftime()` function exposed in PocketBase's API filter syntax (e.g. `?filter=strftime('%Y',created)='2024'`) currently generates SQLite-specific SQL. The `tools/search` package does not yet have access to the dialect. MySQL users should avoid using `strftime` in API filters until this is addressed.

- **Backup and restore** -- The backup mechanism (`CreateBackup` / `RestoreBackup`) works by archiving the `pb_data` directory, which contains the SQLite database files. When using MySQL, the database files won't be in `pb_data`, so backups will only include file storage. A MySQL-compatible backup strategy (e.g. `mysqldump`) would need to be implemented separately.

- **`dbutils` package** -- The original `JSONEach` / `JSONExtract` / `JSONArrayLength` helper functions in `tools/dbutils` remain available for backward compatibility but always produce SQLite syntax. Internal callers have been migrated to the dialect methods. External consumers should transition to `app.DBDialect().JSONEach(...)` etc.

- **Dual-database architecture** -- PocketBase uses two separate databases (`data.db` for main data, `auxiliary.db` for logs). When using MySQL, the `DBConnect` function receives both paths and should map them to appropriate MySQL databases or schemas. A common approach is to use a single MySQL database for both.

- **`no_default_driver` build tag** -- The existing `no_default_driver` build tag disables the default SQLite driver import. When using MySQL, you must provide both a custom `DBConnect` function and set `DBDialect` to `&core.MySQLDialect{}`. The build tag is not required when using MySQL but can be used to reduce binary size by excluding the SQLite driver.

- **`geoDistance` filter function** -- The `geoDistance()` function in API filters uses `radians()`, `acos()`, `cos()`, and `sin()` SQL functions. These are available natively in MySQL but require the SQLite math extension. This function should work in both dialects without changes.

### Testing

PocketBase comes with mixed bag of unit and integration tests.
To run them, use the standard `go test` command:

```sh
go test ./...
```

Check also the [Testing guide](http://pocketbase.io/docs/testing) to learn how to write your own custom application tests.

## Security

If you discover a security vulnerability within PocketBase, please send an e-mail to **support at pocketbase.io**.

All reports will be promptly addressed and you'll be credited in the fix release notes.

## Contributing

PocketBase is free and open source project licensed under the [MIT License](LICENSE.md).
You are free to do whatever you want with it, even offering it as a paid service.

You could help continuing its development by:

- [Contribute to the source code](CONTRIBUTING.md)
- [Suggest new features and report issues](https://github.com/pocketbase/pocketbase/issues)

PRs for new OAuth2 providers, bug fixes, code optimizations and documentation improvements are more than welcome.

But please refrain creating PRs for _new features_ without previously discussing the implementation details.
PocketBase has a [roadmap](https://github.com/orgs/pocketbase/projects/2) and I try to work on issues in specific order and such PRs often come in out of nowhere and skew all initial planning with tedious back-and-forth communication.

Don't get upset if I close your PR, even if it is well executed and tested. This doesn't mean that it will never be merged.
Later we can always refer to it and/or take pieces of your implementation when the time comes to work on the issue (don't worry you'll be credited in the release notes).
