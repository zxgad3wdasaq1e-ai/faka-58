package application

import (
	"sort"
	"strconv"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// HandleOrderPaid 处理订单支付成功后的佣金生成
func (s *Service) HandleOrderPaid(orderID uint) error {
	if orderID == 0 || s.repo == nil || s.orderRepo == nil {
		return nil
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return err
	}
	if !setting.Enabled || setting.CommissionRate <= 0 {
		return nil
	}

	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}
	// 重试任务可能晚于退款/取消执行，此时退款流程已找不到佣金可冲销，不能再生成
	if order == nil || order.Status == constants.OrderStatusRefunded || order.Status == constants.OrderStatusCanceled {
		return nil
	}
	profile, err := s.resolveAffiliateProfileForOrder(order)
	if err != nil {
		return err
	}
	if profile == nil {
		return nil
	}
	if strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
		return nil
	}
	if order.UserID > 0 && profile.UserID == order.UserID {
		return nil
	}

	commissionType := constants.AffiliateCommissionTypeOrder
	existing, err := s.repo.GetCommissionByOrderAndProfile(order.ID, profile.ID, commissionType)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	baseAmount, err := s.calculateCommissionBaseAmount(order)
	if err != nil {
		return err
	}
	if baseAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	rate := decimal.NewFromFloat(setting.CommissionRate).Round(2)
	commissionAmount := baseAmount.Mul(rate).Div(decimal.NewFromInt(100)).Round(2)
	if commissionAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	paidAt := time.Now()
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}
	status := constants.AffiliateCommissionStatusPendingConfirm
	var confirmAt *time.Time
	var availableAt *time.Time
	if setting.ConfirmDays <= 0 {
		status = constants.AffiliateCommissionStatusAvailable
		availableAt = &paidAt
	} else {
		t := paidAt.Add(time.Duration(setting.ConfirmDays) * 24 * time.Hour)
		confirmAt = &t
	}

	commission := &affiliatedomain.Commission{
		AffiliateProfileID: profile.ID,
		OrderID:            order.ID,
		CommissionType:     commissionType,
		BaseAmount:         money.FromDecimal(baseAmount),
		RatePercent:        money.FromDecimal(rate),
		CommissionAmount:   money.FromDecimal(commissionAmount),
		Status:             status,
		ConfirmAt:          confirmAt,
		AvailableAt:        availableAt,
	}
	return s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		if err := repoTx.LockProfiles([]uint{profile.ID}); err != nil {
			return err
		}
		// A retry may have read the order before a concurrent refund committed.
		// Read its current state using this transaction after acquiring the same
		// profile lock used by the refund path.
		state, err := repoTx.GetOrderRefundState(order.ID)
		if err != nil {
			return err
		}
		if state == nil || state.Status == constants.OrderStatusRefunded || state.Status == constants.OrderStatusCanceled {
			return nil
		}
		existing, err := repoTx.GetCommissionByOrderAndProfile(order.ID, profile.ID, commissionType)
		if err != nil || existing != nil {
			return err
		}
		applyCommissionRefund(commission, state.TotalAmount, state.RefundedAmount, "order_refunded", time.Now(), &commissionRefundRounding{})
		if commission.CommissionAmount.Decimal.LessThanOrEqual(decimal.Zero) {
			return nil
		}
		return repoTx.CreateCommission(commission)
	})
}

// ConfirmDueCommissions 将到期佣金转可提现
func (s *Service) ConfirmDueCommissions(now time.Time) error {
	if s.repo == nil {
		return nil
	}
	_, err := s.repo.MarkPendingCommissionsAvailable(now, now)
	return err
}

// HandleOrderCanceled 处理订单取消/退款后的佣金逆向
func (s *Service) HandleOrderCanceled(orderID uint, reason string) error {
	if orderID == 0 || s.repo == nil {
		return nil
	}
	return s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		state, err := repoTx.GetOrderRefundState(orderID)
		if err != nil || state == nil {
			return err
		}
		return s.reconcileOrderRefund(repoTx, state, state.TotalAmount, reason)
	})
}

// HandleOrderRefunded 使用调用方提供的事务 Store 处理退款后的佣金回滚。
func (s *Service) HandleOrderRefunded(
	repoTx affiliatecontract.Store,
	order *orderdomain.Order,
	refundDelta decimal.Decimal,
	refundedBefore decimal.Decimal,
	reason string,
) error {
	if repoTx == nil || order == nil || order.ID == 0 {
		return nil
	}
	delta := refundDelta.Round(2)
	if delta.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	state := &affiliatecontract.OrderRefundState{
		ID: order.ID, AffiliateProfileID: order.AffiliateProfileID, AffiliateCode: order.AffiliateCode,
		TotalAmount: order.TotalAmount.Decimal, RefundedAmount: refundedBefore.Add(delta),
	}
	if order.ParentID != nil && *order.ParentID > 0 {
		// Payment creates the commission on the parent; reserveParentRefund has
		// already updated that parent's cumulative refund in this transaction.
		var err error
		state, err = repoTx.GetOrderRefundState(*order.ParentID)
		if err != nil || state == nil {
			return err
		}
	}
	return s.reconcileOrderRefund(repoTx, state, state.RefundedAmount, reason)
}

func (s *Service) reconcileOrderRefund(repoTx affiliatecontract.Store, state *affiliatecontract.OrderRefundState, refunded decimal.Decimal, reason string) error {
	rows, err := repoTx.ListCommissionsByOrder(state.ID, nil)
	if err != nil {
		return err
	}
	profileIDs := make([]uint, 0, len(rows)+1)
	if state.AffiliateProfileID != nil {
		profileIDs = append(profileIDs, *state.AffiliateProfileID)
	} else if state.AffiliateCode != "" {
		profile, err := repoTx.GetProfileByCode(state.AffiliateCode)
		if err != nil {
			return err
		}
		if profile != nil {
			profileIDs = append(profileIDs, profile.ID)
		}
	}
	for _, row := range rows {
		profileIDs = append(profileIDs, row.AffiliateProfileID)
	}
	if err := repoTx.LockProfiles(profileIDs); err != nil {
		return err
	}
	// All monetary writers lock the profile first. Re-read after waiting, so a
	// simultaneous ApplyWithdraw cannot hide a newly split or bound commission.
	rows, err = repoTx.ListCommissionsByOrder(state.ID, nil)
	if err != nil {
		return err
	}
	now := time.Now()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "order_refunded"
	}
	withdrawIDs := make([]uint, 0)
	seen := make(map[uint]bool)
	for _, row := range rows {
		if row.Status == constants.AffiliateCommissionStatusWithdrawn || row.WithdrawRequestID == nil || !refunded.GreaterThan(row.OrderRefundedAmount.Decimal) {
			continue
		}
		if id := *row.WithdrawRequestID; !seen[id] {
			seen[id] = true
			withdrawIDs = append(withdrawIDs, id)
		}
	}
	sort.Slice(withdrawIDs, func(i, j int) bool { return withdrawIDs[i] < withdrawIDs[j] })
	for _, id := range withdrawIDs {
		req, err := repoTx.GetWithdrawByIDForUpdate(id)
		if err != nil {
			return err
		}
		if req != nil && req.Status == constants.AffiliateWithdrawStatusPendingReview {
			if err := rejectRefundedWithdraw(repoTx, req, now); err != nil {
				return err
			}
		}
	}
	rows, err = repoTx.ListCommissionsByOrderForUpdate(state.ID, []string{
		constants.AffiliateCommissionStatusPendingConfirm, constants.AffiliateCommissionStatusAvailable,
	})
	if err != nil {
		return err
	}
	roundingBySnapshot := make(map[string]*commissionRefundRounding)
	for i := range rows {
		item := &rows[i]
		// Paid withdrawals are immutable history. Never debit shopping wallets.
		if item.WithdrawRequestID != nil {
			continue
		}
		key := item.OrderRefundedAmount.Decimal.StringFixed(2)
		rounding := roundingBySnapshot[key]
		if rounding == nil {
			rounding = &commissionRefundRounding{}
			roundingBySnapshot[key] = rounding
		}
		if applyCommissionRefund(item, state.TotalAmount, refunded, reason, now, rounding) {
			if err := repoTx.UpdateCommission(item); err != nil {
				return err
			}
		}
	}
	return nil
}

type commissionRefundRounding struct {
	commission, base, remainingCommission, remainingBase decimal.Decimal
}

func applyCommissionRefund(item *affiliatedomain.Commission, total, refunded decimal.Decimal, reason string, now time.Time, rounding *commissionRefundRounding) bool {
	total = total.Round(2)
	refunded = decimal.Min(decimal.Max(refunded.Round(2), decimal.Zero), total)
	before := decimal.Max(item.OrderRefundedAmount.Decimal.Round(2), decimal.Zero)
	if total.LessThanOrEqual(decimal.Zero) || !refunded.GreaterThan(before) {
		return false
	}
	remaining := total.Sub(before)
	if remaining.LessThanOrEqual(decimal.Zero) {
		return false
	}
	fraction := total.Sub(refunded).Div(remaining)
	// Round cumulative totals, then distribute the difference. Rounding each
	// split independently lets many one-cent splits evade a partial clawback.
	rounding.commission = rounding.commission.Add(item.CommissionAmount.Decimal)
	rounding.base = rounding.base.Add(item.BaseAmount.Decimal)
	nextCommission := rounding.commission.Mul(fraction).Round(2)
	nextBase := rounding.base.Mul(fraction).Round(2)
	item.CommissionAmount = money.FromDecimal(decimal.Max(nextCommission.Sub(rounding.remainingCommission), decimal.Zero))
	item.BaseAmount = money.FromDecimal(decimal.Max(nextBase.Sub(rounding.remainingBase), decimal.Zero))
	rounding.remainingCommission, rounding.remainingBase = nextCommission, nextBase
	item.OrderRefundedAmount = money.FromDecimal(refunded)
	item.UpdatedAt = now
	if item.CommissionAmount.Decimal.LessThanOrEqual(decimal.Zero) {
		item.Status = constants.AffiliateCommissionStatusRejected
		if item.InvalidReason != affiliatedomain.CommissionInvalidReasonLegacyRefundReview {
			item.InvalidReason = reason
		}
		item.ConfirmAt = nil
		item.AvailableAt = nil
	}
	return true
}

func (s *Service) resolveAffiliateProfileForOrder(order *orderdomain.Order) (*affiliatedomain.Profile, error) {
	if order == nil || s.repo == nil {
		return nil, nil
	}
	if order.AffiliateProfileID != nil && *order.AffiliateProfileID > 0 {
		return s.repo.GetProfileByID(*order.AffiliateProfileID)
	}
	if strings.TrimSpace(order.AffiliateCode) != "" {
		return s.repo.GetProfileByCode(order.AffiliateCode)
	}
	return nil, nil
}

func (s *Service) calculateCommissionBaseAmount(order *orderdomain.Order) (decimal.Decimal, error) {
	if order == nil || s.productRepo == nil {
		return decimal.Zero, nil
	}
	productIDs := collectAffiliateProductIDs(order)
	if len(productIDs) == 0 {
		return decimal.Zero, nil
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return decimal.Zero, err
	}
	productMap := make(map[uint]productdomain.Product, len(products))
	for _, product := range products {
		productMap[product.ID] = product
	}

	targetOrders := order.Children
	if len(targetOrders) == 0 {
		targetOrders = []orderdomain.Order{*order}
	}

	total := decimal.Zero
	for _, current := range targetOrders {
		for _, item := range current.Items {
			product, ok := productMap[item.ProductID]
			if !ok || !product.IsAffiliateEnabled {
				continue
			}
			payable := item.TotalPrice.Decimal.Sub(item.CouponDiscount.Decimal).Round(2)
			if payable.LessThan(decimal.Zero) {
				payable = decimal.Zero
			}
			total = total.Add(payable).Round(2)
		}
	}
	return total, nil
}

func collectAffiliateProductIDs(order *orderdomain.Order) []uint {
	if order == nil {
		return nil
	}
	ids := make([]uint, 0)
	seen := make(map[uint]struct{})
	appendItem := func(item orderdomain.OrderItem) {
		if item.ProductID == 0 {
			return
		}
		if _, ok := seen[item.ProductID]; ok {
			return
		}
		seen[item.ProductID] = struct{}{}
		ids = append(ids, item.ProductID)
	}
	for _, item := range order.Items {
		appendItem(item)
	}
	for _, child := range order.Children {
		for _, item := range child.Items {
			appendItem(item)
		}
	}
	return ids
}

func buildSplitCommissionType(sourceID uint) string {
	suffix := strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
	base := affiliateSplitTypePrefix + strconv.FormatUint(uint64(sourceID), 36)
	result := base + suffix
	if len(result) > 20 {
		return result[:20]
	}
	return result
}
