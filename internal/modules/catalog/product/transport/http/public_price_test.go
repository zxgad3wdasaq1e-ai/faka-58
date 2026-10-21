package producthttp

import (
	"errors"
	"testing"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type stubResellerDisplayPricer struct {
	result *reseller.DisplayPriceResult
	err    error
}

func (s stubResellerDisplayPricer) LoadDisplayPricingBatch(tenant reseller.TenantContext, products []productdomain.Product) (*reseller.DisplayPricingBatch, error) {
	return &reseller.DisplayPricingBatch{Tenant: tenant}, nil
}

func (s stubResellerDisplayPricer) ResolveDisplayPrices(tenant reseller.TenantContext, product *productdomain.Product, batch *reseller.DisplayPricingBatch) (*reseller.DisplayPriceResult, error) {
	return s.result, s.err
}

func TestDecoratePublicProductDisplayPricePrefersFirstActiveSKU(t *testing.T) {
	h := &PublicHandler{}
	product := &productdomain.Product{
		ID:          1,
		PriceAmount: money.FromDecimal(decimal.RequireFromString("59.90")),
		SKUs: []productdomain.ProductSKU{
			{
				ID:          11,
				IsActive:    true,
				SortOrder:   100,
				PriceAmount: money.FromDecimal(decimal.RequireFromString("89.90")),
			},
			{
				ID:          12,
				IsActive:    true,
				SortOrder:   10,
				PriceAmount: money.FromDecimal(decimal.RequireFromString("49.90")),
			},
		},
	}

	item, err := h.decoratePublicProduct(product, nil)
	if err != nil {
		t.Fatalf("decoratePublicProduct failed: %v", err)
	}
	expected := decimal.RequireFromString("89.90")
	if !item.PriceAmount.Decimal.Equal(expected) {
		t.Fatalf("expected display price %s, got: %s", expected.String(), item.PriceAmount.String())
	}
}

func TestDecoratePublicProductForTenantUsesResellerPricesAndHidesMainDiscounts(t *testing.T) {
	product := &productdomain.Product{
		ID:          1,
		Slug:        "reseller-display",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
		},
		SKUs: []productdomain.ProductSKU{
			{
				ID:          11,
				ProductID:   1,
				SKUCode:     "VISIBLE",
				IsActive:    true,
				SortOrder:   100,
				PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
			},
			{
				ID:          12,
				ProductID:   1,
				SKUCode:     "HIDDEN",
				IsActive:    true,
				SortOrder:   10,
				PriceAmount: money.FromDecimal(decimal.NewFromInt(90)),
			},
		},
	}
	h := &PublicHandler{pricer: stubResellerDisplayPricer{result: &reseller.DisplayPriceResult{
		Visible:      true,
		ProductID:    1,
		DisplaySKUID: 11,
		DisplayPrice: money.FromDecimal(decimal.NewFromInt(130)),
		SKUPrices: map[uint]money.Amount{
			11: money.FromDecimal(decimal.NewFromInt(130)),
		},
		HiddenSKUIDs: map[uint]bool{12: true},
	}}}
	tenant := reseller.ResellerTenantContext("shop.example.test", 10, 99, "shop.example.test")

	item, err := h.decoratePublicProductForTenant(product, nil, tenant, &reseller.DisplayPricingBatch{})
	if err != nil {
		t.Fatalf("decoratePublicProductForTenant failed: %v", err)
	}
	if !item.PriceAmount.Decimal.Equal(decimal.NewFromInt(130)) {
		t.Fatalf("expected reseller product price 130, got %s", item.PriceAmount.String())
	}
	if len(item.SKUs) != 1 || item.SKUs[0].ID != 11 {
		t.Fatalf("expected only visible sku 11, got %+v", item.SKUs)
	}
	if !item.SKUs[0].PriceAmount.Decimal.Equal(decimal.NewFromInt(130)) {
		t.Fatalf("expected reseller sku price 130, got %s", item.SKUs[0].PriceAmount.String())
	}
	if item.PromotionPriceAmount != nil || item.PromotionID != nil || len(item.PromotionRules) > 0 {
		t.Fatalf("reseller display must not expose main promotion fields: %+v", item)
	}
	if len(item.WholesalePrices) > 0 {
		t.Fatalf("reseller display must not expose main wholesale prices: %+v", item.WholesalePrices)
	}
}

func TestDecoratePublicProductForTenantHiddenProduct(t *testing.T) {
	h := &PublicHandler{pricer: stubResellerDisplayPricer{result: &reseller.DisplayPriceResult{Visible: false}}}
	tenant := reseller.ResellerTenantContext("shop.example.test", 10, 99, "shop.example.test")
	product := &productdomain.Product{
		ID:          1,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		SKUs: []productdomain.ProductSKU{
			{ID: 11, ProductID: 1, IsActive: true, PriceAmount: money.FromDecimal(decimal.NewFromInt(100))},
		},
	}

	_, err := h.decoratePublicProductForTenant(product, nil, tenant, &reseller.DisplayPricingBatch{})
	if !errors.Is(err, productcontract.ErrResellerProductNotListed) {
		t.Fatalf("expected ErrResellerProductNotListed, got %v", err)
	}
}

func TestDecoratePublicProductForTenantAllHiddenSKUsReturnsNotListed(t *testing.T) {
	h := &PublicHandler{pricer: stubResellerDisplayPricer{result: &reseller.DisplayPriceResult{
		Visible:      true,
		DisplayPrice: money.FromDecimal(decimal.NewFromInt(100)),
		HiddenSKUIDs: map[uint]bool{11: true, 12: true},
	}}}
	tenant := reseller.ResellerTenantContext("shop.example.test", 10, 99, "shop.example.test")
	product := &productdomain.Product{
		ID:          1,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		SKUs: []productdomain.ProductSKU{
			{ID: 11, ProductID: 1, IsActive: true, PriceAmount: money.FromDecimal(decimal.NewFromInt(100))},
			{ID: 12, ProductID: 1, IsActive: true, PriceAmount: money.FromDecimal(decimal.NewFromInt(120))},
		},
	}

	_, err := h.decoratePublicProductForTenant(product, nil, tenant, &reseller.DisplayPricingBatch{})
	if !errors.Is(err, productcontract.ErrResellerProductNotListed) {
		t.Fatalf("expected ErrResellerProductNotListed, got %v", err)
	}
}

func TestDecoratePublicProductForTenantInvalidDisplayPricingIsHidden(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "below base", err: reseller.ErrPriceBelowBase},
		{name: "markup exceeds max", err: reseller.ErrMarkupExceeded},
		{name: "unknown pricing mode", err: reseller.ErrPricingModeInvalid},
		{name: "already not listed", err: productcontract.ErrResellerProductNotListed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &PublicHandler{pricer: stubResellerDisplayPricer{err: tt.err}}
			tenant := reseller.ResellerTenantContext("shop.example.test", 10, 99, "shop.example.test")
			product := &productdomain.Product{
				ID:          1,
				PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
				SKUs: []productdomain.ProductSKU{
					{ID: 11, ProductID: 1, IsActive: true, PriceAmount: money.FromDecimal(decimal.NewFromInt(100))},
				},
			}
			_, err := h.decoratePublicProductForTenant(product, nil, tenant, &reseller.DisplayPricingBatch{})
			if !errors.Is(err, productcontract.ErrResellerProductNotListed) {
				t.Fatalf("expected ErrResellerProductNotListed, got %v", err)
			}
		})
	}
}

type marketingPriceStub struct{ calls int }

func (s *marketingPriceStub) GetProductPromotions(uint) ([]promotiondomain.Promotion, error) {
	s.calls++
	return []promotiondomain.Promotion{{ID: 1}}, nil
}
func (s *marketingPriceStub) ApplyPromotion(product *productdomain.Product, _ int) (*promotiondomain.Promotion, money.Amount, error) {
	s.calls++
	return &promotiondomain.Promotion{ID: 1}, money.FromDecimal(product.PriceAmount.Decimal.Sub(decimal.NewFromInt(10))), nil
}
func (s *marketingPriceStub) GetLevelPricesByProduct(uint) ([]memberleveldomain.MemberLevelPrice, error) {
	s.calls++
	return []memberleveldomain.MemberLevelPrice{{MemberLevelID: 1, SKUID: 11, PriceAmount: money.FromDecimal(decimal.NewFromInt(60))}}, nil
}
func (s *marketingPriceStub) ResolveMemberPrice(_, _, _ uint, price decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	s.calls++
	return price.Sub(decimal.NewFromInt(20)), decimal.NewFromInt(20)
}

func TestUniformPublicPricingHidesMarketingAndKeepsSKUPrices(t *testing.T) {
	product := &productdomain.Product{
		ID: 1, PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: productdomain.WholesalePriceTiers{{MinQuantity: 2, UnitPrice: money.FromDecimal(decimal.NewFromInt(50))}},
		SKUs:            []productdomain.ProductSKU{{ID: 11, IsActive: true, PriceAmount: money.FromDecimal(decimal.NewFromInt(80))}},
	}
	for _, uniform := range []bool{true, false} {
		marketing := &marketingPriceStub{}
		h := &PublicHandler{uniformPricing: uniform, memberLevels: marketing}
		item, err := h.decoratePublicProduct(product, marketing, 1)
		if err != nil {
			t.Fatal(err)
		}
		if item.PriceAmount.String() != "80.00" || len(item.SKUs) != 1 || item.SKUs[0].PriceAmount.String() != "80.00" {
			t.Fatalf("SKU base price changed: %#v", item)
		}
		if uniform {
			if marketing.calls != 0 || len(item.WholesalePrices) != 0 || len(item.MemberPrices) != 0 || len(item.PromotionRules) != 0 || item.PromotionPriceAmount != nil || item.SKUs[0].MemberPriceAmount != nil || item.SKUs[0].PromotionPriceAmount != nil {
				t.Fatalf("uniform prices expose marketing: %#v calls=%d", item, marketing.calls)
			}
		} else if marketing.calls == 0 || len(item.WholesalePrices) != 1 || len(item.MemberPrices) != 1 || item.PromotionPriceAmount == nil || item.SKUs[0].MemberPriceAmount == nil {
			t.Fatalf("legacy marketing missing: %#v", item)
		}
		if len(product.WholesalePrices) != 1 {
			t.Fatal("decorating must not mutate stored wholesale prices")
		}
	}
}
