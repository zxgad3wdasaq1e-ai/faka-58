package redislimiter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/cache"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/logger"
	orderriskcontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/orderrisk/contract"
	settingssecurity "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/schema/security"

	"github.com/redis/go-redis/v9"
)

var orderRateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[1])
end
if tonumber(ARGV[2]) > 0 and tonumber(ARGV[3]) > 0 and current == tonumber(ARGV[2]) + 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[3])
end
local ttl = redis.call("TTL", KEYS[1])
return {current, ttl}
`)

// Limiter 使用 Redis 固定窗口计数；Redis 不可用时以有界本地计数继续保护当前实例。
type Limiter struct {
	local localLimiter
}

var _ orderriskcontract.RateLimiter = (*Limiter)(nil)

func New() *Limiter { return &Limiter{} }

func (l *Limiter) Check(input orderriskcontract.CheckInput, config settingssecurity.OrderRateLimitConfig) error {
	if !config.Enabled || config.WindowSeconds <= 0 || config.MaxRequests <= 0 {
		return nil
	}
	keys := rateKeys(input)
	if len(keys) == 0 {
		return nil
	}
	return l.checkKeys(cache.Client(), keys, config)
}

func rateKeys(input orderriskcontract.CheckInput) []string {
	if input.IsGuest {
		if input.SkipIPCheck || input.RiskIP == "" {
			return nil
		}
		return []string{"dj:risk:order_rate:guest_ip:" + input.RiskIP}
	}
	keys := make([]string, 0, 2)
	if input.UserID > 0 {
		keys = append(keys, fmt.Sprintf("dj:risk:order_rate:user:%d", input.UserID))
	}
	// 按 IP 汇总不同账号，避免轮换小号绕过单账号频率限制。
	if !input.SkipIPCheck && input.RiskIP != "" {
		keys = append(keys, "dj:risk:order_rate:member_ip:"+input.RiskIP)
	}
	return keys
}

func (l *Limiter) checkKeys(client *redis.Client, keys []string, config settingssecurity.OrderRateLimitConfig) error {
	if client == nil || !l.local.allowRedis(time.Now()) {
		return l.local.check(keys, config, time.Now())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, key := range keys {
		if err := checkSingle(ctx, client, key, config); err != nil {
			if errors.Is(err, orderriskcontract.ErrOrderRateLimited) {
				l.local.finishRedisAttempt(true, time.Now())
				return err
			}
			l.local.finishRedisAttempt(false, time.Now())
			if l.local.shouldWarn(time.Now()) {
				logger.Warnw("risk_control_rate_limit_local_fallback", "error", err)
			}
			return l.local.check(keys, config, time.Now())
		}
	}
	l.local.finishRedisAttempt(true, time.Now())
	return nil
}

func checkSingle(ctx context.Context, client *redis.Client, key string, config settingssecurity.OrderRateLimitConfig) error {
	result, err := orderRateLimitScript.Run(ctx, client, []string{key},
		config.WindowSeconds, config.MaxRequests, config.BlockSeconds,
	).Result()
	if err != nil {
		return err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) < 2 {
		return fmt.Errorf("unexpected order rate limit result %T", result)
	}
	current, countOK := values[0].(int64)
	ttl, ttlOK := values[1].(int64)
	if !countOK || !ttlOK {
		return errors.New("invalid order rate limit counters")
	}
	if current <= int64(config.MaxRequests) {
		return nil
	}
	if ttl < 0 {
		ttl = 0
	}
	return &orderriskcontract.RateLimitedError{RetryAfter: ttl}
}

const localMaxEntries = 10000

type localEntry struct {
	count     int
	expiresAt time.Time
}

type localLimiter struct {
	mu                 sync.Mutex
	entries            map[string]localEntry
	lastPrune          time.Time
	lastWarn           time.Time
	redisRetryAfter    time.Time
	redisProbeInFlight bool
}

func (l *localLimiter) check(keys []string, cfg settingssecurity.OrderRateLimitConfig, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]localEntry)
	}
	// 饱和后每秒最多扫描一次，避免高基数攻击反复触发全表清理。
	if len(l.entries) >= localMaxEntries && now.Sub(l.lastPrune) >= time.Second {
		for key, entry := range l.entries {
			if !now.Before(entry.expiresAt) {
				delete(l.entries, key)
			}
		}
		l.lastPrune = now
	}
	for _, key := range keys {
		entry, exists := l.entries[key]
		if exists && !now.Before(entry.expiresAt) {
			delete(l.entries, key)
			exists = false
		}
		if !exists {
			if len(l.entries) >= localMaxEntries {
				// 不淘汰有效计数器；容量耗尽时拒绝新身份，避免绕过已有封禁。
				return &orderriskcontract.RateLimitedError{RetryAfter: int64(cfg.WindowSeconds)}
			}
			entry = localEntry{expiresAt: now.Add(time.Duration(cfg.WindowSeconds) * time.Second)}
		}
		if entry.count <= cfg.MaxRequests {
			entry.count++
			if entry.count == cfg.MaxRequests+1 && cfg.BlockSeconds > 0 {
				entry.expiresAt = now.Add(time.Duration(cfg.BlockSeconds) * time.Second)
			}
		}
		l.entries[key] = entry
		if entry.count > cfg.MaxRequests {
			return &orderriskcontract.RateLimitedError{RetryAfter: int64(math.Ceil(entry.expiresAt.Sub(now).Seconds()))}
		}
	}
	return nil
}

func (l *localLimiter) shouldWarn(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.lastWarn.IsZero() && now.Sub(l.lastWarn) < time.Minute {
		return false
	}
	l.lastWarn = now
	return true
}

// Keep serving through the bounded local limiter while Redis is unavailable;
// only one request retries Redis after each one-second cooldown.
func (l *localLimiter) allowRedis(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.redisRetryAfter.IsZero() {
		return true
	}
	if now.Before(l.redisRetryAfter) || l.redisProbeInFlight {
		return false
	}
	l.redisProbeInFlight = true
	return true
}

func (l *localLimiter) finishRedisAttempt(succeeded bool, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.redisProbeInFlight = false
	if succeeded {
		l.redisRetryAfter = time.Time{}
	} else {
		l.redisRetryAfter = now.Add(time.Second)
	}
}
