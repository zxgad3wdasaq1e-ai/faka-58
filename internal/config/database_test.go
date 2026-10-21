package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/viper"
)

func databasePasswordFixture(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private-password")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveDatabasePasswordFileUsesActualDriverParsing(t *testing.T) {
	for _, test := range []struct {
		name, driver, dsn, contents, password string
	}{
		{"keyword characters", "postgres", "host=postgres user=faka-58 dbname=faka-58 password=old sslmode=disable", " space 'quote' \\slash %percent /path  \n", " space 'quote' \\slash %percent /path  "},
		{"keyword old URL password", "postgres", "host=postgres user=faka-58 dbname=faka-58 password='http://old-password' sslmode=disable", "new-value\n", "new-value"},
		{"keyword URL application name", "postgres", "host=postgres user=faka-58 dbname=faka-58 application_name='faka://server' sslmode=disable", "new-value", "new-value"},
		{"keyword CRLF", "postgresql", "host=postgres user=faka-58 dbname=faka-58 sslmode=disable", " keep spaces \r\n", " keep spaces "},
		{"spaces only are significant", "postgres", "host=postgres user=faka-58 dbname=faka-58 sslmode=disable", "  \n", "  "},
		{"URI replaces both sources", "postgres", "postgres://faka-58:old-password@postgres:5432/faka-58?password=query-password&application_name=app&sslmode=disable", " space 'quote' \\slash %percent /path  \n", " space 'quote' \\slash %percent /path  "},
		{"URI second scheme", "postgresql", "postgresql://faka-58@postgres:5432/faka-58?sslmode=disable&password=old", "secret\n", "secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := DatabaseConfig{Driver: test.driver, DSN: test.dsn, PasswordFile: databasePasswordFixture(t, test.contents)}
			resolved, err := database.ResolveDSN()
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := pgx.ParseConfig(resolved)
			if err != nil {
				t.Fatal("resolved DSN cannot be parsed by PostgreSQL driver")
			}
			if parsed.Password != test.password || parsed.User != "faka-58" || parsed.Database != "faka-58" {
				t.Fatal("driver did not receive the exact password and existing database identity")
			}
			if test.name == "URI replaces both sources" && parsed.RuntimeParams["application_name"] != "app" {
				t.Fatal("URI password override removed an unrelated parameter")
			}
		})
	}
}

func TestResolveDatabasePasswordFileFailsWithoutLeakingSecrets(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-private-secret")
	for _, test := range []struct {
		name, driver, dsn, file string
	}{
		{"missing file", "postgres", "host=postgres password=sensitive-inline", missing},
		{"empty file", "postgres", "host=postgres", databasePasswordFixture(t, "")},
		{"newline only", "postgres", "host=postgres", databasePasswordFixture(t, "\n")},
		{"CRLF only", "postgres", "host=postgres", databasePasswordFixture(t, "\r\n")},
		{"NUL byte", "postgres", "host=postgres", databasePasswordFixture(t, "private\x00secret\n")},
		{"oversized", "postgres", "host=postgres", databasePasswordFixture(t, strings.Repeat("p", (64<<10)+1))},
		{"sqlite with password file", "sqlite", "private-sqlite.db", databasePasswordFixture(t, "secret")},
		{"malformed URI", "postgres", "postgres://faka-58:sensitive-inline@[broken/db", databasePasswordFixture(t, "private-file-secret")},
		{"malformed URI query", "postgres", "postgres://faka-58:sensitive-inline@postgres/db?password=%xy", databasePasswordFixture(t, "private-file-secret")},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := (DatabaseConfig{Driver: test.driver, DSN: test.dsn, PasswordFile: test.file}).ResolveDSN()
			if err == nil || err.Error() != errDatabasePasswordFile.Error() || resolved != "" {
				t.Fatal("invalid password-file configuration must return only the fixed error")
			}
			for _, private := range []string{test.file, "sensitive-inline", "private-file-secret"} {
				if strings.Contains(err.Error(), private) {
					t.Fatal("password-file error disclosed private information")
				}
			}
		})
	}
}

func TestResolveDSNWithoutPasswordFilePreservesConfiguration(t *testing.T) {
	for _, database := range []DatabaseConfig{
		{Driver: "sqlite", DSN: "./db/faka-58.db"},
		{Driver: "postgres", DSN: "host=postgres password='existing secret'"},
	} {
		resolved, err := database.ResolveDSN()
		if err != nil || resolved != database.DSN {
			t.Fatal("configuration without a password file changed")
		}
	}
}

func TestLoadResolvesPasswordFileAndFailsClosed(t *testing.T) {
	t.Chdir(t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
	passwordFile := databasePasswordFixture(t, " exact 'secret' \\value \n")
	document := map[string]interface{}{"database": map[string]string{
		"driver": "postgres", "dsn": "host=postgres user=faka-58 dbname=faka-58 sslmode=disable", "password_file": passwordFile,
	}}
	encoded, err := json.Marshal(document)
	if err != nil || os.WriteFile("config.yml", encoded, 0o600) != nil {
		t.Fatal("cannot write isolated configuration")
	}
	loaded := Load()
	parsed, err := pgx.ParseConfig(loaded.Database.DSN)
	if err != nil || parsed.Password != " exact 'secret' \\value " || loaded.Database.PasswordFile != passwordFile {
		t.Fatal("Load did not resolve the mounted password file")
	}
	if err := os.Remove(passwordFile); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	defer func() {
		if recovered := recover(); recovered != errDatabasePasswordFile {
			t.Fatal("Load must fail with a fixed error when its password file is unavailable")
		}
	}()
	Load()
}
