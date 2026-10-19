package upload

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	uploadapp "github.com/dujiao-next/internal/modules/upload/application"
	uploadcontract "github.com/dujiao-next/internal/modules/upload/contract"
	"github.com/dujiao-next/internal/modules/upload/infrastructure/localstore"
	"github.com/dujiao-next/internal/modules/upload/infrastructure/s3store"
)

// NewService 按启动配置选择上传后端，未设置时保持本地存储行为。
func NewService(cfg config.UploadConfig) (*uploadapp.Service, error) {
	store, err := newStore(cfg)
	if err != nil {
		return nil, err
	}
	return uploadapp.NewService(uploadapp.Policy{
		MaxSize: cfg.MaxSize, AllowedTypes: cfg.AllowedTypes, AllowedExtensions: cfg.AllowedExtensions,
		MaxWidth: cfg.MaxWidth, MaxHeight: cfg.MaxHeight,
	}, store), nil
}

func newStore(cfg config.UploadConfig) (uploadcontract.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Storage)) {
	case "", "local":
		return localstore.New("uploads"), nil
	case "s3":
		return s3store.New(s3store.Options{
			Endpoint: cfg.S3.Endpoint, Region: cfg.S3.Region, Bucket: cfg.S3.Bucket,
			AccessKeyID: cfg.S3.AccessKeyID, SecretAccessKey: cfg.S3.SecretAccessKey,
			PublicBaseURL: cfg.S3.PublicBaseURL, KeyPrefix: cfg.S3.KeyPrefix,
			Timeout: time.Duration(cfg.S3.TimeoutSeconds) * time.Second, MaxSize: cfg.MaxSize,
		})
	default:
		return nil, fmt.Errorf("upload: storage must be local or s3")
	}
}

// NewMediaFiles 为素材删除适配对象存储，并继续支持历史本地文件的读取与删除。
func NewMediaFiles(cfg config.UploadConfig, local contentcontract.FileStore) (contentcontract.FileStore, error) {
	store, err := newStore(cfg)
	if err != nil {
		return nil, err
	}
	if remote, ok := store.(*s3store.Store); ok {
		return mediaFiles{FileStore: local, remote: remote}, nil
	}
	return local, nil
}

type mediaFiles struct {
	contentcontract.FileStore
	remote *s3store.Store
}

func (s mediaFiles) Remove(name string) error {
	if s.remote.OwnsURL(name) {
		return s.remote.DeleteURL(name)
	}
	return s.FileStore.Remove(name)
}
