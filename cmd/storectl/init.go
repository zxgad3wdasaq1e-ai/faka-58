package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type initOptions struct {
	Root, Domain, Image, AdminUser, Subnet string
	Local                                  bool
}

type installationState struct {
	Origin    string `json:"origin"`
	AdminPath string `json:"admin_path"`
	AdminUser string `json:"admin_user"`
	Storage   string `json:"storage"`
	Local     bool   `json:"local"`
	Image     string `json:"image"`
}

var imagePattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*(?::[0-9]{1,5})?/)?[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})$`)
var adminPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{2,31}$`)
var adminPathPattern = regexp.MustCompile(`^/[A-Za-z0-9][A-Za-z0-9/_-]*$`)
var domainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var tldPattern = regexp.MustCompile(`^[a-z]{2,63}$`)

func validateImage(image string) error {
	if len(image) > 255 || !imagePattern.MatchString(image) {
		return errors.New("--image 需要带显式 tag 或 sha256 摘要的 OCI 镜像，不能包含协议、变量或空白")
	}
	first, _, hasPath := strings.Cut(image, "/")
	if hasPath && strings.Contains(first, ":") {
		_, port, _ := strings.Cut(first, ":")
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("镜像仓库端口需在 1–65535 之间")
		}
	}
	return nil
}

func validateDomain(domain string) error {
	parts := strings.Split(domain, ".")
	if len(domain) > 253 || len(parts) < 2 || !tldPattern.MatchString(parts[len(parts)-1]) {
		return errors.New("--domain 需要正式域名，不含协议、端口或路径")
	}
	for _, label := range parts {
		if !domainLabelPattern.MatchString(label) {
			return errors.New("--domain 需要正式域名，不含协议、端口或路径")
		}
	}
	return nil
}

func readJSON(path string, destination any) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.New("无法读取必要的私有 JSON 配置文件")
	}
	defer f.Close()
	contents, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil || len(contents) > 1024*1024 {
		return errors.New("私有配置读取失败或超过大小限制")
	}
	contents = []byte(strings.TrimPrefix(string(contents), "\ufeff"))
	if !utf8.Valid(contents) || json.Unmarshal(contents, destination) != nil {
		return errors.New("私有配置不是有效 JSON；仅支持初始化工具生成的 JSON 格式 YAML")
	}
	return nil
}

func readState(root string) (installationState, error) {
	var state installationState
	err := readJSON(filepath.Join(root, "deploy", "runtime", "installation.json"), &state)
	return state, err
}

func installationInfo(root, field string) (string, error) {
	if field != "origin" && field != "admin_path" && field != "image" {
		return "", errors.New("--field 仅支持 origin、admin_path、image")
	}
	state, err := readState(root)
	if err != nil {
		return "", err
	}
	switch field {
	case "origin":
		value, err := originURL(state.Origin)
		if err != nil || value != state.Origin {
			return "", errors.New("安装状态中的 origin 无效")
		}
		return value, nil
	case "admin_path":
		if !adminPathPattern.MatchString(state.AdminPath) || strings.Contains(state.AdminPath, "//") {
			return "", errors.New("安装状态中的 admin_path 无效")
		}
		return state.AdminPath, nil
	default:
		if err := validateImage(state.Image); err != nil {
			return "", errors.New("安装状态中没有有效的镜像引用；不会自动切换源码部署模式")
		}
		return state.Image, nil
	}
}

func randomSecret() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", errors.New("无法从系统生成随机密钥")
	}
	return hex.EncodeToString(bytes[:]), nil
}

func privateDirectory(root, relative string) error {
	current := root
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err = os.Mkdir(current, 0o700); err != nil {
				return errors.New("无法创建私有部署目录")
			}
		} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("部署目录不能是符号链接或普通文件")
		}
	}
	return os.Chmod(current, 0o700)
}

func privateWrite(path string, contents []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("无法独占创建私有配置；不会覆盖已有文件或密钥")
	}
	if _, err = f.Write(contents); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.New("写入私有配置失败；请保留既有密钥并检查磁盘状态")
	}
	return os.Chmod(path, mode)
}

func initialize(options initOptions) (installationState, bool, error) {
	var zero installationState
	if options.Image != "" {
		if err := validateImage(options.Image); err != nil {
			return zero, false, err
		}
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return zero, false, errors.New("部署根目录无效")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return zero, false, errors.New("部署根目录需要预先创建")
	}
	count := 0
	for _, relative := range []string{"config.yml", ".env", "deploy/runtime/installation.json"} {
		info, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(relative)))
		if statErr == nil {
			if !info.Mode().IsRegular() {
				return zero, false, errors.New("部署配置不能是目录或符号链接")
			}
			count++
		} else if !os.IsNotExist(statErr) {
			return zero, false, errors.New("无法检查既有部署配置")
		}
	}
	if count == 3 {
		state, err := readState(root)
		if err != nil {
			return zero, false, err
		}
		if options.Image != "" && state.Image != options.Image {
			return zero, false, errors.New("镜像与既有安装模式或镜像不一致；不会覆盖配置或切换模式")
		}
		return state, true, nil
	}
	if count != 0 {
		return zero, false, errors.New("发现不完整的既有配置；请恢复 config.yml、.env 和 installation.json，不会覆盖密钥")
	}
	if err := validateImage(options.Image); err != nil {
		return zero, false, err
	}
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(options.Domain)), ".")
	if !options.Local {
		if err := validateDomain(domain); err != nil {
			return zero, false, err
		}
	}
	if !adminPattern.MatchString(options.AdminUser) {
		return zero, false, errors.New("管理员用户名需为 3–32 位字母、数字、下划线或连字符")
	}
	network, err := netip.ParsePrefix(options.Subnet)
	if err != nil || !network.Addr().Is4() || !network.Addr().IsPrivate() || network.Bits() != 24 || network != network.Masked() {
		return zero, false, errors.New("--subnet 需要不含主机位的私有 IPv4 /24 网段")
	}
	address := network.Addr().As4()
	address[3] += 10
	proxyIP := netip.AddrFrom4(address).String()
	caddy, err := os.ReadFile(filepath.Join(root, "deploy", "Caddyfile"))
	if err != nil || len(caddy) == 0 {
		return zero, false, errors.New("缺少 deploy/Caddyfile；请先下载匹配镜像版本的运行文件")
	}
	values := make([]string, 8)
	for i := range values {
		values[i], err = randomSecret()
		if err != nil {
			return zero, false, err
		}
	}
	appKey, jwtKey, userKey, pgPassword, pgAdminPassword, redisPassword, adminRandom := values[0], values[1], values[2], values[3], values[4], values[5], values[6]
	adminPassword := "Aa9_" + adminRandom
	origin, siteAddress, httpPort, httpsPort, mainHost := "https://"+domain, "https://"+domain, 80, 443, domain
	if options.Local {
		origin, siteAddress, httpPort, httpsPort, mainHost = "http://localhost:1888", ":80", 1888, 8443, "localhost"
	}
	state := installationState{Origin: origin, AdminPath: "/console-" + values[7][:12], AdminUser: options.AdminUser, Storage: "local", Local: options.Local, Image: options.Image}
	config := map[string]any{
		"app":      map[string]any{"secret_key": appKey, "totp_issuer": "XIOEN AI"},
		"server":   map[string]any{"host": "0.0.0.0", "port": 8080, "mode": "release", "trusted_proxies": []string{proxyIP + "/32"}},
		"database": map[string]any{"driver": "postgres", "dsn": fmt.Sprintf("host=postgres user=getgpt password=%s dbname=getgpt port=5432 sslmode=disable TimeZone=Asia/Shanghai", pgPassword), "pool": map[string]int{"max_open_conns": 20, "max_idle_conns": 5, "conn_max_lifetime_seconds": 1800, "conn_max_idle_time_seconds": 300}},
		"jwt":      map[string]any{"secret": jwtKey, "expire_hours": 12}, "user_jwt": map[string]any{"secret": userKey, "expire_hours": 24, "remember_me_expire_hours": 168},
		"bootstrap":     map[string]string{"default_admin_username": options.AdminUser, "default_admin_password": adminPassword},
		"redis":         map[string]any{"enabled": true, "host": "redis", "port": 6379, "password": redisPassword, "db": 0, "prefix": "xioen"},
		"queue":         map[string]any{"enabled": true, "host": "redis", "port": 6379, "password": redisPassword, "db": 1, "concurrency": 10, "queues": map[string]int{"default": 10, "critical": 5}},
		"upload":        map[string]any{"storage": "local", "max_size": 10485760, "allowed_types": []string{"image/jpeg", "image/png", "image/gif", "image/webp"}, "allowed_extensions": []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}, "max_width": 4096, "max_height": 4096},
		"cors":          map[string]any{"allowed_origins": []string{origin}, "allowed_methods": []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, "allowed_headers": []string{"Content-Type", "Authorization", "X-Requested-With", "X-CSRF-Token"}, "allow_credentials": true, "max_age": 600},
		"email":         map[string]any{"enabled": false, "host": "", "port": 587, "username": "", "password": "", "from": "", "from_name": "XIOEN AI", "use_tls": true, "use_ssl": false},
		"telegram_auth": map[string]bool{"enabled": false}, "google_auth": map[string]bool{"enabled": false},
		"reseller": map[string]any{"enabled": false, "main_hosts": []string{mainHost}, "trusted_forwarded_host": false}, "web": map[string]string{"admin_path": state.AdminPath},
	}
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return zero, false, errors.New("无法编码部署配置")
	}
	stateJSON, _ := json.MarshalIndent(state, "", "  ")
	env := fmt.Sprintf("COMPOSE_PROJECT_NAME=getgpt\nSITE_ADDRESS=%s\nHTTP_PORT=%d\nHTTPS_PORT=%d\nAPP_SUBNET=%s\nCADDY_IP=%s\nAPP_IMAGE=%s\nAPP_VERSION=v1.0.0-xioen.1\nCOMPOSE_FILE=compose.yml\n", siteAddress, httpPort, httpsPort, network, proxyIP, options.Image)
	files := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{"deploy/secrets/postgres_password", []byte(pgPassword + "\n"), 0o444},
		{"deploy/secrets/postgres_admin_password", []byte(pgAdminPassword + "\n"), 0o444},
		{"deploy/secrets/redis_password", []byte(redisPassword + "\n"), 0o600},
		{".env", []byte(env), 0o600}, {"config.yml", append(configJSON, '\n'), 0o600},
		{"deploy/runtime/Caddyfile", caddy, 0o600},
		{"deploy/runtime/installation.json", append(stateJSON, '\n'), 0o600},
		{"deploy/secrets/initial-admin.txt", []byte(fmt.Sprintf("后台: %s%s\n用户名: %s\n首次密码: %s\n首次登录后修改密码并启用 2FA。此文件含密钥，不得提交或分享。\n", origin, state.AdminPath, options.AdminUser, adminPassword)), 0o600},
	}
	for _, file := range files {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(file.name))); !os.IsNotExist(err) {
			return zero, false, errors.New("发现既有部署文件；不会覆盖或轮换密钥")
		}
	}
	for _, dir := range []string{"deploy/secrets", "deploy/runtime"} {
		if err := privateDirectory(root, dir); err != nil {
			return zero, false, err
		}
	}
	for _, file := range files {
		if err := privateWrite(filepath.Join(root, filepath.FromSlash(file.name)), file.data, file.mode); err != nil {
			return zero, false, err
		}
	}
	return state, false, nil
}
