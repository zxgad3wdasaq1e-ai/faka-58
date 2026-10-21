package application

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestUniformPricingIgnoresMarketingForGuestsAndMembers(t *testing.T) {
	promotionPercent := decimal.NewFromInt(10)
	memberRate := decimal.NewFromInt(80)
	fixture := setupWholesaleOrderFixture(t, "uniform_pricing", productdomain.WholesalePriceTiers{{
		MinQuantity: 2, UnitPrice: money.FromDecimal(decimal.NewFromInt(70)),
	}}, &promotionPercent, &memberRate)
	if err := fixture.db.Model(&fixture.product).Update("purchase_type", constants.ProductPurchaseGuest).Error; err != nil {
		t.Fatal(err)
	}
	// The SKU is authoritative even when the legacy product price differs.
	if err := fixture.db.Model(&fixture.sku).Update("price_amount", "125.00").Error; err != nil {
		t.Fatal(err)
	}
	coupon := createWholesaleOrderCouponFixture(t, fixture.db, []uint{fixture.product.ID}, false)
	fixture.svc.uniformPricing = true
	for _, guest := range []bool{false, true} {
		input := orderCreateParams{
			UserID: fixture.user.ID, IsGuest: guest,
			GuestEmail: "guest@example.com", GuestPassword: "guest-password",
			Items: []CreateOrderItem{{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 2}},
		}
		result, err := fixture.svc.buildOrderResult(input)
		if err != nil {
			t.Fatalf("guest=%v: %v", guest, err)
		}
		if result.TotalAmount.StringFixed(2) != "250.00" || result.OriginalAmount.StringFixed(2) != "250.00" {
			t.Fatalf("guest=%v: original=%s total=%s, want SKU total 250.00", guest, result.OriginalAmount, result.TotalAmount)
		}
		if !result.MemberDiscountAmount.IsZero() || !result.PromotionDiscountAmount.IsZero() || !result.WholesaleDiscountAmount.IsZero() || !result.DiscountAmount.IsZero() || result.MemberLevelID != nil || result.OrderPromotionID != nil || result.AppliedCoupon != nil {
			t.Fatalf("guest=%v: unexpected marketing snapshot: %#v", guest, result)
		}
		item := result.Plans[0].Item
		if item.UnitPrice.String() != "125.00" || item.TotalPrice.String() != "250.00" || !item.MemberDiscount.IsZero() || !item.PromotionDiscount.IsZero() || !item.WholesaleDiscount.IsZero() || !item.CouponDiscount.IsZero() || item.PromotionID != nil {
			t.Fatalf("guest=%v: unexpected item prices: %#v", guest, item)
		}
		input.CouponCode = coupon.Code
		if _, err := fixture.svc.buildOrderResult(input); !errors.Is(err, couponcontract.ErrInvalid) {
			t.Fatalf("guest=%v: coupon error=%v, want invalid coupon", guest, err)
		}
	}

	fixture.svc.uniformPricing = false
	legacy, err := fixture.svc.buildOrderResult(orderCreateParams{
		UserID: fixture.user.ID,
		Items:  []CreateOrderItem{{ProductID: fixture.product.ID, SKUID: fixture.sku.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacy.TotalAmount.StringFixed(2) != "112.00" || legacy.MemberDiscountAmount.IsZero() || legacy.WholesaleDiscountAmount.IsZero() {
		t.Fatalf("legacy pricing should apply wholesale and member discounts: %#v", legacy)
	}
}
