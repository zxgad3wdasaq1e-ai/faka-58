package application_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	orderriskapp "github.com/dujiao-next/internal/modules/orderrisk/application"
	orderriskcontract "github.com/dujiao-next/internal/modules/orderrisk/contract"
	orderriskdomain "github.com/dujiao-next/internal/modules/orderrisk/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

func inventoryRiskFixture(t *testing.T, name string, secrets int) (couponLimitRaceFixture, *orderapp.OrderService) {
	t.Helper()
	f := newCouponLimitRaceFixture(t, name, 0, 0)
	if err := f.db.AutoMigrate(
		&settingsstore.SettingRecord{}, &orderriskdomain.LockKey{},
		&cardsecretdomain.Batch{}, &cardsecretdomain.Secret{}, &paymentdomain.Payment{},
	); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&productdomain.Product{}).Where("id = ?", f.product.ID).Updates(map[string]interface{}{
		"purchase_type":    constants.ProductPurchaseGuest,
		"fulfillment_type": constants.FulfillmentTypeAuto,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for i := range secrets {
		if err := f.db.Create(&cardsecretdomain.Secret{
			ProductID: f.product.ID, SKUID: f.sku.ID, Secret: fmt.Sprintf("synthetic-test-card-%d", i),
			Status: cardsecretdomain.StatusAvailable,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	settings := settingsapp.NewService(settingsstore.New(f.db))
	risk := orderriskapp.NewService(orderriskapp.Options{Settings: settings})
	svc := orderapp.NewOrderService(orderapp.OrderServiceOptions{
		OrderStore:   orderstore.New(f.db, "inventory-risk-test-guest-credential-secret"),
		ProductStore: productstore.NewProductStore(f.db), ProductSKUStore: productstore.NewSKUStore(f.db),
		Queue: f.queue, RiskControlService: risk, ExpireMinutes: 30, UniformPricing: true,
	})
	return f, svc
}

func riskOrderItems(f couponLimitRaceFixture, quantity int) []orderapp.CreateOrderItem {
	return []orderapp.CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: quantity}}
}

type inventoryCreateResult struct {
	order *orderdomain.Order
	err   error
}

func concurrentInventoryCreates(t *testing.T, requests, wantSuccess int, create func(int) (*orderdomain.Order, error)) []*orderdomain.Order {
	t.Helper()
	start := make(chan struct{})
	results := make(chan inventoryCreateResult, requests)
	for index := range requests {
		go func() {
			<-start
			order, err := create(index)
			results <- inventoryCreateResult{order: order, err: err}
		}()
	}
	close(start)
	orders := make([]*orderdomain.Order, 0, wantSuccess)
	for range requests {
		select {
		case result := <-results:
			if result.err == nil {
				orders = append(orders, result.order)
			} else if !errors.Is(result.err, orderriskcontract.ErrTooManyPendingOrders) {
				t.Errorf("unexpected concurrent order error: %v", result.err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("inventory quota check blocked with SQLite MaxOpenConns=1")
		}
	}
	if len(orders) != wantSuccess {
		t.Fatalf("created %d orders, want exactly %d", len(orders), wantSuccess)
	}
	return orders
}

func assertInventoryReservations(t *testing.T, f couponLimitRaceFixture, wantParents, wantReserved int64) {
	t.Helper()
	var parents, reserved int64
	if err := f.db.Model(&orderdomain.Order{}).Where("parent_id IS NULL AND status = ?", constants.OrderStatusPendingPayment).Count(&parents).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&cardsecretdomain.Secret{}).Where("status = ?", cardsecretdomain.StatusReserved).Count(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if parents != wantParents || reserved != wantReserved {
		t.Fatalf("pending parents=%d reserved cards=%d; want %d/%d", parents, reserved, wantParents, wantReserved)
	}
}

func TestDefaultGuestInventoryQuotaIsAtomicAndTimeoutReleasesCards(t *testing.T) {
	f, svc := inventoryRiskFixture(t, "guest-inventory-risk", 20)
	create := func(index int) (*orderdomain.Order, error) {
		return svc.CreateGuestOrder(orderapp.CreateGuestOrderInput{
			Email: fmt.Sprintf("guest-%d@example.com", index), OrderPassword: "test-guest-password",
			ClientIP: "2001:db8:1:2::abcd", Items: riskOrderItems(f, 1),
		})
	}
	orders := concurrentInventoryCreates(t, 12, 2, create)
	assertInventoryReservations(t, f, 2, 2)
	for _, order := range orders {
		if order.ExpiresAt == nil || order.ExpiresAt.Sub(order.CreatedAt) != 10*time.Minute {
			t.Fatalf("guest reservation should expire in 10 minutes: %+v", order.ExpiresAt)
		}
	}
	// 到期取消仍走真实订单释放流程，恢复 IP 配额与对应卡密。
	expired := time.Now().Add(-time.Minute)
	if err := f.db.Model(&orderdomain.Order{}).Where("id = ?", orders[0].ID).Update("expires_at", expired).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelExpiredOrder(orders[0].ID); err != nil {
		t.Fatalf("cancel expired reservation: %v", err)
	}
	assertInventoryReservations(t, f, 1, 1)
	if _, err := create(100); err != nil {
		t.Fatalf("released quota must allow a new purchase: %v", err)
	}
	assertInventoryReservations(t, f, 2, 2)
}

func TestDefaultMemberIPQuotaLimitsConcurrentAccountRotation(t *testing.T) {
	f, svc := inventoryRiskFixture(t, "member-inventory-risk", 30)
	users := make([]userdomain.User, 12)
	for index := range users {
		users[index] = userdomain.User{Email: fmt.Sprintf("rotating-%d@example.com", index), PasswordHash: "hash", Status: "active"}
	}
	if err := f.db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	concurrentInventoryCreates(t, len(users), 10, func(index int) (*orderdomain.Order, error) {
		return svc.CreateOrder(orderapp.CreateOrderInput{
			UserID: users[index].ID, ClientIP: "1.2.3.4", Items: riskOrderItems(f, 1),
		})
	})
	assertInventoryReservations(t, f, 10, 10)
}

func TestInventoryFailureRollsBackParentChildrenAndReservations(t *testing.T) {
	f, svc := inventoryRiskFixture(t, "rollback-inventory-risk", 1)
	if _, err := svc.CreateOrder(orderapp.CreateOrderInput{
		UserID: f.users[0].ID, ClientIP: "1.2.3.4", Items: riskOrderItems(f, 2),
	}); !errors.Is(err, orderapp.ErrCardSecretInsufficient) {
		t.Fatalf("expected insufficient card inventory: %v", err)
	}
	assertInventoryReservations(t, f, 0, 0)
	var count int64
	if err := f.db.Model(&orderdomain.Order{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed create must leave no parent or child: count=%d err=%v", count, err)
	}
}

func TestMemberBatchQuantityAndTrustedBotRetainUserProtection(t *testing.T) {
	f, svc := inventoryRiskFixture(t, "bot-inventory-risk", 40)
	// Bot 共享服务器 IP，因此跳过 IP 配额，但仍接受同一用户的待付与单品限制。
	input := orderapp.CreateOrderInput{
		UserID: f.users[0].ID, SkipIPRiskControl: true, Items: riskOrderItems(f, 5),
	}
	for range 5 {
		if _, err := svc.CreateOrder(input); err != nil {
			t.Fatalf("member should be able to buy five cards per order: %v", err)
		}
	}
	if _, err := svc.CreateOrder(input); !errors.Is(err, orderriskcontract.ErrTooManyPendingOrders) {
		t.Fatalf("trusted bot must enforce user pending quota: %v", err)
	}
	input.UserID = f.users[1].ID
	input.Items = riskOrderItems(f, 6)
	if _, err := svc.CreateOrder(input); !errors.Is(err, orderriskcontract.ErrProductQuantityLimit) {
		t.Fatalf("trusted bot must enforce per-product quantity: %v", err)
	}
	assertInventoryReservations(t, f, 5, 25)
}

func TestExplicitlyDisabledRiskAllowsLargerPurchase(t *testing.T) {
	f, svc := inventoryRiskFixture(t, "disabled-inventory-risk", 10)
	cfg := settingssecurity.DefaultOrderRiskControlConfig()
	cfg.Enabled = false
	if err := f.db.Create(&settingsstore.SettingRecord{
		Key: constants.SettingKeyOrderRiskControlConfig, ValueJSON: settingssecurity.EncodeOrderRiskControlConfig(cfg),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateOrder(orderapp.CreateOrderInput{
		UserID: f.users[0].ID, Items: riskOrderItems(f, 6),
	}); err != nil {
		t.Fatalf("explicit disable must remain authoritative: %v", err)
	}
	assertInventoryReservations(t, f, 1, 6)
}
