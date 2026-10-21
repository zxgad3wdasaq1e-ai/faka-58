package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/config"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/i18n"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/platform/http/response"
)

const callbackBodyValidatedKey = "callback_body_validated"

// PublicRequestProtectionMiddleware protects anonymous reads and login attempts
// before tenant resolution, JSON parsing or other database-backed handlers run.
func PublicRequestProtectionMiddleware(client *redis.Client, prefix string, cfg config.SecurityConfig) gin.HandlerFunc {
	public := RateLimitGuard(client, requestRateLimitRule(prefix+":public", cfg.PublicRateLimit.WithDefaults(60, 300, 60)), KeyByIP)
	click := RateLimitGuard(client, requestRateLimitRule(prefix+":affiliate_click", cfg.AffiliateClickRateLimit.WithDefaults(60, 20, 300)), KeyByIP)
	captcha := RateLimitGuard(client, requestRateLimitRule(prefix+":captcha", cfg.CaptchaRateLimit.WithDefaults(60, 20, 60)), KeyByIP)
	login := RateLimitGuard(client, requestRateLimitRule(prefix+":login_ip", cfg.LoginIPRateLimit.WithDefaults(300, 30, 900)), KeyByIP)
	channel := RateLimitGuard(client, requestRateLimitRule(prefix+":channel_ip", cfg.ChannelIPRateLimit.WithDefaults(60, 1200, 60)), KeyByIP)
	upstream := RateLimitGuard(client, requestRateLimitRule(prefix+":upstream_ip", cfg.UpstreamIPRateLimit.WithDefaults(60, 120, 60)), KeyByIP)
	return func(c *gin.Context) {
		if c.GetBool(callbackBodyValidatedKey) {
			return
		}
		requestPath := strings.TrimRight(c.Request.URL.Path, "/")
		if strings.HasPrefix(requestPath, "/api/v1/public/") {
			public(c)
			if c.IsAborted() {
				return
			}
			switch requestPath {
			case "/api/v1/public/affiliate/click":
				click(c)
			case "/api/v1/public/captcha/image":
				captcha(c)
			}
		} else if strings.HasPrefix(requestPath, "/api/v1/auth/") {
			// Shared across email, 2FA and third-party login endpoints. Existing
			// account and sensitive-operation limits remain separately enforced.
			login(c)
		} else if strings.HasPrefix(requestPath, "/api/v1/channel/") {
			channel(c)
		} else if strings.HasPrefix(requestPath, "/api/v1/upstream/") {
			upstream(c)
		}
	}
}

func requestRateLimitRule(prefix string, cfg config.RequestRateLimitConfig) RateLimitRule {
	return RateLimitRule{Prefix: prefix, WindowSeconds: cfg.WindowSeconds, MaxRequests: cfg.MaxRequests,
		BlockSeconds: cfg.BlockSeconds, MessageKey: "error.rate_limited"}
}

// RequestBodyLimitMiddleware bounds ordinary API requests before any JSON key
// extraction. Signed callback bodies are bounded separately by their dispatcher.
// Upload and CSV import bodies remain streamed so authentication happens before
// allocating multipart payloads; their handlers retain file-level validation.
func RequestBodyLimitMiddleware(uploadMaxBytes int64) gin.HandlerFunc {
	if uploadMaxBytes <= 0 {
		uploadMaxBytes = 10 << 20
	}
	return func(c *gin.Context) {
		requestPath := strings.TrimRight(c.Request.URL.Path, "/")
		if !strings.HasPrefix(requestPath, "/api/") || c.GetBool(callbackBodyValidatedKey) {
			return
		}
		maxBytes := int64(128 << 10)
		if strings.HasPrefix(requestPath, "/api/v1/admin/") || strings.HasPrefix(requestPath, "/api/v1/channel/") || strings.HasPrefix(requestPath, "/api/v1/upstream/") {
			maxBytes = 1 << 20
		}
		stream := false
		switch requestPath {
		case "/api/v1/admin/upload":
			maxBytes = uploadMaxBytes + (1 << 20)
			stream = true
		case "/api/v1/admin/card-secrets/import":
			maxBytes = 12 << 20
			stream = true
		case "/api/v1/admin/card-secrets/batch":
			maxBytes = 12 << 20
		}
		limitRequestBody(c, maxBytes, stream)
	}
}

func limitRequestBody(c *gin.Context, maxBytes int64, stream bool) bool {
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return true
	}
	tooLarge := func() bool {
		message := i18n.T(i18n.ResolveLocale(c), "error.request_too_large")
		if isChannelAPIRequest(c) {
			response.ChannelError(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, message, "request_too_large")
		} else {
			response.ErrorWithHTTPStatus(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, message)
		}
		c.Abort()
		return false
	}
	if c.Request.ContentLength > maxBytes {
		return tooLarge()
	}
	if stream {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		return true
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBytes+1))
	if int64(len(body)) > maxBytes {
		return tooLarge()
	}
	if err != nil {
		response.ErrorWithHTTPStatus(c, http.StatusBadRequest, response.CodeBadRequest, i18n.T(i18n.ResolveLocale(c), "error.bad_request"))
		return false
	}
	// Preserve the exact bytes used for webhook/API signatures and later binding.
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return true
}
