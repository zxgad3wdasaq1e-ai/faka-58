package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func databaseCheckFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	passwordFile := filepath.Join(root, "private-password")
	if err := os.WriteFile(passwordFile, []byte(" private 'password' \\value \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	document := map[string]interface{}{"database": map[string]string{
		"driver": "postgres", "dsn": "host=remote-db.example port=15432 user=app dbname=shop sslmode=disable", "password_file": passwordFile,
	}}
	encoded, err := json.Marshal(document)
	if err != nil || os.WriteFile(filepath.Join(root, "config.yml"), encoded, 0o600) != nil {
		t.Fatal("cannot create isolated database-check configuration")
	}
	return root
}

type databaseConnectionStub struct {
	pingError      error
	pinged, closed bool
}

func (s *databaseConnectionStub) Ping(ctx context.Context) error {
	s.pinged = true
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing operation deadline")
	}
	return s.pingError
}

func (s *databaseConnectionStub) Close(context.Context) error {
	s.closed = true
	return nil
}

func TestDatabaseCheckUsesSharedSecretAndBoundedReadOnlyConnection(t *testing.T) {
	root := databaseCheckFixture(t)
	connection := &databaseConnectionStub{}
	var stdout, stderr bytes.Buffer
	exit := runCLIWithDatabaseConnector([]string{"db-check", "--root", root}, &stdout, &stderr, func(ctx context.Context, parsed *pgx.ConnConfig) (databaseConnection, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 10*time.Second || time.Until(deadline) < 9*time.Second {
			t.Fatal("database check must share a ten-second connect and Ping deadline")
		}
		if parsed.Host != "remote-db.example" || parsed.Port != 15432 || parsed.Password != " private 'password' \\value " {
			t.Fatal("db-check changed the configured destination or mounted password")
		}
		return connection, nil
	})
	if exit != 0 || stdout.String() != "PostgreSQL 凭据验证成功。\n" || stderr.Len() != 0 || !connection.pinged || !connection.closed {
		t.Fatal("successful check must Ping, close and report only its fixed result")
	}
}

func TestDatabaseCheckClassifiesErrorsWithoutDisclosingDriverDetails(t *testing.T) {
	root := databaseCheckFixture(t)
	const sensitive = "private-password-and-DSN"
	for _, test := range []struct {
		name                    string
		connectError, pingError error
		exit                    int
	}{
		{"connect authentication", fmt.Errorf("%s: %w", sensitive, &pgconn.PgError{Code: "28P01", Message: sensitive}), nil, 3},
		{"Ping authentication", nil, &pgconn.PgError{Code: "28P01", Message: sensitive}, 3},
		{"permission failure", &pgconn.PgError{Code: "42501", Message: sensitive}, nil, 1},
		{"connection error", errors.New(sensitive), nil, 1},
		{"Ping timeout", nil, context.DeadlineExceeded, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := runCLIWithDatabaseConnector([]string{"db-check", "--root", root}, &stdout, &stderr, func(context.Context, *pgx.ConnConfig) (databaseConnection, error) {
				if test.connectError != nil {
					return nil, test.connectError
				}
				return &databaseConnectionStub{pingError: test.pingError}, nil
			})
			if exit != test.exit || stdout.Len() != 0 {
				t.Fatalf("exit = %d, want %d", exit, test.exit)
			}
			for _, private := range []string{sensitive, root, "remote-db.example", "password"} {
				if strings.Contains(stderr.String(), private) {
					t.Fatal("database-check error disclosed private connection details")
				}
			}
			if (test.exit == 3) != strings.Contains(stderr.String(), "28P01") {
				t.Fatal("only password authentication failures must use the dedicated error classification")
			}
		})
	}
}

func TestDatabaseCheckRejectsInvalidConfigurationBeforeConnecting(t *testing.T) {
	root := databaseCheckFixture(t)
	if err := os.Remove(filepath.Join(root, "private-password")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := runCLIWithDatabaseConnector([]string{"db-check", "--root", root}, &stdout, &stderr, func(context.Context, *pgx.ConnConfig) (databaseConnection, error) {
		t.Fatal("missing password file must prevent connection")
		return nil, nil
	})
	if exit != 1 || stdout.Len() != 0 || stderr.String() != "操作未完成: PostgreSQL 连接配置无效\n" {
		t.Fatal("invalid check configuration must fail without private details")
	}
}
