package telegramauthcache

import (
	"context"
	"time"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/cache"
	telegramauthapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/telegramauth/application"
)

func Options() []telegramauthapp.Option {
	return []telegramauthapp.Option{
		telegramauthapp.WithReplaySetNX(cache.SetNX),
		telegramauthapp.WithOIDCStateStore(setOIDCState, takeOIDCState),
	}
}

func setOIDCState(ctx context.Context, key string, value string, ttlSeconds int) (bool, error) {
	return cache.SetNX(ctx, key, value, time.Duration(ttlSeconds)*time.Second)
}

func takeOIDCState(ctx context.Context, key string) (string, bool, error) {
	value, err := cache.GetString(ctx, key)
	if err != nil {
		return "", false, err
	}
	if value == "" {
		return "", false, nil
	}
	_ = cache.Del(ctx, key)
	return value, true, nil
}
