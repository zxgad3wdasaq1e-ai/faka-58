package integrationtest

import (
	"testing"

	productdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/catalog/product/domain"
	orderdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/domain"

	affiliateapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/application"
	affiliatedomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/domain"
	affiliategormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/infrastructure/gormstore"
	userstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/user/infrastructure/gormstore"

	settingsapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/application"
	settingsintegration "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/schema/integration"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/testkit/memorysettings"

	"github.com/shopspring/decimal"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/constants"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/shared/money"
)

type staticOrderReader struct{ order *orderdomain.Order }

func (r staticOrderReader) GetByID(uint) (*orderdomain.Order, error) { return r.order, nil }

type staticProductReader struct{}

func (staticProductReader) ListByIDs([]uint) ([]productdomain.Product, error) {
	return []productdomain.Product{{ID: 1, IsAffiliateEnabled: true}}, nil
}

// 佣金重试任务晚于退款执行时，退款流程已找不到佣金可冲销，重试不能再生成佣金。
func TestHandleOrderPaidSkipsRefundedOrder(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{constants.OrderStatusPaid, true},
		{constants.OrderStatusRefunded, false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			_, db, _ := setupAffiliateServiceTest(t)
			if err := db.AutoMigrate(&orderdomain.Order{}, &affiliatedomain.Commission{}); err != nil {
				t.Fatalf("migrate commission: %v", err)
			}
			promoter := createAffiliateTestUser(t, db, "promoter@example.com")
			profile := createAffiliateTestProfile(t, db, promoter.ID, "PROMO1", constants.AffiliateProfileStatusActive)

			order := &orderdomain.Order{
				ID: 10, UserID: promoter.ID + 1, Status: tc.status, AffiliateProfileID: &profile.ID,
				OrderNo: "COMMISSION-RETRY", Currency: "CNY", TotalAmount: money.FromDecimal(decimal.NewFromInt(100)),
				Items: []orderdomain.OrderItem{{ProductID: 1, TotalPrice: money.FromDecimal(decimal.NewFromInt(100))}},
			}
			if err := db.Omit("Items").Create(order).Error; err != nil {
				t.Fatalf("create order: %v", err)
			}
			store := affiliategormstore.New(db)
			settings := settingsapp.NewService(memorysettings.New())
			if _, err := settings.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{Enabled: true, CommissionRate: 20}); err != nil {
				t.Fatalf("init affiliate setting: %v", err)
			}
			svc := affiliateapp.NewService(store, userstore.New(db), staticOrderReader{order}, staticProductReader{}, settings)
			if err := svc.HandleOrderPaid(order.ID); err != nil {
				t.Fatalf("HandleOrderPaid: %v", err)
			}
			got, err := store.GetCommissionByOrderAndProfile(order.ID, profile.ID, constants.AffiliateCommissionTypeOrder)
			if err != nil {
				t.Fatalf("load commission: %v", err)
			}
			if (got != nil) != tc.want {
				t.Fatalf("status %s: commission created=%v, want %v", tc.status, got != nil, tc.want)
			}
		})
	}
}

type refundDuringProductRead struct{ refund func() }

func (r refundDuringProductRead) ListByIDs([]uint) ([]productdomain.Product, error) {
	r.refund()
	return []productdomain.Product{{ID: 1, IsAffiliateEnabled: true}}, nil
}

func TestHandleOrderPaidRechecksRefundCommittedAfterInitialOrderRead(t *testing.T) {
	for _, refundAmount := range []string{"40", "100"} {
		t.Run(refundAmount, func(t *testing.T) {
			f := newAffiliateRefundFixture(t)
			order := f.order(t, "100", nil, false)
			order.Items = []orderdomain.OrderItem{{ProductID: 1, TotalPrice: amount("100")}}
			settings := settingsapp.NewService(memorysettings.New())
			if _, err := settings.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{Enabled: true, CommissionRate: 20}); err != nil {
				t.Fatal(err)
			}
			svc := affiliateapp.NewService(f.store, nil, staticOrderReader{&order}, refundDuringProductRead{func() {
				f.refund(t, order.ID, refundAmount)
			}}, settings)
			if err := svc.HandleOrderPaid(order.ID); err != nil {
				t.Fatal(err)
			}
			rows, err := f.store.ListCommissionsByOrder(order.ID, nil)
			if err != nil {
				t.Fatal(err)
			}
			if refundAmount == "40" {
				if len(rows) != 1 || !rows[0].CommissionAmount.Decimal.Equal(decimal.NewFromInt(12)) || !rows[0].OrderRefundedAmount.Decimal.Equal(decimal.NewFromInt(40)) {
					t.Fatalf("delayed commission ignored partial refund: %+v %v", rows, err)
				}
			} else if len(rows) != 0 {
				t.Fatal("delayed job recreated commission after full refund")
			}
			f.assertWalletsUnchanged(t)
		})
	}
}
