package boot

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/roidmc/quotagate/internal/config"
	"github.com/roidmc/kex-utils/pkg/kexdatabase"
)

// embeddedInstance holds the running embedded PostgreSQL so StopEmbeddedDB can
// shut it down on server exit. Nil unless database.embedded=true.
var embeddedInstance *kexdatabase.Instance

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	if cfg.Database.Embedded {
		return startEmbeddedDB(cfg)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	pgCfg := postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}

	db, err := gorm.Open(postgres.New(pgCfg), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	return db, nil
}

// startEmbeddedDB launches a bundled PostgreSQL under cfg.Database.DataDir and
// connects to its default database. The existing AutoMigrate flow (InitRepos)
// then builds the schema on top, so no separate migration step is needed.
func startEmbeddedDB(cfg *config.Config) (*gorm.DB, error) {
	dataDir := cfg.Database.DataDir
	if dataDir == "" {
		dataDir = "./data/database"
	}

	username := cfg.Database.Username
	if username == "" {
		username = kexdatabase.DefaultUsername
	}
	password := cfg.Database.Password
	if password == "" {
		password = kexdatabase.DefaultPassword
	}
	dbName := cfg.Database.Database
	if dbName == "" {
		dbName = kexdatabase.DefaultDatabase
	}

	inst, err := kexdatabase.Start(kexdatabase.Options{
		Username:     username,
		Password:     password,
		Database:     dbName,
		Port:         uint32(cfg.Database.Port), // 0 => auto free port
		BinariesPath: filepath.Join(dataDir, "bin"),
		DataPath:     filepath.Join(dataDir, "data"),
		RuntimePath:  filepath.Join(dataDir, "runtime"),
	})
	if err != nil {
		return nil, fmt.Errorf("start embedded database: %w", err)
	}
	embeddedInstance = inst

	slog.Info("quotagate/boot: started embedded postgres",
		"data_dir", dataDir, "port", inst.Port(), "database", dbName)

	db, err := inst.DefaultDB()
	if err != nil {
		_ = inst.Stop()
		embeddedInstance = nil
		return nil, err
	}
	return db, nil
}

// StopEmbeddedDB shuts down the embedded PostgreSQL if one was started. Safe to
// call when database.embedded=false (it is a no-op).
func StopEmbeddedDB() {
	if embeddedInstance != nil {
		if err := embeddedInstance.Stop(); err != nil {
			slog.Warn("quotagate/boot: embedded postgres stop error", "error", err)
		}
		embeddedInstance = nil
	}
}
