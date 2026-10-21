// Package s3store 提供标准 SigV4 path-style S3/MinIO 存储，不依赖额外 SDK。
package s3store

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/upload/contract"
)

// Options 仅从服务器启动配置装配，客户端不能指定目标端点或凭据。
type Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	KeyPrefix       string
	Timeout         time.Duration
	MaxSize         int64
}

// Store 使用签名请求上传对象，返回与内部 API 地址分离的公开 URL。
type Store struct {
	options  Options
	endpoint *url.URL
	client   *http.Client
	now      func() time.Time
}

var _ contract.Store = (*Store)(nil)

// New 校验完整配置；启动时失败，避免静默回落到错误的存储后端。
func New(options Options) (*Store, error) {
	endpoint, err := parseHTTPURL(options.Endpoint)
	if err != nil || endpoint.Path != "" && endpoint.Path != "/" {
		return nil, fmt.Errorf("upload s3: endpoint must be an HTTP(S) origin without a path")
	}
	if _, err := parseHTTPURL(options.PublicBaseURL); err != nil {
		return nil, fmt.Errorf("upload s3: public_base_url must be an HTTP(S) URL without credentials, query or fragment")
	}
	if !validBucket(options.Bucket) {
		return nil, fmt.Errorf("upload s3: bucket must be a valid lowercase S3 bucket name")
	}
	if !contract.SafeKeySegment(options.Region) || strings.TrimSpace(options.AccessKeyID) == "" || strings.TrimSpace(options.SecretAccessKey) == "" {
		return nil, fmt.Errorf("upload s3: region and credentials are required")
	}
	if strings.ContainsAny(options.AccessKeyID, "\r\n/ ,") {
		return nil, fmt.Errorf("upload s3: invalid access_key_id")
	}
	options.KeyPrefix = strings.Trim(options.KeyPrefix, "/")
	if options.KeyPrefix != "" {
		for _, part := range strings.Split(options.KeyPrefix, "/") {
			if !contract.SafeKeySegment(part) {
				return nil, fmt.Errorf("upload s3: key_prefix contains an unsafe path segment")
			}
		}
	}
	if options.MaxSize <= 0 || options.Timeout <= 0 || options.Timeout > 5*time.Minute {
		return nil, fmt.Errorf("upload s3: positive max_size and timeout of at most 300 seconds are required")
	}
	options.PublicBaseURL = strings.TrimRight(options.PublicBaseURL, "/")
	return &Store{
		options:  options,
		endpoint: endpoint,
		client: &http.Client{
			Timeout: options.Timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now,
	}, nil
}

// Save 上传有大小上限的完整对象，并保留经过内容嗅探的 MIME 类型。
func (s *Store) Save(input contract.StoreInput) (string, error) {
	key, err := contract.ObjectKey(input)
	if err != nil {
		return "", err
	}
	if input.Source == nil || input.Size < 0 || input.Size > s.options.MaxSize {
		return "", fmt.Errorf("upload s3: invalid object size")
	}
	data, err := io.ReadAll(io.LimitReader(input.Source, s.options.MaxSize+1))
	if err != nil {
		return "", fmt.Errorf("upload s3: read object: %w", err)
	}
	if int64(len(data)) > s.options.MaxSize || int64(len(data)) != input.Size {
		return "", fmt.Errorf("upload s3: object size does not match validated size")
	}
	if s.options.KeyPrefix != "" {
		key = s.options.KeyPrefix + "/" + key
	}
	contentType := input.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	request, err := s.request(http.MethodPut, key, data, contentType)
	if err != nil {
		return "", err
	}
	request.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	if !strings.HasPrefix(contentType, "image/") {
		request.Header.Set("Content-Disposition", "attachment")
	}
	if err := s.execute(request); err != nil {
		return "", err
	}
	return s.options.PublicBaseURL + "/" + key, nil
}

// OwnsURL 确认 URL 属于配置的公开前缀；不信任 URL 中的主机或桶名。
func (s *Store) OwnsURL(publicURL string) bool {
	return strings.HasPrefix(publicURL, s.options.PublicBaseURL+"/")
}

// DeleteURL 只删除本存储公开前缀内的对象，拒绝路径穿越与其他服务器 URL。
func (s *Store) DeleteURL(publicURL string) error {
	if !s.OwnsURL(publicURL) {
		return fmt.Errorf("upload s3: object URL does not belong to this store")
	}
	key := strings.TrimPrefix(publicURL, s.options.PublicBaseURL+"/")
	if s.options.KeyPrefix != "" && !strings.HasPrefix(key, s.options.KeyPrefix+"/") {
		return fmt.Errorf("upload s3: object is outside the configured key_prefix")
	}
	for _, part := range strings.Split(key, "/") {
		if !contract.SafeKeySegment(part) {
			return fmt.Errorf("upload s3: unsafe object key")
		}
	}
	request, err := s.request(http.MethodDelete, key, nil, "application/octet-stream")
	if err != nil {
		return err
	}
	return s.execute(request)
}

func (s *Store) request(method, key string, payload []byte, contentType string) (*http.Request, error) {
	objectURL := *s.endpoint
	objectURL.Path = "/" + s.options.Bucket + "/" + key
	request, err := http.NewRequest(method, objectURL.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("upload s3: create request: %w", err)
	}
	request.Header.Set("Content-Type", contentType)
	s.sign(request, payload, s.now().UTC())
	return request, nil
}

func (s *Store) execute(request *http.Request) error {
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("upload s3: storage request failed: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("upload s3: storage returned HTTP %d", response.StatusCode)
	}
	return nil
}

func (s *Store) sign(request *http.Request, payload []byte, now time.Time) {
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	payloadHash := sha256.Sum256(payload)
	checksum := hex.EncodeToString(payloadHash[:])
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", checksum)
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "content-type:" + strings.TrimSpace(request.Header.Get("Content-Type")) + "\n" +
		"host:" + request.URL.Host + "\n" +
		"x-amz-content-sha256:" + checksum + "\n" +
		"x-amz-date:" + amzDate + "\n"
	canonicalRequest := strings.Join([]string{request.Method, request.URL.EscapedPath(), "", canonicalHeaders, signedHeaders, checksum}, "\n")
	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	scope := date + "/" + s.options.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])
	dateKey := hmacSHA256([]byte("AWS4"+s.options.SecretAccessKey), date)
	regionKey := hmacSHA256(dateKey, s.options.Region)
	serviceKey := hmacSHA256(regionKey, "s3")
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.options.AccessKeyID+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func parseHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || parsed.ForceQuery {
		return nil, fmt.Errorf("invalid HTTP URL")
	}
	for _, part := range strings.Split(parsed.Path, "/") {
		if part == "." || part == ".." || strings.ContainsAny(part, "\\\r\n") {
			return nil, fmt.Errorf("unsafe URL path")
		}
	}
	return parsed, nil
}

func validBucket(bucket string) bool {
	if len(bucket) < 3 || len(bucket) > 63 || bucket[0] == '-' || bucket[len(bucket)-1] == '-' {
		return false
	}
	for _, character := range bucket {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}
