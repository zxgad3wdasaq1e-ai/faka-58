package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const testImage = "ghcr.io/example/store@sha256:1111111111111111111111111111111111111111111111111111111111111111"

func initFixture(t *testing.T) initOptions {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deploy", "Caddyfile"), []byte("{$SITE_ADDRESS} {\n    reverse_proxy app:8080\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return initOptions{Root: root, Domain: "xopenai.vip", Image: testImage, AdminUser: "admin", Subnet: "172.29.240.0/24"}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err == nil {
			result[path] = string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestInitCompatibleConfigurationAndPermissions(t *testing.T) {
	options := initFixture(t)
	state, reused, err := initialize(options)
	if err != nil || reused {
		t.Fatalf("initialize reused=%v err=%v", reused, err)
	}
	if state.Origin != "https://xopenai.vip" || state.Image != testImage || state.Storage != "local" {
		t.Fatal("unexpected installation state")
	}
	var config map[string]any
	if err := readJSON(filepath.Join(options.Root, "config.yml"), &config); err != nil {
		t.Fatal(err)
	}
	keys := []string{config["app"].(map[string]any)["secret_key"].(string), config["jwt"].(map[string]any)["secret"].(string), config["user_jwt"].(map[string]any)["secret"].(string)}
	for i, key := range keys {
		if len(key) != 64 || strings.Contains(state.AdminPath, key[:12]) {
			t.Fatal("application key length or admin path independence invalid")
		}
		for _, other := range keys[i+1:] {
			if key == other {
				t.Fatal("application keys must be independent")
			}
		}
	}
	if config["email"].(map[string]any)["enabled"] != false || config["upload"].(map[string]any)["storage"] != "local" {
		t.Fatal("new installation must disable mail and use local image storage")
	}
	password := config["bootstrap"].(map[string]any)["default_admin_password"].(string)
	if len(password) < 32 || !strings.HasPrefix(password, "Aa9_") {
		t.Fatal("initial administrator password does not satisfy policy")
	}
	env, err := os.ReadFile(filepath.Join(options.Root, ".env"))
	if err != nil || !strings.Contains(string(env), "APP_IMAGE="+testImage+"\n") || !strings.Contains(string(env), "COMPOSE_PROJECT_NAME=getgpt\n") {
		t.Fatal("environment image or project does not match installation state")
	}
	credentials, _ := os.ReadFile(filepath.Join(options.Root, "deploy", "secrets", "initial-admin.txt"))
	if !strings.Contains(string(credentials), password) {
		t.Fatal("initial credential file is inconsistent")
	}
	if runtime.GOOS != "windows" {
		for relative, mode := range map[string]os.FileMode{"config.yml": 0o600, ".env": 0o600, "deploy/runtime/installation.json": 0o600, "deploy/runtime/Caddyfile": 0o600, "deploy/secrets/initial-admin.txt": 0o600, "deploy/secrets/postgres_password": 0o444, "deploy/secrets/postgres_admin_password": 0o444, "deploy/secrets/redis_password": 0o600, "deploy/secrets": 0o700, "deploy/runtime": 0o700} {
			info, err := os.Stat(filepath.Join(options.Root, filepath.FromSlash(relative)))
			if err != nil || info.Mode().Perm() != mode {
				t.Errorf("%s mode invalid", relative)
			}
		}
	}
}

func TestInitExistingFilesNeverOverwritten(t *testing.T) {
	options := initFixture(t)
	if _, _, err := initialize(options); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, options.Root)
	options.Domain, options.AdminUser, options.Local = "other.example", "another", true
	if _, reused, err := initialize(options); err != nil || !reused {
		t.Fatalf("existing config not reused: %v", err)
	}
	options.Image = "ghcr.io/example/store:changed"
	if _, _, err := initialize(options); err == nil {
		t.Fatal("changing an existing image should fail")
	}
	options.Image = ""
	if _, reused, err := initialize(options); err != nil || !reused {
		t.Fatal("omitted image should preserve existing configuration")
	}
	after := snapshot(t, options.Root)
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Fatal("existing files or credentials changed")
	}
}

func TestInitLocalOriginAndMissingRuntimePreflight(t *testing.T) {
	options := initFixture(t)
	options.Local = true
	state, _, err := initialize(options)
	if err != nil || state.Origin != "http://localhost:1888" || !state.Local {
		t.Fatal("local initialization did not retain its HTTP origin")
	}
	env, _ := os.ReadFile(filepath.Join(options.Root, ".env"))
	for _, entry := range []string{"SITE_ADDRESS=:80\n", "HTTP_PORT=1888\n", "HTTPS_PORT=8443\n"} {
		if !strings.Contains(string(env), entry) {
			t.Fatal("local port contract changed")
		}
	}
	options = initFixture(t)
	if err := os.Remove(filepath.Join(options.Root, "deploy", "Caddyfile")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := initialize(options); err == nil {
		t.Fatal("missing runtime template accepted")
	}
	if _, err := os.Stat(filepath.Join(options.Root, "deploy", "secrets")); !os.IsNotExist(err) {
		t.Fatal("secrets created before runtime preflight succeeded")
	}
}

func TestInitInvalidInputDoesNotCreatePrivateFiles(t *testing.T) {
	mutations := []func(*initOptions){
		func(o *initOptions) { o.Image = "ghcr.io/example/store:$TOKEN" },
		func(o *initOptions) { o.Image += "\nENV=bad" },
		func(o *initOptions) { o.Image = "" },
		func(o *initOptions) { o.Image = "registry:99999/store:v1" },
		func(o *initOptions) { o.Domain = "https://example.com" },
		func(o *initOptions) { o.Domain = "example.com\nENV=bad" },
		func(o *initOptions) { o.AdminUser = "admin\nENV=bad" },
		func(o *initOptions) { o.Subnet = "8.8.8.0/24" },
		func(o *initOptions) { o.Subnet = "172.29.240.1/24" },
	}
	for index, mutate := range mutations {
		options := initFixture(t)
		mutate(&options)
		if _, _, err := initialize(options); err == nil {
			t.Errorf("invalid input %d accepted", index)
		}
		for _, name := range []string{"config.yml", ".env", "deploy/secrets", "deploy/runtime"} {
			if _, err := os.Lstat(filepath.Join(options.Root, filepath.FromSlash(name))); !os.IsNotExist(err) {
				t.Errorf("invalid input %d wrote %s", index, name)
			}
		}
	}
}

func TestInitPartialAndCollidingSecretsFailClosed(t *testing.T) {
	for _, relative := range []string{"config.yml", "deploy/secrets/redis_password"} {
		options := initFixture(t)
		path := filepath.Join(options.Root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("existing-private-value"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := initialize(options); err == nil {
			t.Fatal("partial or colliding configuration accepted")
		}
		contents, _ := os.ReadFile(path)
		if string(contents) != "existing-private-value" {
			t.Fatal("existing secret overwritten")
		}
	}
}

func TestInitRejectsSymlinkPrivateDirectory(t *testing.T) {
	options := initFixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(options.Root, "deploy", "secrets")); err != nil {
		t.Skip("symlink creation unavailable")
	}
	if _, _, err := initialize(options); err == nil {
		t.Fatal("symlink private directory accepted")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("private files escaped deployment root")
	}
}

func TestInfoStrictSingleValueAndNoSecrets(t *testing.T) {
	options := initFixture(t)
	state, _, err := initialize(options)
	if err != nil {
		t.Fatal(err)
	}
	for field, expected := range map[string]string{"origin": state.Origin, "admin_path": state.AdminPath, "image": state.Image} {
		var stdout, stderr bytes.Buffer
		if runCLI([]string{"info", "--root", options.Root, "--field", field}, &stdout, &stderr) != 0 || stdout.String() != expected+"\n" || stderr.Len() != 0 {
			t.Errorf("info field %s violated single-value protocol", field)
		}
	}
	if _, err := installationInfo(options.Root, "password"); err == nil {
		t.Fatal("unknown info field accepted")
	}
	state.Origin = "https://example.com\nTOKEN=private"
	state.AdminPath = "/console\nTOKEN=private"
	state.Image = "$(private)"
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(options.Root, "deploy", "runtime", "installation.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"origin", "admin_path", "image"} {
		var stdout, stderr bytes.Buffer
		if runCLI([]string{"info", "--root", options.Root, "--field", field}, &stdout, &stderr) == 0 || stdout.Len() != 0 || strings.Contains(stderr.String(), "TOKEN=") || strings.Contains(stderr.String(), "$(private)") {
			t.Errorf("info emitted invalid or private %s", field)
		}
	}
}
