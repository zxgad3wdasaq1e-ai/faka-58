package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestSQLiteDefaultPreservesExistingInstallation(t *testing.T) {
	t.Chdir(t.TempDir())
	if got := defaultSQLiteDSN(); got != "./db/faka-58.db" {
		t.Fatalf("new database default = %q", got)
	}
	if err := os.Mkdir("db", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("db/dujiao.db", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got := defaultSQLiteDSN(); got != "./db/dujiao.db" {
		t.Fatalf("existing database was abandoned: %q", got)
	}
	if err := os.WriteFile("db/faka-58.db", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got := defaultSQLiteDSN(); got != "./db/faka-58.db" {
		t.Fatalf("current database must take precedence: %q", got)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("DATABASE_DSN", "./db/custom.db")
	if got := Load().Database.DSN; got != "./db/custom.db" {
		t.Fatalf("explicit database configuration changed: %q", got)
	}
}
