package upload

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/config"
)

type localFilesStub struct{ removed string }

func (*localFilesStub) Stat(string) (fs.FileInfo, error)   { return nil, fs.ErrNotExist }
func (*localFilesStub) Open(string) (io.ReadCloser, error) { return nil, fs.ErrNotExist }
func (s *localFilesStub) Remove(name string) error         { s.removed = name; return nil }

func TestNewServicePreservesLocalDefaultAndRejectsUnknownStorage(t *testing.T) {
	if service, err := NewService(config.UploadConfig{MaxSize: 1024}); err != nil || service == nil {
		t.Fatalf("local default failed: %v", err)
	}
	if _, err := NewService(config.UploadConfig{Storage: "unsupported"}); err == nil {
		t.Fatal("unknown storage silently accepted")
	}
	if _, err := NewService(config.UploadConfig{Storage: "s3"}); err == nil {
		t.Fatal("incomplete remote storage silently accepted")
	}
}

func TestMediaFilesDeletesRemoteObjectAndPreservesLocalHistory(t *testing.T) {
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/storefront/common/2026/10/asset.png" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	local := &localFilesStub{}
	cfg := config.UploadConfig{Storage: "s3", MaxSize: 1024, S3: config.UploadS3Config{
		Endpoint: server.URL, Region: "us-east-1", Bucket: "storefront",
		AccessKeyID: "test", SecretAccessKey: "test", PublicBaseURL: "https://shop.example.com/media", TimeoutSeconds: 30,
	}}
	files, err := NewMediaFiles(cfg, local)
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Remove("https://shop.example.com/media/common/2026/10/asset.png"); err != nil {
		t.Fatal(err)
	}
	if !deleted || local.removed != "" {
		t.Fatal("remote deletion was not dispatched to object storage")
	}
	if err := files.Remove("uploads/common/legacy.png"); err != nil || local.removed != "uploads/common/legacy.png" {
		t.Fatalf("legacy local deletion failed: %v", err)
	}
}
