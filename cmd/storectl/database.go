package main

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/config"
)

var (
	errDatabaseConfiguration  = errors.New("PostgreSQL 连接配置无效")
	errDatabaseAuthentication = errors.New("PostgreSQL 密码认证失败（SQLSTATE 28P01）")
	errDatabaseConnection     = errors.New("PostgreSQL 连接或验证失败；未修改数据库")
)

type databaseConnection interface {
	Ping(context.Context) error
	Close(context.Context) error
}

type databaseConnector func(context.Context, *pgx.ConnConfig) (databaseConnection, error)

func loadDatabaseConfig(root string) (*pgx.ConnConfig, error) {
	var document struct {
		Database struct {
			Driver       string `json:"driver"`
			DSN          string `json:"dsn"`
			PasswordFile string `json:"password_file"`
		} `json:"database"`
	}
	if err := readJSON(filepath.Join(root, "config.yml"), &document); err != nil {
		return nil, errDatabaseConfiguration
	}
	if document.Database.Driver != "postgres" && document.Database.Driver != "postgresql" {
		return nil, errDatabaseConfiguration
	}
	database := config.DatabaseConfig{
		Driver:       document.Database.Driver,
		DSN:          document.Database.DSN,
		PasswordFile: document.Database.PasswordFile,
	}
	dsn, err := database.ResolveDSN()
	if err != nil {
		return nil, errDatabaseConfiguration
	}
	connectionConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errDatabaseConfiguration
	}
	return connectionConfig, nil
}

func connectPostgres(ctx context.Context, connectionConfig *pgx.ConnConfig) (databaseConnection, error) {
	return pgx.ConnectConfig(ctx, connectionConfig)
}

// checkDatabase is read-only and uses the same password-file resolution as the
// server. Raw driver errors can contain DSNs, so only fixed classifications are
// returned to the command line.
func checkDatabase(root string, connect databaseConnector) error {
	connectionConfig, err := loadDatabaseConfig(root)
	if err != nil {
		return err
	}
	if connect == nil {
		return errDatabaseConnection
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := connect(ctx, connectionConfig)
	if err != nil {
		return classifyDatabaseError(err)
	}
	if connection == nil {
		return errDatabaseConnection
	}
	defer func() { _ = connection.Close(ctx) }()
	return classifyDatabaseError(connection.Ping(ctx))
}

func classifyDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "28P01" {
		return errDatabaseAuthentication
	}
	return errDatabaseConnection
}
