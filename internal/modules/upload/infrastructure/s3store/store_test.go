package s3store

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/upload/contract"
)

func testOptions(endpoint string) Options {
	return Options{
		Endpoint: endpoint, Region: "us-east-1", Bucket: "storefront",
		AccessKeyID: "test-access", SecretAccessKey: "test-secret",
		PublicBaseURL: "https://shop.example.com/media", Timeout: time.Second, MaxSize: 1024,
	}
}

func objectInput(content string) contract.StoreInput {
	return contract.StoreInput{
		Source: strings.NewReader(content), Size: int64(len(content)), MimeType: "image/png",
		Scene: "common", Year: "2026", Month: "10", Filename: "asset.png",
	}
}

func TestStoreUploadsToInternalEndpointAndReturnsPublicURL(t *testing.T) {
	var put, deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/storefront/images/common/2026/10/asset.png" {
			t.Errorf("unexpected object path %q", r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=test-access/") || r.Header.Get("X-Amz-Content-Sha256") == "" {
			t.Errorf("missing signed credentials or payload checksum")
		}
		switch r.Method {
		case http.MethodPut:
			put = true
			body, _ := io.ReadAll(r.Body)
			if string(body) != "image-content" || r.ContentLength != int64(len(body)) || r.Header.Get("Content-Type") != "image/png" {
				t.Errorf("body, content length or MIME type changed")
			}
		case http.MethodDelete:
			deleted = true
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	options := testOptions(server.URL)
	options.KeyPrefix = "images"
	store, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	publicURL, err := store.Save(objectInput("image-content"))
	if err != nil {
		t.Fatal(err)
	}
	if publicURL != "https://shop.example.com/media/images/common/2026/10/asset.png" {
		t.Fatalf("wrong public URL: %q", publicURL)
	}
	if err := store.DeleteURL(publicURL); err != nil {
		t.Fatal(err)
	}
	if !put || !deleted {
		t.Fatal("expected upload and physical deletion")
	}
}

func TestSigV4MatchesIndependentFixedFixture(t *testing.T) {
	store, err := New(testOptions("https://s3.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC) }
	request, err := store.request(http.MethodPut, "common/2026/10/asset.txt", []byte("hello world"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	// Fixed vector independently calculated with Python hashlib/hmac from the SigV4 canonical request.
	expected := "AWS4-HMAC-SHA256 Credential=test-access/20261019/us-east-1/s3/aws4_request, SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date, Signature=d7e3ccd5d637372eeaa0197bb26cf0a3ebd7be1d4a16c1e81ebd8a89493e1440"
	if actual := request.Header.Get("Authorization"); actual != expected {
		t.Fatalf("signature mismatch:\n%s", actual)
	}
}

func TestStoreRejectsUnsafePathsAndSizeBeforeNetwork(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	store, err := New(testOptions(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"../escape.png", `..\escape.png`, "https:evil", "%2e%2e", ".hidden"} {
		input := objectInput("data")
		input.Filename = unsafe
		if _, err := store.Save(input); err == nil {
			t.Errorf("unsafe filename accepted: %q", unsafe)
		}
	}
	for _, value := range []string{"https://evil.example/media/file.png", "https://shop.example.com/media/../secret", "https://shop.example.com/media/%2e%2e/secret"} {
		if err := store.DeleteURL(value); err == nil {
			t.Errorf("unsafe deletion accepted: %q", value)
		}
	}
	input := objectInput("content larger than declared")
	input.Size = 1
	if _, err := store.Save(input); err == nil {
		t.Error("size mismatch accepted")
	}
	if _, err := store.Save(objectInput(strings.Repeat("a", 1025))); err == nil {
		t.Error("oversized upload accepted")
	}
	if requests != 0 {
		t.Fatalf("invalid input made %d network requests", requests)
	}
}

func TestStoreRejectsRedirectAndStorageFailure(t *testing.T) {
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	status := http.StatusTemporaryRedirect
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(status)
	}))
	defer server.Close()
	store, err := New(testOptions(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(objectInput("data")); err == nil || leaked {
		t.Fatal("redirect followed or storage error ignored")
	}
	status = http.StatusForbidden
	if _, err := store.Save(objectInput("data")); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("storage denial not propagated: %v", err)
	}
}

func TestStoreRejectsInvalidConfiguration(t *testing.T) {
	for name, change := range map[string]func(*Options){
		"endpoint scheme": func(o *Options) { o.Endpoint = "file:///tmp/bucket" },
		"endpoint auth":   func(o *Options) { o.Endpoint = "https://user:secret@example.com" },
		"endpoint query":  func(o *Options) { o.Endpoint = "https://example.com?redirect=evil" },
		"endpoint path":   func(o *Options) { o.Endpoint = "https://example.com/prefix" },
		"public URL":      func(o *Options) { o.PublicBaseURL = "javascript:alert(1)" },
		"public query":    func(o *Options) { o.PublicBaseURL = "https://example.com?signature=x" },
		"bucket":          func(o *Options) { o.Bucket = "../other" },
		"prefix":          func(o *Options) { o.KeyPrefix = "images/../other" },
		"credential":      func(o *Options) { o.SecretAccessKey = "" },
		"timeout":         func(o *Options) { o.Timeout = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			options := testOptions("https://s3.example.com")
			change(&options)
			if _, err := New(options); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
