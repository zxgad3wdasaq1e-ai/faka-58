package refund

import orderapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/application"

var (
	ErrOrderFetchFailed         = orderapp.ErrOrderFetchFailed
	ErrOrderNotFound            = orderapp.ErrOrderNotFound
	ErrOrderRefundExpired       = orderapp.ErrOrderRefundExpired
	ErrOrderStatusInvalid       = orderapp.ErrOrderStatusInvalid
	ErrOrderUpdateFailed        = orderapp.ErrOrderUpdateFailed
	ErrRefundRecordCreateFailed = orderapp.ErrRefundRecordCreateFailed
)
