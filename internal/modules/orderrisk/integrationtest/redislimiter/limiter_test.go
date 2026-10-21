package redislimiter_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/cache"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/config"
	orderriskcontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/orderrisk/contract"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/orderrisk/infrastructure/redislimiter"
	settingssecurity "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/schema/security"
)

func TestLocalLimiterSerializesRequestsAndExpiresBlock(t *testing.T) {
	if err := cache.InitRedis(nil); err != nil {
		t.Fatal(err)
	}
	limiter := redislimiter.New()
	cfg := settingssecurity.DefaultOrderRiskControlConfig().Guest.RateLimit
	cfg.WindowSeconds = 1
	cfg.BlockSeconds = 1
	input := orderriskcontract.CheckInput{IsGuest: true, RiskIP: "1.2.3.4"}
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if err := limiter.Check(input, cfg); err == nil {
				allowed.Add(1)
			} else if !errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
				t.Errorf("unexpected limit error: %v", err)
			}
		})
	}
	wg.Wait()
	if allowed.Load() != int64(cfg.MaxRequests) {
		t.Fatalf("concurrent requests allowed=%d want=%d", allowed.Load(), cfg.MaxRequests)
	}
	if err := limiter.Check(input, cfg); !errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
		t.Fatalf("block must not reset early: %v", err)
	}
	time.Sleep(1050 * time.Millisecond)
	if err := limiter.Check(input, cfg); err != nil {
		t.Fatalf("block should expire: %v", err)
	}
}

func useFailingRedis(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("-ERR synthetic redis unavailable\r\n"))
			_ = conn.Close()
		}
	}()
	if err := cache.InitRedis(&config.RedisConfig{
		Enabled: true, Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cache.Client().Close()
		_ = cache.InitRedis(nil)
	})
}

func TestLimiterFallsBackWhenRedisIsUnavailableAndLimitsAcrossAccounts(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis_error_%t", unavailable), func(t *testing.T) {
			if unavailable {
				useFailingRedis(t)
			} else if err := cache.InitRedis(nil); err != nil {
				t.Fatal(err)
			}
			limiter := redislimiter.New()
			cfg := settingssecurity.DefaultOrderRiskControlConfig().Member.RateLimit
			cfg.MaxRequests = 2
			for i := uint(1); i <= 3; i++ {
				err := limiter.Check(orderriskcontract.CheckInput{UserID: i, RiskIP: "1.2.3.4"}, cfg)
				if i <= 2 && err != nil {
					t.Fatalf("ordinary request failed: %v", err)
				}
				if i == 3 && !errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
					t.Fatalf("rotating accounts must retain the IP limit: %v", err)
				}
			}
			input := orderriskcontract.CheckInput{UserID: 4, RiskIP: "1.2.3.4", SkipIPCheck: true}
			if err := limiter.Check(input, cfg); err != nil {
				t.Fatalf("trusted bot requests should not share a server IP limit: %v", err)
			}
			if err := limiter.Check(input, cfg); err != nil {
				t.Fatal(err)
			}
			if err := limiter.Check(input, cfg); !errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
				t.Fatalf("trusted bot must retain per-user frequency protection: %v", err)
			}
		})
	}
}

func TestLocalLimiterBoundsIdentityStateWithoutEvictingCounters(t *testing.T) {
	if err := cache.InitRedis(nil); err != nil {
		t.Fatal(err)
	}
	limiter := redislimiter.New()
	cfg := settingssecurity.DefaultOrderRiskControlConfig().Member.RateLimit
	cfg.MaxRequests = 1
	// 本地兜底容量为 10000；新身份不能逐出现有计数器以规避限流。
	for i := uint(1); i <= 10000; i++ {
		if err := limiter.Check(orderriskcontract.CheckInput{UserID: i, SkipIPCheck: true}, cfg); err != nil {
			t.Fatal(err)
		}
	}
	if err := limiter.Check(orderriskcontract.CheckInput{UserID: 10001, SkipIPCheck: true}, cfg); !errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
		t.Fatalf("full fallback must reject new identities: %v", err)
	}
	if err := limiter.Check(orderriskcontract.CheckInput{UserID: 1, SkipIPCheck: true}, cfg); !errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
		t.Fatalf("existing counters must not be evicted: %v", err)
	}
}

func TestRedisFailureCooldownProtectsConcurrentOrderChecks(t *testing.T) {
	if err := cache.InitRedis(&config.RedisConfig{Enabled: true, Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	client := cache.Client()
	t.Cleanup(func() { _ = client.Close(); _ = cache.InitRedis(nil) })
	hook := &orderRedisFailureHook{}
	client.AddHook(hook)
	limiter := redislimiter.New()
	cfg := settingssecurity.DefaultOrderRiskControlConfig().Member.RateLimit
	cfg.MaxRequests = 2
	// The first failure opens the cooldown before a concurrent burst arrives.
	if err := limiter.Check(orderriskcontract.CheckInput{UserID: 1, RiskIP: "203.0.113.31"}, cfg); err != nil {
		t.Fatal(err)
	}
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for index := uint(2); index <= 20; index++ {
		wg.Go(func() {
			err := limiter.Check(orderriskcontract.CheckInput{UserID: index, RiskIP: "203.0.113.31"}, cfg)
			if err == nil {
				allowed.Add(1)
			} else if !errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
				t.Errorf("unexpected check error: %v", err)
			}
		})
	}
	wg.Wait()
	if allowed.Load() != 1 {
		t.Fatalf("local limit allowed %d burst requests, want 1", allowed.Load())
	}
	if hook.calls.Load() != 1 {
		t.Fatalf("concurrent requests retried failed Redis %d times, want 1", hook.calls.Load())
	}
}

type orderRedisFailureHook struct{ calls atomic.Int64 }

func (*orderRedisFailureHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *orderRedisFailureHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(context.Context, redis.Cmder) error { h.calls.Add(1); return errors.New("test Redis unavailable") }
}
func (*orderRedisFailureHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
