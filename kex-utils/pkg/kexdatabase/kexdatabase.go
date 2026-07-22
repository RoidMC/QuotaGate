// Package kexdatabase provides a self-contained embedded PostgreSQL instance,
// wrapping github.com/fergusstrange/embedded-postgres so the rest of the code
// (test suites AND the backend dev server) can start a real Postgres without a
// remote database.
//
// Two callers share this package:
//   - internal/testutil/testdb  -> spins up a fresh instance per `go test` run,
//     then opens one isolated database per test (mirrors SQLite :memory:).
//   - internal/boot             -> when database.embedded=true, starts a
//     persistent instance whose data lives under ./data/database.
//
// The PG binary is downloaded and cached on first Start(); subsequent starts
// reuse it. On a machine with no network the first Start() will fail — that is
// expected and the caller should surface a clear error.
package kexdatabase

import (
	"fmt"
	"net"
	"sync"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	DefaultUsername = "kexdatabase"
	DefaultPassword = "kexdatabase"
	DefaultDatabase = "kexdatabase"
)

// Options configures a Start call. Paths left empty fall back to sensible
// defaults: BinariesPath uses the library cache, DataPath/RuntimePath use a
// temp dir. Port 0 lets kexdatabase pick a free port.
type Options struct {
	Username     string
	Password     string
	Database     string // default database created at initdb time
	Port         uint32 // 0 => pick a free port
	BinariesPath string // optional; where the PG binary is cached/downloaded
	DataPath     string // PG data cluster dir
	RuntimePath  string // sockets / lock files dir
	Verbose      bool
}

// Instance is a running embedded PostgreSQL.
type Instance struct {
	pg      *embeddedpostgres.EmbeddedPostgres
	opts    Options
	adminDB *gorm.DB
	mu      sync.Mutex
	seq     int64
}

// Start launches a new embedded PostgreSQL and returns a handle to it. Call
// Stop when done. The instance connects as the superuser that owns the cluster.
func Start(opts Options) (*Instance, error) {
	if opts.Username == "" {
		opts.Username = DefaultUsername
	}
	if opts.Password == "" {
		opts.Password = DefaultPassword
	}
	if opts.Database == "" {
		opts.Database = DefaultDatabase
	}
	if opts.Port == 0 {
		opts.Port = uint32(freePort())
	}

	config := embeddedpostgres.DefaultConfig().
		Username(opts.Username).
		Password(opts.Password).
		Database(opts.Database).
		Port(opts.Port)
	if opts.BinariesPath != "" {
		config = config.BinariesPath(opts.BinariesPath)
	}
	if opts.DataPath != "" {
		config = config.DataPath(opts.DataPath)
	}
	if opts.RuntimePath != "" {
		config = config.RuntimePath(opts.RuntimePath)
	}

	pg := embeddedpostgres.NewDatabase(config)
	if err := pg.Start(); err != nil {
		return nil, fmt.Errorf("kexdatabase: start embedded postgres: %w", err)
	}

	adminDB, err := gorm.Open(postgres.Open(dsn(opts, opts.Database)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		_ = pg.Stop()
		return nil, fmt.Errorf("kexdatabase: open default db: %w", err)
	}

	return &Instance{pg: pg, opts: opts, adminDB: adminDB}, nil
}

// Port returns the actual TCP port the instance listens on (useful when Port
// was 0 and a free port was auto-selected).
func (i *Instance) Port() uint32 { return i.opts.Port }

// Opts returns the effective options the instance was started with (after
// defaults were applied). Useful for tests and diagnostics.
func (i *Instance) Opts() Options { return i.opts }

// DSN returns a gorm/SQL DSN for the given database name on this instance.
func (i *Instance) DSN(dbName string) string { return dsn(i.opts, dbName) }

// CreateDatabase creates a new database on the cluster. Idempotent callers
// should check existence beforehand; this returns the raw error.
func (i *Instance) CreateDatabase(name string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, name)).Error
}

// Open connects to the named database and returns a *gorm.DB.
func (i *Instance) Open(dbName string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(i.DSN(dbName)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("kexdatabase: open %s: %w", dbName, err)
	}
	return db, nil
}

// DefaultDB connects to the instance's default database (opts.Database).
func (i *Instance) DefaultDB() (*gorm.DB, error) { return i.Open(i.opts.Database) }

// Stop terminates the embedded PostgreSQL. Safe to call multiple times.
func (i *Instance) Stop() error {
	if i.pg == nil {
		return nil
	}
	err := i.pg.Stop()
	i.pg = nil
	return err
}

func dsn(opts Options, dbName string) string {
	return fmt.Sprintf("host=localhost port=%d user=%s password=%s dbname=%s sslmode=disable",
		opts.Port, opts.Username, opts.Password, dbName)
}

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 5433
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
