package application

import (
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/cart/contract"
	"github.com/dujiao-next/internal/modules/cart/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type uniformCartItems struct {
	contract.Repository
	item contract.StoredItem
}

func (s uniformCartItems) ListByUser(uint) ([]contract.StoredItem, error) {
	return []contract.StoredItem{s.item}, nil
}

type uniformCartPromotion struct {
	promotioncontract.Repository
	calls int
}

func (s *uniformCartPromotion) GetAllActiveByProduct(uint, time.Time) ([]promotiondomain.Promotion, error) {
	s.calls++
	return []promotiondomain.Promotion{{ID: 1, ScopeType: constants.ScopeTypeProduct, Type: constants.PromotionTypeFixed, Value: money.FromDecimal(decimal.NewFromInt(10))}}, nil
}

func TestCartUniformPricingUsesSKUPriceWithoutApplyingPromotion(t *testing.T) {
	items := uniformCartItems{item: contract.StoredItem{
		Item:    domain.Item{UserID: 1, ProductID: 2, SKUID: 3, Quantity: 2},
		Product: &productdomain.Product{ID: 2, IsActive: true, FulfillmentType: constants.FulfillmentTypeAuto, PriceAmount: money.FromDecimal(decimal.NewFromInt(100))},
		SKU:     &productdomain.ProductSKU{ID: 3, IsActive: true, PriceAmount: money.FromDecimal(decimal.NewFromInt(80))},
	}}
	for _, uniform := range []bool{true, false} {
		promotions := &uniformCartPromotion{}
		s := NewService(items, nil, nil, promotions, nil, uniform)
		details, err := s.ListByUser(1)
		if err != nil {
			t.Fatal(err)
		}
		want := "70.00"
		if uniform {
			want = "80.00"
		}
		if len(details) != 1 || details[0].UnitPrice.String() != want || details[0].OriginalPrice.String() != "80.00" {
			t.Fatalf("uniform=%v: cart prices=%#v, want %s", uniform, details, want)
		}
		if uniform && promotions.calls != 0 {
			t.Fatal("uniform cart should not query promotions")
		}
	}
}
