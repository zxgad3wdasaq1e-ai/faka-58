package refund_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderrefund "github.com/dujiao-next/internal/modules/order/application/refund"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type refundLockTrace struct {
	rootID, changedChildID uint
	rootLocked             bool
	locks                  []uint
}

type tracedRefundStore struct {
	ordercontract.Store
	trace *refundLockTrace
}

type tracedRefundTransaction struct {
	ordercontract.Transaction
	trace *refundLockTrace
}

func (s tracedRefundStore) WithinTransaction(fn func(ordercontract.Transaction) error) error {
	return s.Store.WithinTransaction(func(tx ordercontract.Transaction) error {
		s.trace.rootLocked = false
		return fn(tracedRefundTransaction{Transaction: tx, trace: s.trace})
	})
}

func (tx tracedRefundTransaction) Orders() ordercontract.Store {
	return tracedRefundStore{Store: tx.Transaction.Orders(), trace: tx.trace}
}

func (s tracedRefundStore) GetByIDForUpdate(id uint) (*orderdomain.Order, error) {
	if !s.trace.rootLocked && id != s.trace.rootID {
		return nil, fmt.Errorf("locked child %d before root %d", id, s.trace.rootID)
	}
	s.trace.rootLocked = true
	s.trace.locks = append(s.trace.locks, id)
	order, err := s.Store.GetByIDForUpdate(id)
	if err == nil && order != nil && id == s.trace.changedChildID {
		other := s.trace.rootID + 1000
		order.ParentID = &other
	}
	return order, err
}

func (s tracedRefundStore) UpdateChildrenStatus(parentID uint, status string, now time.Time) (int64, error) {
	if !s.trace.rootLocked || parentID != s.trace.rootID {
		return 0, errors.New("updating children without locking their parent first")
	}
	return s.Store.UpdateChildrenStatus(parentID, status, now)
}

func refundLockFixture(t *testing.T) (*gorm.DB, *orderdomain.Order, *orderdomain.Order) {
	t.Helper()
	_, db := setupOrderRefundWalletTest(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	createTestUser(t, db, 1)
	parent := createTestOrder(t, db, 1, "REFUND-LOCK-PARENT", decimal.NewFromInt(100))
	now := time.Now()
	parent.PaidAt, parent.Status = &now, constants.OrderStatusCompleted
	if err := db.Save(parent).Error; err != nil {
		t.Fatal(err)
	}
	child := createTestChildOrderWithFulfillmentType(t, db, parent, "REFUND-LOCK-CHILD", constants.OrderStatusCompleted, decimal.NewFromInt(100), constants.FulfillmentTypeManual, false)
	return db, parent, child
}

func TestAllRefundMutationEntrypointsLockParentBeforeChildren(t *testing.T) {
	for _, action := range []string{"manual", "wallet", "fee"} {
		for _, target := range []string{"parent", "child"} {
			t.Run(action+"/"+target, func(t *testing.T) {
				db, parent, child := refundLockFixture(t)
				trace := &refundLockTrace{rootID: parent.ID}
				store := tracedRefundStore{Store: orderstore.New(db, "refund-lock-order-test-secret"), trace: trace}
				svc := orderrefund.New(store, nil, nil, nil, walletServiceForTest(db), nil)
				orderID := parent.ID
				if target == "child" {
					orderID = child.ID
				}
				amount := money.FromDecimal(decimal.NewFromInt(10))
				var err error
				switch action {
				case "manual":
					_, _, err = svc.AdminManualRefund(orderrefund.AdminManualRefundInput{OrderID: orderID, Amount: amount})
				case "wallet":
					_, _, _, err = svc.AdminRefundToWallet(orderrefund.AdminRefundToWalletInput{OrderID: orderID, Amount: amount})
				case "fee":
					record := &orderdomain.OrderRefundRecord{OrderID: orderID, UserID: 1, Type: constants.OrderRefundTypeManual, Amount: amount, Currency: "CNY"}
					if err := db.Create(record).Error; err != nil {
						t.Fatal(err)
					}
					_, err = svc.UpdatePaymentFeeRefunded(orderrefund.UpdatePaymentFeeRefundedInput{RefundRecordID: record.ID})
				}
				if err != nil {
					t.Fatalf("refund entrypoint failed its transaction lock order: %v", err)
				}
				if len(trace.locks) == 0 || trace.locks[0] != parent.ID {
					t.Fatalf("parent must be the first row lock: %v", trace.locks)
				}
				if target == "child" && (len(trace.locks) < 2 || trace.locks[1] != child.ID) {
					t.Fatalf("child must only lock after its parent: %v", trace.locks)
				}
			})
		}
	}
}

func TestRefundRejectsParentRelationshipChangedBeforeLock(t *testing.T) {
	db, parent, child := refundLockFixture(t)
	trace := &refundLockTrace{rootID: parent.ID, changedChildID: child.ID}
	store := tracedRefundStore{Store: orderstore.New(db, "refund-lock-order-test-secret"), trace: trace}
	svc := orderrefund.New(store, nil, nil, nil, walletServiceForTest(db), nil)
	if _, _, _, err := svc.AdminRefundToWallet(orderrefund.AdminRefundToWalletInput{OrderID: child.ID, Amount: money.FromDecimal(decimal.NewFromInt(10))}); !errors.Is(err, orderrefund.ErrOrderStatusInvalid) {
		t.Fatalf("changed parent must be rejected instead of acquiring another parent lock: %v", err)
	}
	var records int64
	if err := db.Model(&orderdomain.OrderRefundRecord{}).Count(&records).Error; err != nil || records != 0 {
		t.Fatalf("invalid hierarchy created a refund: count=%d err=%v", records, err)
	}
	if err := db.First(parent, parent.ID).Error; err != nil || !parent.RefundedAmount.Decimal.IsZero() {
		t.Fatalf("invalid hierarchy changed parent refund total: %+v %v", parent, err)
	}
}

func TestConcurrentParentAndChildRefundShareOneAmountBudgetOnSingleConnection(t *testing.T) {
	for _, action := range []string{"manual", "wallet"} {
		t.Run(action, func(t *testing.T) {
			db, parent, child := refundLockFixture(t)
			svc := orderrefund.New(orderstore.New(db, "refund-lock-order-test-secret"), nil, nil, nil, walletServiceForTest(db), nil)
			start := make(chan struct{})
			results := make(chan error, 2)
			for _, orderID := range []uint{parent.ID, child.ID} {
				go func() {
					<-start
					amount := money.FromDecimal(decimal.NewFromInt(60))
					var err error
					if action == "manual" {
						_, _, err = svc.AdminManualRefund(orderrefund.AdminManualRefundInput{OrderID: orderID, Amount: amount})
					} else {
						_, _, _, err = svc.AdminRefundToWallet(orderrefund.AdminRefundToWalletInput{OrderID: orderID, Amount: amount})
					}
					results <- err
				}()
			}
			close(start)
			succeeded, exceeded := 0, 0
			for range 2 {
				select {
				case err := <-results:
					switch {
					case err == nil:
						succeeded++
					case errors.Is(err, walletcontract.ErrRefundExceeded):
						exceeded++
					default:
						t.Fatalf("unexpected concurrent refund error: %v", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("refund blocked with SQLite MaxOpenConns=1")
				}
			}
			if succeeded != 1 || exceeded != 1 {
				t.Fatalf("refunds bypassed shared parent budget: succeeded=%d exceeded=%d", succeeded, exceeded)
			}
			if err := db.First(parent, parent.ID).Error; err != nil || !parent.RefundedAmount.Decimal.Equal(decimal.NewFromInt(60)) {
				t.Fatalf("unexpected cumulative parent refund: %+v %v", parent, err)
			}
			if action == "wallet" {
				var wallet walletdomain.Account
				if err := db.First(&wallet, "user_id = ?", 1).Error; err != nil || !wallet.Balance.Decimal.Equal(decimal.NewFromInt(60)) {
					t.Fatalf("wallet must only be credited once: %+v %v", wallet, err)
				}
			}
		})
	}
}
