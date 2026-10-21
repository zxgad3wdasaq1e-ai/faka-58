package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	captchacontract "github.com/dujiao-next/internal/modules/captcha/contract"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userauthhttp "github.com/dujiao-next/internal/modules/identity/userauth/transport/http"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	paymenthttp "github.com/dujiao-next/internal/modules/payment/transport/http"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/gin-gonic/gin"
)

func TestPublicProtectionLimitsBeforeHandlersAndSeparatesSensitiveBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.SecurityConfig{
		PublicRateLimit:         config.RequestRateLimitConfig{MaxRequests: 5},
		AffiliateClickRateLimit: config.RequestRateLimitConfig{MaxRequests: 1},
		CaptchaRateLimit:        config.RequestRateLimitConfig{MaxRequests: 1},
	}
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(PublicRequestProtectionMiddleware(nil, "test:public", cfg))
	clicks := 0
	public := router.Group("/api/v1/public")
	public.POST("/affiliate/click", func(c *gin.Context) { clicks++; c.Status(http.StatusNoContent) })
	captcha := &protectionCaptchaStub{}
	captchahttp.RegisterPublicRoutes(public, captchahttp.NewPublicHandler(captcha))
	public.GET("/products", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, test := range []struct {
		method, path string
		status       int
	}{
		{http.MethodPost, "/api/v1/public/affiliate/click", 204},
		{http.MethodPost, "/api/v1/public/affiliate/click", 429},
		{http.MethodGet, "/api/v1/public/captcha/image", 200},
		{http.MethodGet, "/api/v1/public/captcha/image", 429},
		{http.MethodGet, "/api/v1/public/products", 204},
		{http.MethodGet, "/api/v1/public/products", 429},
	} {
		got := protectionRequest(router, test.method, test.path, "203.0.113.7", nil)
		if got.Code != test.status {
			t.Fatalf("%s %s status=%d want=%d body=%s", test.method, test.path, got.Code, test.status, got.Body.String())
		}
	}
	if clicks != 1 || captcha.calls != 1 {
		t.Fatalf("limited requests reached expensive handlers: clicks=%d captcha=%d", clicks, captcha.calls)
	}
	if got := protectionRequest(router, http.MethodGet, "/api/v1/public/products", "203.0.113.8", nil); got.Code != http.StatusNoContent {
		t.Fatalf("independent source IP limited: %d", got.Code)
	}
}

func TestLoginProtectionDifferentEmailsShareIPLimitAndRejectLargeBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(PublicRequestProtectionMiddleware(nil, "test:login", config.SecurityConfig{LoginIPRateLimit: config.RequestRateLimitConfig{MaxRequests: 2}}))
	router.Use(RequestBodyLimitMiddleware(0))
	auth := &protectionLoginStub{}
	handler := userauthhttp.NewUserLoginHandler(auth, auth, nil, nil)
	userauthhttp.RegisterUserLoginAuthRoutes(router.Group("/api/v1/auth"), handler,
		RateLimitMiddleware(nil, RateLimitRule{WindowSeconds: 60, MaxRequests: 1}, KeyByIPAndJSONField("email")))
	for index := 0; index < 3; index++ {
		body := fmt.Sprintf(`{"email":"user%d@example.com","password":"bad-password"}`, index)
		got := protectionRequest(router, http.MethodPost, "/api/v1/auth/login", "203.0.113.9", strings.NewReader(body))
		want := http.StatusOK
		if index == 2 {
			want = http.StatusTooManyRequests
		}
		if got.Code != want {
			t.Fatalf("attempt %d status=%d want=%d body=%s", index, got.Code, want, got.Body.String())
		}
	}
	if auth.calls != 2 {
		t.Fatalf("authentication called %d times, want 2", auth.calls)
	}
	// Unknown/chunked content length must also be bounded before key extraction.
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"x@example.com","password":"`+strings.Repeat("x", 128<<10)+`"}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = -1
	got := httptest.NewRecorder()
	router.ServeHTTP(got, request)
	if got.Code != http.StatusRequestEntityTooLarge || auth.calls != 2 {
		t.Fatalf("large chunked login status=%d calls=%d body=%s", got.Code, auth.calls, got.Body.String())
	}
}

func TestCallbackRoutesShareLimitWithoutDoubleCountingAndPreserveSignedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const customPath = "/api/v1/upstream/private-stripe"
	settings := settingsapp.NewService(&protectionSettingsStub{routes: jsonmap.JSON{constants.SettingFieldStripeWebhook: customPath}})
	settings.InvalidateCallbackRoutesCache()
	t.Cleanup(settings.InvalidateCallbackRoutesCache)
	webhooks := &protectionWebhooksStub{}
	handler := paymenthttp.NewWebhookHandler(webhooks, nil)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	guard := RateLimitGuard(nil, RateLimitRule{WindowSeconds: 60, MaxRequests: 3}, KeyByIP)
	router.Use(CallbackRouteMiddleware(settings, nil, handler, nil, guard))
	router.Use(PublicRequestProtectionMiddleware(nil, "test:callback", config.SecurityConfig{UpstreamIPRateLimit: config.RequestRateLimitConfig{MaxRequests: 1}}))
	router.Use(RequestBodyLimitMiddleware(0))
	paymenthttp.RegisterWebhookRoutes(router.Group("/api/v1"), handler)
	body := []byte(` { "signed": "` + strings.Repeat("x", 256<<10) + `" } `)
	for _, path := range []string{customPath, customPath, constants.DefaultDujiaoPayWebhookPath} {
		got := protectionRequest(router, http.MethodPost, path, "203.0.113.11", bytes.NewReader(body))
		if got.Code != http.StatusOK {
			t.Fatalf("callback %s status=%d body=%s", path, got.Code, got.Body.String())
		}
		if !bytes.Equal(webhooks.body, body) {
			t.Fatal("signed callback bytes changed")
		}
	}
	got := protectionRequest(router, http.MethodPost, customPath, "203.0.113.11", bytes.NewReader(body))
	if got.Code != http.StatusTooManyRequests || webhooks.calls != 3 {
		t.Fatalf("shared limit bypassed: status=%d calls=%d", got.Code, webhooks.calls)
	}
	if got.Header().Get("Retry-After") == "" {
		t.Fatal("429 omitted Retry-After")
	}
	got = protectionRequest(router, http.MethodPost, constants.DefaultStripeWebhookPath, "203.0.113.12", bytes.NewReader(body))
	if got.Code != http.StatusNotFound || webhooks.calls != 3 {
		t.Fatalf("customized default route was not hidden: status=%d calls=%d", got.Code, webhooks.calls)
	}
	got = protectionRequest(router, http.MethodPost, customPath, "203.0.113.13", strings.NewReader(strings.Repeat("x", (1<<20)+1)))
	if got.Code != http.StatusRequestEntityTooLarge || webhooks.calls != 3 {
		t.Fatalf("oversized webhook reached handler: status=%d calls=%d", got.Code, webhooks.calls)
	}
}

func TestRequestBodyLimitsPreserveMultipartAndBulkImports(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestBodyLimitMiddleware(10 << 20))
	uploadCalls, bulkCalls := 0, 0
	router.POST("/api/v1/admin/upload", func(c *gin.Context) {
		uploadCalls++
		file, err := c.FormFile("file")
		if err != nil || file.Size != 2<<20 {
			t.Errorf("multipart corrupted: file=%v err=%v", file, err)
			c.Status(400)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.POST("/api/v1/admin/card-secrets/batch", func(c *gin.Context) {
		bulkCalls++
		body, err := io.ReadAll(c.Request.Body)
		if err != nil || len(body) != 2<<20 {
			t.Errorf("bulk body corrupted: bytes=%d err=%v", len(body), err)
		}
		c.Status(http.StatusNoContent)
	})
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	file, err := writer.CreateFormFile("file", "cards.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(file, strings.NewReader(strings.Repeat("x", 2<<20)))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/upload", &multipartBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	got := httptest.NewRecorder()
	router.ServeHTTP(got, request)
	if got.Code != http.StatusNoContent {
		t.Fatalf("normal upload rejected: %d %s", got.Code, got.Body.String())
	}
	got = protectionRequest(router, http.MethodPost, "/api/v1/admin/card-secrets/batch", "203.0.113.14", strings.NewReader(strings.Repeat("x", 2<<20)))
	if got.Code != http.StatusNoContent {
		t.Fatalf("normal bulk import rejected: %d", got.Code)
	}
	got = protectionRequest(router, http.MethodPost, "/api/v1/admin/upload", "203.0.113.14", strings.NewReader(strings.Repeat("x", (11<<20)+1)))
	if got.Code != http.StatusRequestEntityTooLarge || uploadCalls != 1 || bulkCalls != 1 {
		t.Fatalf("oversized upload status=%d uploadCalls=%d bulkCalls=%d", got.Code, uploadCalls, bulkCalls)
	}
}

func TestRequestBodyReadFailureAbortsBeforeHandler(t *testing.T) {
	router := gin.New()
	router.Use(RequestBodyLimitMiddleware(0))
	calls := 0
	router.POST("/api/v1/auth/login", func(c *gin.Context) { calls++; c.Status(http.StatusNoContent) })
	got := protectionRequest(router, http.MethodPost, "/api/v1/auth/login", "203.0.113.20", protectionErrorReader{})
	if got.Code != http.StatusBadRequest || calls != 0 {
		t.Fatalf("failed body read reached handler: status=%d calls=%d", got.Code, calls)
	}
}

func TestPublicProtectionIgnoresSpoofedForwardedHeadersAndSeparatesTrustedClients(t *testing.T) {
	for _, test := range []struct {
		name    string
		trusted []string
		remote  string
	}{
		{name: "direct untrusted peer", remote: "203.0.113.21"},
		{name: "trusted proxy chain", trusted: []string{"127.0.0.1/32", "192.0.2.10/32"}, remote: "127.0.0.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			if err := router.SetTrustedProxies(test.trusted); err != nil {
				t.Fatal(err)
			}
			router.Use(PublicRequestProtectionMiddleware(nil, "test:proxy", config.SecurityConfig{PublicRateLimit: config.RequestRateLimitConfig{MaxRequests: 1}}))
			router.GET("/api/v1/public/products", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
			for index, spoof := range []string{"198.51.100.1", "198.51.100.2"} {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
				request.RemoteAddr = test.remote + ":1234"
				request.Header.Set("X-Forwarded-For", spoof+", 203.0.113.22, 192.0.2.10")
				request.Header.Set("CF-Connecting-IP", spoof)
				got := httptest.NewRecorder()
				router.ServeHTTP(got, request)
				want := http.StatusOK
				if index == 1 {
					want = http.StatusTooManyRequests
				}
				if got.Code != want {
					t.Fatalf("spoofed IP bypassed limit: status=%d want=%d", got.Code, want)
				}
			}
			if len(test.trusted) > 0 {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
				request.RemoteAddr = test.remote + ":1234"
				request.Header.Set("X-Forwarded-For", "203.0.113.23, 192.0.2.10")
				got := httptest.NewRecorder()
				router.ServeHTTP(got, request)
				if got.Code != http.StatusOK || got.Body.String() != "203.0.113.23" {
					t.Fatalf("separate proxied client blocked: status=%d body=%s", got.Code, got.Body.String())
				}
			}
		})
	}
}

func TestAPIPureIPLimitsCannotBeBypassedByRotatingUnverifiedKeys(t *testing.T) {
	for _, api := range []string{"channel", "upstream"} {
		t.Run(api, func(t *testing.T) {
			router := gin.New()
			_ = router.SetTrustedProxies(nil)
			router.Use(PublicRequestProtectionMiddleware(nil, "test:api", config.SecurityConfig{
				ChannelIPRateLimit:  config.RequestRateLimitConfig{MaxRequests: 2},
				UpstreamIPRateLimit: config.RequestRateLimitConfig{MaxRequests: 2},
			}))
			calls := 0
			router.GET("/api/v1/"+api+"/products", func(c *gin.Context) { calls++; c.Status(http.StatusNoContent) })
			for index := 0; index < 3; index++ {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/"+api+"/products", nil)
				request.RemoteAddr = "203.0.113.24:1234"
				request.Header.Set("Dujiao-Next-Api-Key", fmt.Sprint(index))
				request.Header.Set("Dujiao-Next-Channel-Key", fmt.Sprint(index))
				got := httptest.NewRecorder()
				router.ServeHTTP(got, request)
				want := http.StatusNoContent
				if index == 2 {
					want = http.StatusTooManyRequests
				}
				if got.Code != want {
					t.Fatalf("rotating API key bypassed IP limit: status=%d want=%d", got.Code, want)
				}
			}
			if calls != 2 {
				t.Fatalf("handler called %d times, want 2", calls)
			}
		})
	}
}

// This is an in-process Gin protection benchmark with no network, database,
// payment provider, Redis or CDN. It cannot establish production concurrency.
func BenchmarkPublicProtectionParallel(b *testing.B) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{"allowed", "rejected"} {
		b.Run(mode, func(b *testing.B) {
			maxRequests := 1 << 30
			if mode == "rejected" {
				maxRequests = 1
			}
			router := gin.New()
			_ = router.SetTrustedProxies(nil)
			router.Use(PublicRequestProtectionMiddleware(nil, "benchmark:public", config.SecurityConfig{PublicRateLimit: config.RequestRateLimitConfig{MaxRequests: maxRequests}}))
			router.Use(RequestBodyLimitMiddleware(0))
			router.GET("/api/v1/public/products", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			if mode == "rejected" {
				protectionRequest(router, http.MethodGet, "/api/v1/public/products", "203.0.113.25", nil)
			}
			var bad atomic.Int64
			b.ReportAllocs()
			b.SetParallelism(8)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
				request.RemoteAddr = "203.0.113.25:1234"
				want := http.StatusNoContent
				if mode == "rejected" {
					want = http.StatusTooManyRequests
				}
				for pb.Next() {
					got := httptest.NewRecorder()
					router.ServeHTTP(got, request)
					if got.Code != want {
						bad.Add(1)
					}
				}
			})
			if bad.Load() > 0 {
				b.Fatalf("%d incorrect responses", bad.Load())
			}
		})
	}
}

type protectionErrorReader struct{}

func (protectionErrorReader) Read([]byte) (int, error) { return 0, errors.New("broken request body") }

func protectionRequest(router http.Handler, method, path, ip string, body io.Reader) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, body)
	request.RemoteAddr = ip + ":1234"
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

type protectionCaptchaStub struct{ calls int }

func (s *protectionCaptchaStub) GenerateImageChallenge() (*captchacontract.ImageChallenge, error) {
	s.calls++
	return &captchacontract.ImageChallenge{CaptchaID: "test"}, nil
}

type protectionLoginStub struct{ calls int }

func (*protectionLoginStub) GetRegistrationEnabled(value bool) (bool, error)      { return value, nil }
func (*protectionLoginStub) GetEmailVerificationEnabled(value bool) (bool, error) { return value, nil }
func (*protectionLoginStub) Register(context.Context, string, string, string, string, bool, bool) (*userdomain.User, string, time.Time, error) {
	panic("unexpected registration")
}
func (s *protectionLoginStub) LoginStep1(context.Context, string, string, bool) (*userauthhttp.AuthLoginResult, error) {
	s.calls++
	return nil, userauthhttp.ErrInvalidCredentials
}

type protectionSettingsStub struct{ routes jsonmap.JSON }

func (s *protectionSettingsStub) GetByKey(key string) (jsonmap.JSON, bool, error) {
	if key != constants.SettingKeyCallbackRoutesConfig {
		return nil, false, nil
	}
	return s.routes, true, nil
}
func (s *protectionSettingsStub) Upsert(_ string, value jsonmap.JSON) (jsonmap.JSON, error) {
	s.routes = value
	return value, nil
}

type protectionWebhooksStub struct {
	calls int
	body  []byte
}

func (s *protectionWebhooksStub) HandleStripeWebhook(input paymenthttp.WebhookCallbackInput) (*paymentdomain.Payment, string, error) {
	s.calls++
	s.body = input.Body
	return nil, "test", nil
}
func (s *protectionWebhooksStub) HandlePaypalWebhook(input paymenthttp.WebhookCallbackInput) (*paymentdomain.Payment, string, error) {
	return s.HandleStripeWebhook(input)
}
func (s *protectionWebhooksStub) HandleDujiaoPayWebhook(input paymenthttp.WebhookCallbackInput) (*paymentdomain.Payment, string, error) {
	return s.HandleStripeWebhook(input)
}
