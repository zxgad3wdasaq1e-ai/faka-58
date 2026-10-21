package application

import (
	"strings"
	"time"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// ApplyWithdraw 用户提交提现申请
func (s *Service) ApplyWithdraw(userID uint, input WithdrawApplyInput) (*affiliatedomain.WithdrawRequest, error) {
	if userID == 0 || s.repo == nil {
		return nil, ErrNotOpened
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return nil, err
	}
	if !setting.Enabled {
		return nil, ErrDisabled
	}

	amount := input.Amount.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrWithdrawAmountInvalid
	}
	minAmount := decimal.NewFromFloat(setting.MinWithdrawAmount).Round(2)
	if amount.LessThan(minAmount) {
		return nil, ErrWithdrawAmountInvalid
	}
	channel := strings.TrimSpace(input.Channel)
	account := strings.TrimSpace(input.Account)
	if channel == "" || account == "" {
		return nil, ErrWithdrawChannelInvalid
	}
	if len(setting.WithdrawChannels) > 0 && !containsWithdrawChannel(setting.WithdrawChannels, channel) {
		return nil, ErrWithdrawChannelInvalid
	}
	if err := s.ConfirmDueCommissions(time.Now()); err != nil {
		return nil, err
	}

	var createdID uint
	err = s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		profile, err := repoTx.GetProfileByUserID(userID)
		if err != nil {
			return err
		}
		if profile == nil {
			return ErrNotOpened
		}
		if err := repoTx.LockProfiles([]uint{profile.ID}); err != nil {
			return err
		}
		profile, err = repoTx.GetProfileByUserID(userID)
		if err != nil {
			return err
		}
		if profile == nil {
			return ErrNotOpened
		}
		if strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
			return ErrNotOpened
		}

		commissions, err := repoTx.ListAvailableCommissionsForUpdate(profile.ID)
		if err != nil {
			return err
		}
		// Reconcile legacy commissions before freezing money; otherwise an old
		// refunded order could be split into a fresh withdrawal indefinitely.
		checked := make(map[uint]bool)
		for _, commission := range commissions {
			if checked[commission.OrderID] {
				continue
			}
			checked[commission.OrderID] = true
			state, err := repoTx.GetOrderRefundState(commission.OrderID)
			if err != nil {
				return err
			}
			if state == nil {
				return ErrWithdrawInsufficient
			}
			refunded := commissionRefundTarget(state)
			if refunded.GreaterThan(commission.OrderRefundedAmount.Decimal) {
				if err := s.reconcileOrderRefund(repoTx, state, refunded, "order_refunded"); err != nil {
					return err
				}
			}
		}
		commissions, err = repoTx.ListAvailableCommissionsForUpdate(profile.ID)
		if err != nil {
			return err
		}

		remaining := amount
		hasLegacyHold := false
		selectedIDs := make([]uint, 0)
		now := time.Now()
		for _, commission := range commissions {
			if commission.InvalidReason == affiliatedomain.CommissionInvalidReasonLegacyRefundReview {
				hasLegacyHold = true
				continue
			}
			if remaining.LessThanOrEqual(decimal.Zero) {
				break
			}
			rowAmount := commission.CommissionAmount.Decimal.Round(2)
			if rowAmount.LessThanOrEqual(decimal.Zero) {
				continue
			}
			if rowAmount.LessThanOrEqual(remaining) {
				selectedIDs = append(selectedIDs, commission.ID)
				remaining = remaining.Sub(rowAmount).Round(2)
				continue
			}

			// 最后一条记录金额大于申请剩余金额时，拆分记录避免超额冻结。
			boundAmount := remaining.Round(2)
			remainAmount := rowAmount.Sub(boundAmount).Round(2)
			originalBase := commission.BaseAmount.Decimal
			boundBase := originalBase.Mul(boundAmount).Div(rowAmount).Round(2)
			commission.CommissionAmount = money.FromDecimal(boundAmount)
			commission.BaseAmount = money.FromDecimal(boundBase)
			commission.UpdatedAt = now
			if err := repoTx.UpdateCommission(&commission); err != nil {
				return err
			}

			remainCommission := commission
			remainCommission.ID = 0
			remainCommission.CommissionType = buildSplitCommissionType(commission.ID)
			remainCommission.CommissionAmount = money.FromDecimal(remainAmount)
			remainCommission.BaseAmount = money.FromDecimal(originalBase.Sub(boundBase).Round(2))
			remainCommission.WithdrawRequestID = nil
			remainCommission.Status = constants.AffiliateCommissionStatusAvailable
			remainCommission.InvalidReason = ""
			remainCommission.CreatedAt = now
			remainCommission.UpdatedAt = now
			if err := repoTx.CreateCommission(&remainCommission); err != nil {
				return err
			}

			selectedIDs = append(selectedIDs, commission.ID)
			remaining = decimal.Zero
			break
		}
		if remaining.GreaterThan(decimal.Zero) {
			if hasLegacyHold {
				return ErrWithdrawLegacyRefund
			}
			return ErrWithdrawInsufficient
		}

		req := &affiliatedomain.WithdrawRequest{
			AffiliateProfileID: profile.ID,
			Amount:             money.FromDecimal(amount),
			Channel:            channel,
			Account:            account,
			Status:             constants.AffiliateWithdrawStatusPendingReview,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := repoTx.CreateWithdraw(req); err != nil {
			return err
		}
		if err := repoTx.BatchUpdateCommissions(selectedIDs, map[string]interface{}{
			"withdraw_request_id": req.ID,
			"updated_at":          now,
		}); err != nil {
			return err
		}
		createdID = req.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithdrawByID(createdID)
}

// ReviewWithdraw 管理端审核提现申请
func (s *Service) ReviewWithdraw(adminID, withdrawID uint, action, rejectReason string) (*affiliatedomain.WithdrawRequest, error) {
	if withdrawID == 0 || s.repo == nil {
		return nil, ErrNotFound
	}
	act := strings.ToLower(strings.TrimSpace(action))
	if act != constants.AffiliateWithdrawActionReject && act != constants.AffiliateWithdrawActionPay {
		return nil, ErrWithdrawStatusInvalid
	}
	rejectReason = strings.TrimSpace(rejectReason)

	var reviewErr error
	err := s.repo.WithinTransaction(func(repoTx affiliatecontract.Store) error {
		initial, err := repoTx.GetWithdrawByID(withdrawID)
		if err != nil {
			return err
		}
		if initial == nil {
			return ErrNotFound
		}
		// Refund holds order locks before entering Affiliate. Only lock the
		// profile here; read order states without FOR UPDATE below to avoid an
		// order -> profile / profile -> order deadlock on PostgreSQL.
		if err := repoTx.LockProfiles([]uint{initial.AffiliateProfileID}); err != nil {
			return err
		}
		req, err := repoTx.GetWithdrawByIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		if req == nil {
			return ErrNotFound
		}
		if req.Status != constants.AffiliateWithdrawStatusPendingReview {
			if (act == constants.AffiliateWithdrawActionPay && req.Status == constants.AffiliateWithdrawStatusPaid) ||
				(act == constants.AffiliateWithdrawActionReject && req.Status == constants.AffiliateWithdrawStatusRejected) {
				return nil
			}
			if req.RejectReason == WithdrawRefundRejectReason {
				return ErrWithdrawRefunded
			}
			if req.RejectReason == WithdrawLegacyRefundRejectReason {
				return ErrWithdrawLegacyRefund
			}
			return ErrWithdrawStatusInvalid
		}

		commissions, err := repoTx.ListCommissionsByWithdrawIDForUpdate(withdrawID)
		if err != nil {
			return err
		}
		ids := make([]uint, 0, len(commissions))
		total := decimal.Zero
		invalid, refunded, legacyHeld := len(commissions) == 0, false, false
		checked := make(map[uint]bool)
		for _, commission := range commissions {
			legacyHeld = legacyHeld || commission.InvalidReason == affiliatedomain.CommissionInvalidReasonLegacyRefundReview
			ids = append(ids, commission.ID)
			total = total.Add(commission.CommissionAmount.Decimal)
			if commission.AffiliateProfileID != req.AffiliateProfileID || commission.Status != constants.AffiliateCommissionStatusAvailable || commission.CommissionAmount.Decimal.LessThanOrEqual(decimal.Zero) {
				invalid = true
			}
			state, err := repoTx.GetOrderRefundState(commission.OrderID)
			if err != nil {
				return err
			}
			if state == nil {
				invalid = true
				continue
			}
			if target := commissionRefundTarget(state); target.GreaterThan(commission.OrderRefundedAmount.Decimal) {
				refunded = true
				if !checked[state.ID] {
					checked[state.ID] = true
					if err := s.reconcileOrderRefund(repoTx, state, target, "order_refunded"); err != nil {
						return err
					}
				}
			}
		}
		invalid = invalid || !total.Round(2).Equal(req.Amount.Decimal.Round(2))
		if refunded || invalid || legacyHeld {
			reason := WithdrawInvalidRejectReason
			reviewErr = ErrWithdrawStatusInvalid
			if refunded {
				reason = WithdrawRefundRejectReason
				reviewErr = ErrWithdrawRefunded
			}
			if legacyHeld {
				reason = WithdrawLegacyRefundRejectReason
				reviewErr = ErrWithdrawLegacyRefund
			}
			// Commit the rejection/reconciliation before returning the error to
			// the administrator. Returning it from the closure would roll back
			// the safety repair and leave the request payable on the next retry.
			return rejectWithdraw(repoTx, req, reason, time.Now())
		}

		now := time.Now()
		req.ProcessedBy = &adminID
		req.ProcessedAt = &now
		req.UpdatedAt = now
		if act == constants.AffiliateWithdrawActionReject {
			req.Status = constants.AffiliateWithdrawStatusRejected
			req.RejectReason = rejectReason
			if err := repoTx.BatchUpdateCommissions(ids, map[string]interface{}{
				"withdraw_request_id": nil,
				"updated_at":          now,
			}); err != nil {
				return err
			}
		} else {
			req.Status = constants.AffiliateWithdrawStatusPaid
			req.RejectReason = ""
			if err := repoTx.BatchUpdateCommissions(ids, map[string]interface{}{
				"status":     constants.AffiliateCommissionStatusWithdrawn,
				"updated_at": now,
			}); err != nil {
				return err
			}
		}
		return repoTx.UpdateWithdraw(req)
	})
	if err != nil {
		return nil, err
	}
	if reviewErr != nil {
		return nil, reviewErr
	}
	return s.repo.GetWithdrawByID(withdrawID)
}

// These machine reasons are localized by the transport, not persisted as one
// administrator's language in records shared by all users.
const (
	WithdrawRefundRejectReason       = "error.affiliate_withdraw_refunded"
	WithdrawInvalidRejectReason      = "error.affiliate_withdraw_invalid"
	WithdrawLegacyRefundRejectReason = "error.affiliate_withdraw_legacy_refund"
)

func rejectRefundedWithdraw(repoTx affiliatecontract.Store, req *affiliatedomain.WithdrawRequest, now time.Time) error {
	return rejectWithdraw(repoTx, req, WithdrawRefundRejectReason, now)
}

func rejectWithdraw(repoTx affiliatecontract.Store, req *affiliatedomain.WithdrawRequest, reason string, now time.Time) error {
	commissions, err := repoTx.ListCommissionsByWithdrawIDForUpdate(req.ID)
	if err != nil {
		return err
	}
	ids := make([]uint, 0, len(commissions))
	for _, commission := range commissions {
		if commission.Status != constants.AffiliateCommissionStatusWithdrawn {
			ids = append(ids, commission.ID)
		}
	}
	if err := repoTx.BatchUpdateCommissions(ids, map[string]interface{}{"withdraw_request_id": nil, "updated_at": now}); err != nil {
		return err
	}
	req.Status = constants.AffiliateWithdrawStatusRejected
	req.RejectReason = reason
	req.ProcessedBy = nil
	req.ProcessedAt = &now
	req.UpdatedAt = now
	return repoTx.UpdateWithdraw(req)
}

func commissionRefundTarget(state *affiliatecontract.OrderRefundState) decimal.Decimal {
	if state.Status == constants.OrderStatusRefunded || state.Status == constants.OrderStatusCanceled {
		return state.TotalAmount
	}
	return state.RefundedAmount
}

func containsWithdrawChannel(channels []string, channel string) bool {
	target := strings.ToLower(strings.TrimSpace(channel))
	if target == "" {
		return false
	}
	for _, item := range channels {
		if strings.ToLower(strings.TrimSpace(item)) == target {
			return true
		}
	}
	return false
}
