package presenter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	affiliatedomain "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/domain"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestProfileOmitsSensitiveFields(t *testing.T) {
	profile := &affiliatedomain.Profile{
		ID:            1,
		UserID:        99,
		AffiliateCode: "AFF-001",
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	resp := NewProfile(profile)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	if strings.Contains(jsonStr, `"user_id"`) {
		t.Error("user_id should not appear")
	}
	if strings.Contains(jsonStr, `"updated_at"`) {
		t.Error("updated_at should not appear")
	}
	if resp.AffiliateCode != "AFF-001" {
		t.Errorf("expected code AFF-001, got %s", resp.AffiliateCode)
	}
}

func TestCommissionOmitsSensitiveFields(t *testing.T) {
	commission := &affiliatedomain.Commission{
		ID:                 1,
		AffiliateProfileID: 5,
		OrderID:            10,
		CommissionType:     "order",
		BaseAmount:         newMoney("100.00"),
		RatePercent:        newMoney("10.00"),
		CommissionAmount:   newMoney("10.00"),
		Status:             "available",
		CreatedAt:          time.Now(),
	}

	resp := NewCommission(commission)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	sensitiveFields := []string{"affiliate_profile_id", "base_amount", "rate_percent", "invalid_reason"}
	for _, field := range sensitiveFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("sensitive field %q should not appear", field)
		}
	}

	if !strings.Contains(jsonStr, `"commission_amount"`) {
		t.Error("commission_amount should appear")
	}
}

func TestCommissionReviewFlagDoesNotExposeInternalReason(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		wantReview   bool
	}{
		{"available", "", false},
		{"refund_requires_review", affiliatedomain.CommissionInvalidReasonLegacyRefundReview, true},
		{"other_internal_reason", "internal-order-diagnostic", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := NewCommissionList([]affiliatedomain.Commission{{
				ID: 1, Status: "available", InvalidReason: tc.reason,
				CommissionAmount: newMoney("12.00"),
			}})
			data, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			var payload []map[string]any
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if got, exists := payload[0]["requires_review"]; !exists || got != tc.wantReview {
				t.Fatalf("requires_review = %v, exists=%v, want %v", got, exists, tc.wantReview)
			}
			if _, exists := payload[0]["invalid_reason"]; exists {
				t.Fatal("internal invalid_reason leaked")
			}
			if tc.reason != "" && strings.Contains(string(data), tc.reason) {
				t.Fatal("internal reason value leaked")
			}
			if rows[0].Status != "available" || !rows[0].CommissionAmount.Decimal.Equal(decimal.NewFromInt(12)) {
				t.Fatal("presenter changed commission history")
			}
		})
	}
}

func TestWithdrawOmitsSensitiveFields(t *testing.T) {
	withdraw := &affiliatedomain.WithdrawRequest{
		ID:                 1,
		AffiliateProfileID: 5,
		Amount:             newMoney("50.00"),
		Channel:            "alipay",
		Account:            "user@alipay.com",
		Status:             "pending",
		ProcessedBy:        ptrUint(3),
		CreatedAt:          time.Now(),
	}

	resp := NewWithdraw(withdraw)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	if strings.Contains(jsonStr, `"affiliate_profile_id"`) {
		t.Error("affiliate_profile_id should not appear")
	}
	if strings.Contains(jsonStr, `"processed_by"`) {
		t.Error("processed_by should not appear")
	}
	if !strings.Contains(jsonStr, `"channel"`) {
		t.Error("channel should appear")
	}
}

func ptrUint(v uint) *uint {
	return &v
}

func newMoney(raw string) money.Amount {
	return money.FromDecimal(decimal.RequireFromString(raw))
}
