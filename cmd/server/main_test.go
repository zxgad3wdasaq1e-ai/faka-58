package main

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/config"
)

var bannerANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestDefaultAdminCredentialsPreferCurrentEnvironmentAndPreserveLegacy(t *testing.T) {
	cfg := &config.Config{Bootstrap: config.BootstrapConfig{
		DefaultAdminUsername: "configured-admin",
		DefaultAdminPassword: "configured-password",
	}}
	for _, suffix := range []string{"USERNAME", "PASSWORD"} {
		key := "FAKA_58_DEFAULT_ADMIN_" + suffix
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DJ_DEFAULT_ADMIN_USERNAME", " legacy-admin ")
	t.Setenv("DJ_DEFAULT_ADMIN_PASSWORD", " legacy-password ")
	if user, pass := resolveDefaultAdminCredentials(cfg); user != "legacy-admin" || pass != "legacy-password" {
		t.Fatalf("legacy bootstrap credentials = %q / %q", user, pass)
	}
	t.Setenv("FAKA_58_DEFAULT_ADMIN_USERNAME", " current-admin ")
	t.Setenv("FAKA_58_DEFAULT_ADMIN_PASSWORD", " current-password ")
	if user, pass := resolveDefaultAdminCredentials(cfg); user != "current-admin" || pass != "current-password" {
		t.Fatalf("current bootstrap keys did not take precedence: %q / %q", user, pass)
	}
	t.Setenv("FAKA_58_DEFAULT_ADMIN_USERNAME", "")
	t.Setenv("FAKA_58_DEFAULT_ADMIN_PASSWORD", "")
	if user, pass := resolveDefaultAdminCredentials(cfg); user != "configured-admin" || pass != "configured-password" {
		t.Fatalf("explicitly cleared current keys must fall back to config: %q / %q", user, pass)
	}
	if user, pass := resolveDefaultAdminCredentials(nil); user != "" || pass != "" {
		t.Fatal("cleared current keys must not restore legacy bootstrap credentials")
	}
}

// bannerLinks 提取横幅里的链接，剥掉 ANSI 颜色码并按空白切分。
// 断言链接本身而非"标签 + 对齐空白"的整行字面量，横幅排版微调就不会连带弄红 CI。
func bannerLinks(banner string) []string {
	var links []string
	for _, field := range strings.Fields(bannerANSIPattern.ReplaceAllString(banner, "")) {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			links = append(links, field)
		}
	}
	return links
}

func TestWriteStartupBannerUsesStoreBrandAndLinks(t *testing.T) {
	var output strings.Builder
	writeStartupBanner(&output)

	banner := output.String()
	if !strings.Contains(banner, "faka-58") {
		t.Fatal("startup banner is missing the current project name")
	}
	for _, removed := range []string{
		"Dujiao-Next",
		"XOPEN AI",
		"XIOEN AI",
		"https://github.com/dujiao-next",
		"https://dujiao-next.com",
		"https://t.me/dujiaonext_official",
	} {
		if strings.Contains(banner, removed) {
			t.Errorf("startup banner still contains retired repository: %s", removed)
		}
	}

	// 按完整链接比较，避免仓库地址的前缀被误当成完整文档入口。
	links := bannerLinks(banner)
	for _, retained := range []string{
		"https://xopenai.vip",
		"https://github.com/zxgad3wdasaq1e-ai/faka-58",
		"https://github.com/zxgad3wdasaq1e-ai/faka-58/blob/main/README.md",
	} {
		if !slices.Contains(links, retained) {
			t.Errorf("startup banner is missing retained repository: %s (got %v)", retained, links)
		}
	}
}

func TestWeakRuntimeSecretNamesCoversEveryRootSecret(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "change-me-32-byte-secret-key!!"},
		JWT:     config.JWTConfig{SecretKey: "your-secret-key-change-in-production-please"},
		UserJWT: config.JWTConfig{SecretKey: "user-change-me-in-production"},
	}

	want := []string{"app.secret_key", "jwt.secret", "user_jwt.secret"}
	if got := weakRuntimeSecretNames(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("weak runtime secrets want %v got %v", want, got)
	}
}

func TestWeakRuntimeSecretNamesAcceptsStrongIndependentSecrets(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "2f8d164772cd4bbcaef8fa4ad19a2a26f7a15505"},
		JWT:     config.JWTConfig{SecretKey: "dd914407e55c4528a393fe522215c18f5fc8687b"},
		UserJWT: config.JWTConfig{SecretKey: "ca36df49b49446d2a9b2cac7f035d11574575b53"},
	}

	if got := weakRuntimeSecretNames(cfg); len(got) != 0 {
		t.Fatalf("strong runtime secrets reported as weak: %v", got)
	}
}

func TestWeakRuntimeSecretNamesRejectsReusedStrongSecrets(t *testing.T) {
	shared := "2f8d164772cd4bbcaef8fa4ad19a2a26f7a15505"
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: shared},
		JWT:     config.JWTConfig{SecretKey: shared},
		UserJWT: config.JWTConfig{SecretKey: "ca36df49b49446d2a9b2cac7f035d11574575b53"},
	}

	want := []string{"app.secret_key", "jwt.secret"}
	if got := weakRuntimeSecretNames(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("reused runtime secrets want %v got %v", want, got)
	}
}

func TestUnsafeBootstrapAdminPasswordRejectsDefaultsAndPolicyViolations(t *testing.T) {
	cfg := &config.Config{Security: config.SecurityConfig{PasswordPolicy: config.PasswordPolicyConfig{
		MinLength:     10,
		RequireUpper:  true,
		RequireLower:  true,
		RequireNumber: true,
	}}}

	for _, password := range []string{"admin123", "alllowercase1", "NOLOWERCASE1", "NoNumberHere"} {
		if !unsafeBootstrapAdminPassword(cfg, password) {
			t.Errorf("expected bootstrap password %q to be rejected", password)
		}
	}
	if unsafeBootstrapAdminPassword(cfg, "StrongBootstrap123") {
		t.Fatal("expected strong bootstrap password to be accepted")
	}
	if unsafeBootstrapAdminPassword(cfg, "") {
		t.Fatal("empty bootstrap password should keep the skip-initialization behavior")
	}
}
