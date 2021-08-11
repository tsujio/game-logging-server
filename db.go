package main

import (
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"golang.org/x/xerrors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DBConfig struct {
	Host     string
	Port     int
	User     string
	DBName   string
	Password string
}

func getDBURL(conf *DBConfig) string {
	var endpoint string
	if strings.HasPrefix(conf.Host, "/cloudsql/") {
		endpoint = fmt.Sprintf("unix(%s)", conf.Host)
	} else {
		endpoint = fmt.Sprintf("tcp(%s:%d)", conf.Host, conf.Port)
	}

	return fmt.Sprintf("%s:%s@%s/%s?charset=utf8mb4&parseTime=True&loc=Local",
		conf.User, conf.Password, endpoint, conf.DBName)
}

func SetupDB(conf *DBConfig, migrationsDir string) error {
	// Run migrations
	m, err := migrate.New("file://"+migrationsDir, "mysql://"+getDBURL(conf))
	if err != nil {
		return xerrors.Errorf("Failed to initiate migration: %w", err)
	}
	err = m.Up()
	if err != nil && !xerrors.Is(err, migrate.ErrNoChange) {
		return xerrors.Errorf("Failed to migrate db: %w", err)
	}

	// Check db connection
	db, err := OpenDB(conf)
	if err != nil {
		return xerrors.Errorf("Failed to get db connection: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return xerrors.Errorf("Failed to get sqlDB object: %w", err)
	}
	err = sqlDB.Close()
	if err != nil {
		return xerrors.Errorf("Failed to close db connection: %w", err)
	}

	return nil
}

func OpenDB(conf *DBConfig) (*gorm.DB, error) {
	// Open db
	db, err := gorm.Open(mysql.Open(getDBURL(conf)), &gorm.Config{})
	if err != nil {
		return nil, xerrors.Errorf("Failed to open database: %w", err)
	}

	return db, nil
}
