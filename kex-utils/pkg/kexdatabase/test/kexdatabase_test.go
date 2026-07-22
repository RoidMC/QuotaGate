// Package kexdatabase_test exercises the exported surface of
// github.com/.../core/pkg/kexdatabase with a real embedded PostgreSQL.
//
// These tests download the PG binary on first run (cached afterwards). On a
// machine with no network the first Start() fails — that is expected and the
// test will report it clearly rather than hang.
package kexdatabase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roidmc/kex-utils/pkg/kexdatabase"
)

// startInstance boots a fresh embedded PG and registers Stop on cleanup.
func startInstance(t *testing.T, opts kexdatabase.Options) *kexdatabase.Instance {
	t.Helper()
	inst, err := kexdatabase.Start(opts)
	require.NoError(t, err, "kexdatabase.Start should succeed (needs network for first-run PG download)")
	require.NotNil(t, inst)
	t.Cleanup(func() { _ = inst.Stop() })
	return inst
}

func TestStart_Defaults(t *testing.T) {
	inst := startInstance(t, kexdatabase.Options{})

	// Port 0 => a free port must have been auto-selected and is non-zero.
	assert.NotZero(t, inst.Port(), "auto-selected port should be non-zero")

	// Default credentials / database should be usable.
	db, err := inst.DefaultDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	var one int
	require.NoError(t, db.Raw("SELECT 1").Scan(&one).Error)
	assert.Equal(t, 1, one)
}

func TestStart_ExplicitPortAndCreds(t *testing.T) {
	inst := startInstance(t, kexdatabase.Options{
		Username: "alice",
		Password: "secret",
		Database: "appdb",
		Port:     0, // still let it pick free to avoid collisions
	})

	assert.Equal(t, "alice", inst.Opts().Username)
	assert.Equal(t, "appdb", inst.Opts().Database)

	db, err := inst.DefaultDB()
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS t (id int)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO t VALUES (42)`).Error)

	var n int
	require.NoError(t, db.Raw(`SELECT id FROM t LIMIT 1`).Scan(&n).Error)
	assert.Equal(t, 42, n)
}

func TestDSN(t *testing.T) {
	inst := startInstance(t, kexdatabase.Options{Port: 0})
	dsn := inst.DSN("some_db")
	assert.Contains(t, dsn, "dbname=some_db")
	assert.Contains(t, dsn, "sslmode=disable")
	assert.Contains(t, dsn, "user=kexdatabase")
}

func TestCreateDatabaseAndOpen(t *testing.T) {
	inst := startInstance(t, kexdatabase.Options{})

	const newDB = "tenant_one"
	require.NoError(t, inst.CreateDatabase(newDB))

	db, err := inst.Open(newDB)
	require.NoError(t, err)
	require.NotNil(t, db)

	require.NoError(t, db.Exec(`CREATE TABLE widgets (name text)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO widgets VALUES ('gizmo')`).Error)

	var name string
	require.NoError(t, db.Raw(`SELECT name FROM widgets LIMIT 1`).Scan(&name).Error)
	assert.Equal(t, "gizmo", name)

	// A second, independent database must not see the first one's tables.
	const otherDB = "tenant_two"
	require.NoError(t, inst.CreateDatabase(otherDB))
	other, err := inst.Open(otherDB)
	require.NoError(t, err)
	var name2 string
	err = other.Raw(`SELECT name FROM widgets`).Scan(&name2).Error
	require.Error(t, err, "cross-database table leakage must not happen")
}

func TestCreateDatabase_DuplicateIsError(t *testing.T) {
	inst := startInstance(t, kexdatabase.Options{})
	const dup = "dupdb"
	require.NoError(t, inst.CreateDatabase(dup))
	err := inst.CreateDatabase(dup)
	require.Error(t, err, "creating the same database twice should error")
}

func TestStop_Idempotent(t *testing.T) {
	inst, err := kexdatabase.Start(kexdatabase.Options{})
	require.NoError(t, err)

	require.NoError(t, inst.Stop())
	// Second Stop must be a safe no-op (pg already nil).
	require.NoError(t, inst.Stop())
}

func TestStop_ClosesConnections(t *testing.T) {
	inst := startInstance(t, kexdatabase.Options{})
	db, err := inst.DefaultDB()
	require.NoError(t, err)

	require.NoError(t, inst.Stop())

	// After Stop, a query on a connection opened earlier should fail.
	var one int
	qErr := db.Raw("SELECT 1").Scan(&one).Error
	require.Error(t, qErr, "query after Stop should fail")
}
