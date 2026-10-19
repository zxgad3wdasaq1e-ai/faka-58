package localstore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dujiao-next/internal/modules/upload/contract"
)

// Store 将上传文件写入本地 uploads 目录。
type Store struct {
	root string
}

var _ contract.Store = (*Store)(nil)

// New 创建本地文件存储适配器。
func New(root string) *Store {
	if root == "" {
		panic("upload local store: root is empty")
	}
	return &Store{root: root}
}

// Save 保存文件并返回公开访问 URL。
func (s *Store) Save(input contract.StoreInput) (string, error) {
	key, err := contract.ObjectKey(input)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(s.root, input.Scene, input.Year, input.Month)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(directory, input.Filename)
	destination, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(destination, input.Source); err != nil {
		_ = destination.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := destination.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("/uploads/%s", key), nil
}
