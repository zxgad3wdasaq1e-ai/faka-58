package integrationtest

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/constants"
	affiliateapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/application"
	affiliatecontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/contract"
	affiliatedomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/domain"
	fulfillmentdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/fulfillment/domain"
	admindomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/admin/domain"
	orderrefund "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/application/refund"
	orderdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/domain"
	orderstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/infrastructure/gormstore"
	walletdomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/wallet/domain"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/shared/money"
	"gorm.io/gorm"
)

type affiliateRefundFixture struct {
	svc           *affiliateapp.Service
	db            *gorm.DB
	store         affiliatecontract.Store
	refunds       *orderrefund.Service
	profile       affiliatedomain.Profile
	buyerID       uint
	orderSequence uint
}

func newAffiliateRefundFixture(t *testing.T) *affiliateRefundFixture {
	t.Helper()
	svc, db, store := setupAffiliateServiceTest(t)
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &orderdomain.OrderRefundRecord{}, &fulfillmentdomain.Fulfillment{}, &admindomain.Admin{},
		&affiliatedomain.WithdrawRequest{}, &affiliatedomain.Commission{}, &walletdomain.Account{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Match production SQLite: a global-store read inside any transaction hangs.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	promoter := createAffiliateTestUser(t, db, "refund-promoter@example.com")
	buyer := createAffiliateTestUser(t, db, "refund-buyer@example.com")
	profile := createAffiliateTestProfile(t, db, promoter.ID, "REFUND01", constants.AffiliateProfileStatusActive)
	for _, userID := range []uint{promoter.ID, buyer.ID} {
		if err := db.Create(&walletdomain.Account{UserID: userID, Balance: amount("77.33")}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&admindomain.Admin{ID: 1, Username: "reviewer", PasswordHash: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	return &affiliateRefundFixture{
		svc: svc, db: db, store: store, profile: profile, buyerID: buyer.ID,
		refunds: orderrefund.New(orderstore.New(db, "affiliate-refund-test-key"), nil, svc, nil, nil, nil),
	}
}

func amount(value string) money.Amount { return money.FromDecimal(decimal.RequireFromString(value)) }

func (f *affiliateRefundFixture) order(t *testing.T, total string, parentID *uint, guest bool) orderdomain.Order {
	t.Helper()
	now := time.Now()
	f.orderSequence++
	row := orderdomain.Order{
		OrderNo: fmt.Sprintf("AFF-REFUND-%d", f.orderSequence), ParentID: parentID,
		UserID: f.buyerID, Status: constants.OrderStatusPaid, Currency: "CNY", TotalAmount: amount(total),
		AffiliateProfileID: &f.profile.ID, PaidAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if guest {
		row.UserID = 0
		row.GuestEmail = "guest-refund@example.com"
	}
	if err := f.db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func (f *affiliateRefundFixture) commission(t *testing.T, orderID uint, base, value, status string) affiliatedomain.Commission {
	t.Helper()
	row := affiliatedomain.Commission{
		AffiliateProfileID: f.profile.ID, OrderID: orderID, CommissionType: constants.AffiliateCommissionTypeOrder,
		BaseAmount: amount(base), RatePercent: amount("20"), CommissionAmount: amount(value), Status: status,
	}
	if err := f.store.CreateCommission(&row); err != nil {
		t.Fatal(err)
	}
	return row
}

func (f *affiliateRefundFixture) withdraw(t *testing.T, value string) *affiliatedomain.WithdrawRequest {
	t.Helper()
	row, err := f.svc.ApplyWithdraw(f.profile.UserID, affiliateapp.WithdrawApplyInput{
		Amount: decimal.RequireFromString(value), Channel: "usdt", Account: "test-only-account",
	})
	if err != nil {
		t.Fatalf("apply withdrawal %s: %v", value, err)
	}
	return row
}

func (f *affiliateRefundFixture) refund(t *testing.T, orderID uint, value string) {
	t.Helper()
	if _, _, err := f.refunds.AdminManualRefund(orderrefund.AdminManualRefundInput{OrderID: orderID, Amount: amount(value)}); err != nil {
		t.Fatalf("refund %s: %v", value, err)
	}
}

func (f *affiliateRefundFixture) commissions(t *testing.T, orderID uint, want string) []affiliatedomain.Commission {
	t.Helper()
	rows, err := f.store.ListCommissionsByOrder(orderID, nil)
	if err != nil {
		t.Fatal(err)
	}
	total := decimal.Zero
	for _, row := range rows {
		total = total.Add(row.CommissionAmount.Decimal)
	}
	if !total.Equal(decimal.RequireFromString(want)) {
		t.Fatalf("order %d commission total = %s, want %s: %+v", orderID, total, want, rows)
	}
	return rows
}

func (f *affiliateRefundFixture) assertRejected(t *testing.T, id uint, value string) {
	t.Helper()
	req, err := f.store.GetWithdrawByID(id)
	if err != nil || req == nil {
		t.Fatalf("load withdrawal: %v", err)
	}
	if req.Status != constants.AffiliateWithdrawStatusRejected || req.RejectReason != affiliateapp.WithdrawRefundRejectReason ||
		!req.Amount.Decimal.Equal(decimal.RequireFromString(value)) {
		t.Fatalf("refund must reject original request without changing amount: %+v", req)
	}
	again, err := f.svc.ReviewWithdraw(1, id, constants.AffiliateWithdrawActionReject, "replace audit reason")
	if err != nil || again.RejectReason != req.RejectReason || !again.ProcessedAt.Equal(*req.ProcessedAt) {
		t.Fatalf("repeated rejection changed audit history: %+v %v", again, err)
	}
	if _, err := f.svc.ReviewWithdraw(1, id, constants.AffiliateWithdrawActionPay, ""); !errors.Is(err, affiliateapp.ErrWithdrawRefunded) {
		t.Fatalf("refunded withdrawal was payable: %v", err)
	}
}

func (f *affiliateRefundFixture) assertWalletsUnchanged(t *testing.T) {
	t.Helper()
	var wallets []walletdomain.Account
	if err := f.db.Find(&wallets).Error; err != nil {
		t.Fatal(err)
	}
	if len(wallets) != 2 {
		t.Fatalf("wallet count = %d", len(wallets))
	}
	for _, wallet := range wallets {
		if !wallet.Balance.Decimal.Equal(decimal.RequireFromString("77.33")) {
			t.Fatalf("affiliate correction changed shopping wallet: %+v", wallet)
		}
	}
}

func TestAffiliateRefundPartialFullAndRetry(t *testing.T) {
	for _, status := range []string{constants.AffiliateCommissionStatusAvailable, constants.AffiliateCommissionStatusPendingConfirm} {
		t.Run(status, func(t *testing.T) {
			f := newAffiliateRefundFixture(t)
			order := f.order(t, "100", nil, false)
			f.commission(t, order.ID, "100", "20", status)
			f.refund(t, order.ID, "25")
			f.commissions(t, order.ID, "15")
			f.refund(t, order.ID, "50")
			f.commissions(t, order.ID, "5")
			f.refund(t, order.ID, "25")
			rows := f.commissions(t, order.ID, "0")
			if rows[0].Status != constants.AffiliateCommissionStatusRejected || !rows[0].BaseAmount.Decimal.IsZero() {
				t.Fatalf("full refund did not invalidate commission: %+v", rows[0])
			}
			if err := f.store.WithinTransaction(func(tx affiliatecontract.Store) error {
				return f.svc.HandleOrderRefunded(tx, &order, decimal.NewFromInt(25), decimal.NewFromInt(75), "retry")
			}); err != nil {
				t.Fatal(err)
			}
			f.commissions(t, order.ID, "0")
			f.assertWalletsUnchanged(t)
		})
	}
}

func TestAffiliateRefundRejectsMultiOrderWithdrawAndUnfreezesValidMoney(t *testing.T) {
	f := newAffiliateRefundFixture(t)
	a := f.order(t, "100", nil, false)
	b := f.order(t, "150", nil, false)
	f.commission(t, a.ID, "100", "20", constants.AffiliateCommissionStatusAvailable)
	f.commission(t, b.ID, "150", "30", constants.AffiliateCommissionStatusAvailable)
	req := f.withdraw(t, "25")
	f.refund(t, a.ID, "50")
	f.assertRejected(t, req.ID, "25")
	f.commissions(t, a.ID, "10")
	rows := f.commissions(t, b.ID, "30")
	base := decimal.Zero
	for _, row := range rows {
		if row.WithdrawRequestID != nil {
			t.Fatalf("unrelated commission stayed frozen: %+v", row)
		}
		base = base.Add(row.BaseAmount.Decimal)
	}
	if !base.Equal(decimal.NewFromInt(150)) {
		t.Fatalf("split duplicated commission base: %s", base)
	}
	replacement := f.withdraw(t, "40")
	paid, err := f.svc.ReviewWithdraw(1, replacement.ID, constants.AffiliateWithdrawActionPay, "")
	if err != nil || paid.Status != constants.AffiliateWithdrawStatusPaid {
		t.Fatalf("remaining valid commissions not payable: %+v %v", paid, err)
	}
	paidAgain, err := f.svc.ReviewWithdraw(2, replacement.ID, constants.AffiliateWithdrawActionPay, "")
	if err != nil || !paidAgain.ProcessedAt.Equal(*paid.ProcessedAt) || *paidAgain.ProcessedBy != 1 {
		t.Fatalf("repeated pay changed audit history: %+v %v", paidAgain, err)
	}
	// A refund after a completed payout keeps the settled history intact.
	f.refund(t, a.ID, "50")
	rows = f.commissions(t, a.ID, "10")
	if rows[0].Status != constants.AffiliateCommissionStatusWithdrawn {
		t.Fatalf("paid commission history changed: %+v", rows[0])
	}
	f.assertWalletsUnchanged(t)
}

func TestAffiliateRefundReconcilesAllSplitsAndIsIdempotent(t *testing.T) {
	f := newAffiliateRefundFixture(t)
	order := f.order(t, "100", nil, false)
	f.commission(t, order.ID, "100", "20", constants.AffiliateCommissionStatusAvailable)
	req := f.withdraw(t, "8")
	f.refund(t, order.ID, "25")
	f.assertRejected(t, req.ID, "8")
	rows := f.commissions(t, order.ID, "15")
	if len(rows) != 2 {
		t.Fatalf("unexpected split rows: %+v", rows)
	}
	for _, row := range rows {
		if row.WithdrawRequestID != nil || !row.OrderRefundedAmount.Decimal.Equal(decimal.NewFromInt(25)) {
			t.Fatalf("split was not reconciled: %+v", row)
		}
	}
	if err := f.store.WithinTransaction(func(tx affiliatecontract.Store) error {
		return f.svc.HandleOrderRefunded(tx, &order, decimal.NewFromInt(25), decimal.Zero, "duplicate")
	}); err != nil {
		t.Fatal(err)
	}
	f.commissions(t, order.ID, "15")
	replacement := f.withdraw(t, "15")
	if _, err := f.svc.ReviewWithdraw(1, replacement.ID, constants.AffiliateWithdrawActionPay, ""); err != nil {
		t.Fatalf("reconciled split commissions could not be paid: %v", err)
	}
	f.assertWalletsUnchanged(t)
}

func TestAffiliateReviewRepairsLegacyRefundedPendingWithdrawal(t *testing.T) {
	for _, refunded := range []string{"40", "100"} {
		t.Run(refunded, func(t *testing.T) {
			f := newAffiliateRefundFixture(t)
			order := f.order(t, "100", nil, false)
			f.commission(t, order.ID, "100", "20", constants.AffiliateCommissionStatusAvailable)
			req := f.withdraw(t, "20")
			if err := f.db.Model(&orderdomain.Order{}).Where("id = ?", order.ID).
				Updates(map[string]interface{}{"refunded_amount": amount(refunded), "status": constants.OrderStatusPartiallyRefunded}).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.ReviewWithdraw(1, req.ID, constants.AffiliateWithdrawActionPay, ""); !errors.Is(err, affiliateapp.ErrWithdrawRefunded) {
				t.Fatalf("legacy refunded withdrawal was payable: %v", err)
			}
			f.assertRejected(t, req.ID, "20")
			want := "12"
			if refunded == "100" {
				want = "0"
			}
			f.commissions(t, order.ID, want)
			if want == "12" {
				replacement := f.withdraw(t, want)
				if _, err := f.svc.ReviewWithdraw(1, replacement.ID, constants.AffiliateWithdrawActionPay, ""); err != nil {
					t.Fatalf("reconciled remainder not payable: %v", err)
				}
			}
			f.assertWalletsUnchanged(t)
		})
	}
}

func TestAffiliateGuestAndChildRefundReconcileParentCommission(t *testing.T) {
	for _, child := range []bool{false, true} {
		t.Run(fmt.Sprintf("child=%v", child), func(t *testing.T) {
			f := newAffiliateRefundFixture(t)
			parent := f.order(t, "100", nil, true)
			target := parent
			if child {
				target = f.order(t, "50", &parent.ID, true)
				f.order(t, "50", &parent.ID, true)
			}
			f.commission(t, parent.ID, "100", "20", constants.AffiliateCommissionStatusAvailable)
			req := f.withdraw(t, "20")
			f.refund(t, target.ID, "50")
			f.assertRejected(t, req.ID, "20")
			f.commissions(t, parent.ID, "10")
			f.assertWalletsUnchanged(t)
		})
	}
}

func TestAffiliateConcurrentApplyAndReviewAreIdempotent(t *testing.T) {
	f := newAffiliateRefundFixture(t)
	order := f.order(t, "100", nil, false)
	f.commission(t, order.ID, "100", "20", constants.AffiliateCommissionStatusAvailable)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			<-start
			_, err := f.svc.ApplyWithdraw(f.profile.UserID, affiliateapp.WithdrawApplyInput{
				Amount: decimal.NewFromInt(20), Channel: "usdt", Account: "test-only-account",
			})
			results <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, affiliateapp.ErrWithdrawInsufficient) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent applications spent commissions %d times", success)
	}
	requests, _, err := f.store.ListWithdraws(affiliatecontract.WithdrawListFilter{Page: 1, PageSize: 20})
	if err != nil || len(requests) != 1 {
		t.Fatalf("unexpected requests: %+v %v", requests, err)
	}
	start = make(chan struct{})
	results = make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			<-start
			_, err := f.svc.ReviewWithdraw(1, requests[0].ID, constants.AffiliateWithdrawActionPay, "")
			results <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	rows := f.commissions(t, order.ID, "20")
	if rows[0].Status != constants.AffiliateCommissionStatusWithdrawn {
		t.Fatal("concurrent reviews did not settle commission exactly once")
	}
	f.assertWalletsUnchanged(t)
}

func TestAffiliateRefundVersusReviewRaceHasOneFinalOutcome(t *testing.T) {
	for iteration := range 8 {
		t.Run(fmt.Sprint(iteration), func(t *testing.T) {
			f := newAffiliateRefundFixture(t)
			order := f.order(t, "100", nil, false)
			f.commission(t, order.ID, "100", "20", constants.AffiliateCommissionStatusAvailable)
			req := f.withdraw(t, "20")
			start := make(chan struct{})
			var refundErr, reviewErr error
			var wg sync.WaitGroup
			wg.Go(func() {
				<-start
				_, _, refundErr = f.refunds.AdminManualRefund(orderrefund.AdminManualRefundInput{OrderID: order.ID, Amount: amount("100")})
			})
			wg.Go(func() {
				<-start
				_, reviewErr = f.svc.ReviewWithdraw(1, req.ID, constants.AffiliateWithdrawActionPay, "")
			})
			close(start)
			wg.Wait()
			if refundErr != nil || (reviewErr != nil && !errors.Is(reviewErr, affiliateapp.ErrWithdrawRefunded)) {
				t.Fatalf("race errors: refund=%v review=%v", refundErr, reviewErr)
			}
			final, err := f.store.GetWithdrawByID(req.ID)
			if err != nil {
				t.Fatal(err)
			}
			switch final.Status {
			case constants.AffiliateWithdrawStatusRejected:
				f.assertRejected(t, req.ID, "20")
				f.commissions(t, order.ID, "0")
			case constants.AffiliateWithdrawStatusPaid:
				rows := f.commissions(t, order.ID, "20")
				if rows[0].Status != constants.AffiliateCommissionStatusWithdrawn || reviewErr != nil {
					t.Fatalf("paid audit history inconsistent: %+v, %v", rows, reviewErr)
				}
			default:
				t.Fatalf("race left withdrawal unfinished: %+v", final)
			}
			f.assertWalletsUnchanged(t)
		})
	}
}

func TestAffiliateLegacyRefundHoldExcludesOnlyDisputedCommissions(t *testing.T) {
	f := newAffiliateRefundFixture(t)
	oldOrder := f.order(t, "100", nil, false)
	held := f.commission(t, oldOrder.ID, "60", "12", constants.AffiliateCommissionStatusAvailable)
	held.OrderRefundedAmount = amount("40")
	held.InvalidReason = affiliatedomain.CommissionInvalidReasonLegacyRefundReview
	if err := f.store.UpdateCommission(&held); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&orderdomain.Order{}).Where("id = ?", oldOrder.ID).Updates(map[string]interface{}{
		"refunded_amount": amount("40"), "status": constants.OrderStatusPartiallyRefunded,
	}).Error; err != nil {
		t.Fatal(err)
	}
	valid := f.order(t, "50", nil, false)
	f.commission(t, valid.ID, "50", "10", constants.AffiliateCommissionStatusAvailable)
	available, err := f.store.SumCommissionByProfile(f.profile.ID, []string{constants.AffiliateCommissionStatusAvailable}, true)
	if err != nil || !available.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("available balance includes disputed money: %s %v", available, err)
	}
	stats, err := f.store.GetProfileStatsBatch([]uint{f.profile.ID})
	if err != nil || !stats[f.profile.ID].AvailableCommission.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("admin balance includes disputed money: %+v %v", stats, err)
	}
	if _, err := f.svc.ApplyWithdraw(f.profile.UserID, affiliateapp.WithdrawApplyInput{
		Amount: decimal.NewFromInt(11), Channel: "usdt", Account: "test-only",
	}); !errors.Is(err, affiliateapp.ErrWithdrawLegacyRefund) {
		t.Fatalf("missing explicit held-money error: %v", err)
	}
	req := f.withdraw(t, "10")
	if _, err := f.svc.ReviewWithdraw(1, req.ID, constants.AffiliateWithdrawActionPay, ""); err != nil {
		t.Fatalf("unrelated valid commission was blocked: %v", err)
	}
	f.refund(t, oldOrder.ID, "20")
	rows := f.commissions(t, oldOrder.ID, "8")
	if rows[0].InvalidReason != affiliatedomain.CommissionInvalidReasonLegacyRefundReview {
		t.Fatal("later refund cleared the manual review hold")
	}
	if _, err := f.svc.ApplyWithdraw(f.profile.UserID, affiliateapp.WithdrawApplyInput{
		Amount: decimal.NewFromInt(1), Channel: "usdt", Account: "test-only",
	}); !errors.Is(err, affiliateapp.ErrWithdrawLegacyRefund) {
		t.Fatalf("later refund revived held commission: %v", err)
	}
	f.refund(t, oldOrder.ID, "40")
	rows = f.commissions(t, oldOrder.ID, "0")
	if rows[0].InvalidReason != affiliatedomain.CommissionInvalidReasonLegacyRefundReview || rows[0].Status != constants.AffiliateCommissionStatusRejected {
		t.Fatal("full refund lost the hold audit trail")
	}
	f.assertWalletsUnchanged(t)
}

func TestAffiliateTinySplitsCannotEvadePartialRefundRounding(t *testing.T) {
	f := newAffiliateRefundFixture(t)
	order := f.order(t, "100", nil, false)
	for i := range 3 {
		row := affiliatedomain.Commission{AffiliateProfileID: f.profile.ID, OrderID: order.ID,
			CommissionType: fmt.Sprintf("split%d", i), BaseAmount: amount("0.05"), RatePercent: amount("20"),
			CommissionAmount: amount("0.01"), Status: constants.AffiliateCommissionStatusAvailable,
		}
		if err := f.store.CreateCommission(&row); err != nil {
			t.Fatal(err)
		}
	}
	req := f.withdraw(t, "0.03")
	f.refund(t, order.ID, "50")
	f.assertRejected(t, req.ID, "0.03")
	f.commissions(t, order.ID, "0.02")
	f.assertWalletsUnchanged(t)
}

type failAfterAffiliateRefund struct{ svc *affiliateapp.Service }

var errAfterAffiliateRefund = errors.New("failure after affiliate reconciliation")

func (p failAfterAffiliateRefund) HandleOrderRefunded(store affiliatecontract.Store, order *orderdomain.Order, delta, before decimal.Decimal, reason string) error {
	if err := p.svc.HandleOrderRefunded(store, order, delta, before, reason); err != nil {
		return err
	}
	return errAfterAffiliateRefund
}

func TestAffiliateRefundAndWithdrawalRejectionRollBackTogether(t *testing.T) {
	f := newAffiliateRefundFixture(t)
	order := f.order(t, "100", nil, false)
	f.commission(t, order.ID, "100", "20", constants.AffiliateCommissionStatusAvailable)
	req := f.withdraw(t, "20")
	broken := orderrefund.New(orderstore.New(f.db, "affiliate-refund-test-key"), nil, failAfterAffiliateRefund{f.svc}, nil, nil, nil)
	if _, _, err := broken.AdminManualRefund(orderrefund.AdminManualRefundInput{OrderID: order.ID, Amount: amount("100")}); !errors.Is(err, errAfterAffiliateRefund) {
		t.Fatalf("expected injected failure, got %v", err)
	}
	stillPending, err := f.store.GetWithdrawByID(req.ID)
	if err != nil || stillPending.Status != constants.AffiliateWithdrawStatusPendingReview {
		t.Fatalf("failed refund committed withdrawal rejection: %+v %v", stillPending, err)
	}
	rows := f.commissions(t, order.ID, "20")
	if rows[0].WithdrawRequestID == nil || !rows[0].OrderRefundedAmount.Decimal.IsZero() {
		t.Fatal("failed refund committed commission correction")
	}
	state, err := f.store.GetOrderRefundState(order.ID)
	if err != nil || !state.RefundedAmount.IsZero() {
		t.Fatalf("failed refund changed order principal: %+v %v", state, err)
	}
	f.refund(t, order.ID, "100")
	f.assertRejected(t, req.ID, "20")
	f.commissions(t, order.ID, "0")
	f.assertWalletsUnchanged(t)
}
