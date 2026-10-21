package fulfillmentwiring

import (
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/app/container"
	fulfillmenttransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/fulfillment/transport/http"
)

func NewAdminHandler(c *container.Container) *fulfillmenttransport.AdminHandler {
	return fulfillmenttransport.NewAdminHandler(
		fulfillmentManualCreatorAdapter{svc: c.FulfillmentService},
		fulfillmentAdminOrderAdapter{orders: c.OrderService},
	)
}
