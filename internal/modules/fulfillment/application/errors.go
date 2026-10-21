package application

import (
	"errors"

	productcontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/catalog/product/contract"
	orderapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/application"
)

var (
	ErrFulfillmentInvalid      = productcontract.ErrFulfillmentInvalid
	ErrFulfillmentExists       = errors.New("fulfillment exists")
	ErrFulfillmentCreateFailed = errors.New("fulfillment create failed")
	ErrFulfillmentNotAuto      = errors.New("fulfillment not auto")
	ErrOrderNotFound           = orderapp.ErrOrderNotFound
	ErrOrderFetchFailed        = orderapp.ErrOrderFetchFailed
	ErrOrderStatusInvalid      = orderapp.ErrOrderStatusInvalid
	ErrOrderUpdateFailed       = orderapp.ErrOrderUpdateFailed
	ErrCardSecretInsufficient  = orderapp.ErrCardSecretInsufficient
)
