package gormstore

import (
	"time"

	paymentdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/payment/domain"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/constants"
	dashboard "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/dashboard/contract"

	"gorm.io/gorm"
)

// Store is the GORM adapter for dashboard persistence ports.
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func paidOrderStatuses() []string {
	return []string{
		constants.OrderStatusPaid,
		constants.OrderStatusFulfilling,
		constants.OrderStatusPartiallyDelivered,
		constants.OrderStatusPartiallyRefunded,
		constants.OrderStatusDelivered,
		constants.OrderStatusCompleted,
	}
}

func onlinePaymentBase(db *gorm.DB, startAt, endAt time.Time) *gorm.DB {
	timeWhere, timeArgs := timeRangeQuery(db, "payments.created_at", startAt, endAt)
	args := append(timeArgs, constants.PaymentProviderWallet)
	return db.Model(&paymentdomain.Payment{}).
		Where("payments.deleted_at IS NULL").
		Where(timeWhere+" AND payments.provider_type <> ?", args...)
}

var _ dashboard.Repository = (*Store)(nil)
