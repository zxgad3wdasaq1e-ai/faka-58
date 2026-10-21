package contract

import (
	"context"
	"encoding/json"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/adproxy/domain"
)

// Gateway 是应用层访问广告网关所需的端口。
type Gateway interface {
	RenderSlot(ctx context.Context, slotCode string, params map[string]string) (*domain.RenderResponse, error)
	ReportImpression(ctx context.Context, payload json.RawMessage) error
}
