package config

import (
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
)

var errDatabasePasswordFile = errors.New("数据库密码文件或连接配置无效")

// ResolveDSN reads the same mounted secret used to initialize PostgreSQL.
// It keeps leading/trailing spaces in the password and strips only the file's
// final line ending. Neither file contents nor the DSN appear in errors.
func (c DatabaseConfig) ResolveDSN() (string, error) {
	if c.PasswordFile == "" {
		return c.DSN, nil
	}
	if c.Driver != "postgres" && c.Driver != "postgresql" {
		return "", errDatabasePasswordFile
	}
	file, err := os.Open(c.PasswordFile)
	if err != nil {
		return "", errDatabasePasswordFile
	}
	defer file.Close()
	const maximumPasswordFileBytes = 64 << 10
	contents, err := io.ReadAll(io.LimitReader(file, maximumPasswordFileBytes+1))
	if err != nil || len(contents) > maximumPasswordFileBytes {
		return "", errDatabasePasswordFile
	}
	password := string(contents)
	if strings.HasSuffix(password, "\r\n") {
		password = strings.TrimSuffix(password, "\r\n")
	} else {
		password = strings.TrimSuffix(password, "\n")
	}
	if password == "" || strings.ContainsRune(password, '\x00') {
		return "", errDatabasePasswordFile
	}
	if strings.HasPrefix(c.DSN, "postgres://") || strings.HasPrefix(c.DSN, "postgresql://") {
		parsed, err := url.Parse(c.DSN)
		if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Fragment != "" {
			return "", errDatabasePasswordFile
		}
		query, err := url.ParseQuery(parsed.RawQuery)
		if err != nil {
			return "", errDatabasePasswordFile
		}
		query.Del("password")
		parsed.RawQuery = query.Encode()
		username := ""
		if parsed.User != nil {
			username = parsed.User.Username()
		}
		parsed.User = url.UserPassword(username, password)
		return parsed.String(), nil
	}
	escaped := strings.ReplaceAll(password, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return c.DSN + " password='" + escaped + "'", nil
}
