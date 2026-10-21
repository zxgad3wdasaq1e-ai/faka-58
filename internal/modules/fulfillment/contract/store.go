package contract

import fulfillmentdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/fulfillment/domain"

// Store 是交付记录持久化端口。
type Store interface {
	Create(fulfillment *fulfillmentdomain.Fulfillment) error
	GetByOrderID(orderID uint) (*fulfillmentdomain.Fulfillment, error)
	FindByOrderIDForUpdate(orderID uint) (*fulfillmentdomain.Fulfillment, bool, error)
}
