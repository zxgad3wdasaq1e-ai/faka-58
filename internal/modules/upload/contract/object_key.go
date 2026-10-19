package contract

import (
	"fmt"
	"strings"
)

// ObjectKey 只接受独立的安全路径段，阻止本地路径穿越和对象路径歧义。
func ObjectKey(input StoreInput) (string, error) {
	parts := []string{input.Scene, input.Year, input.Month, input.Filename}
	for _, part := range parts {
		if !SafeKeySegment(part) {
			return "", fmt.Errorf("upload: invalid object key segment")
		}
	}
	return strings.Join(parts, "/"), nil
}

// SafeKeySegment 限制存储路径为 ASCII 文件名，兼容 Windows 和 S3。
func SafeKeySegment(value string) bool {
	if value == "" || len(value) > 255 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}
