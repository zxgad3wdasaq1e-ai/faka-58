package container

import (
	"context"

	adproxyapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/adproxy/application"
	adproxygateway "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/adproxy/infrastructure/adgateway"
	categoryapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/catalog/category/application"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/logger"
	cardsecretapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/cardsecret/application"
	cartapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/cart/application"
	contentapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/content/application"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/content/infrastructure/gormstore"
	couponapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/coupon/application"
	fulfillmentapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/fulfillment/application"
	fulfillmentqueue "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/fulfillment/infrastructure/queueadapter"
	giftcardapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/giftcard/application"
	giftcardsettingscurrency "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/giftcard/infrastructure/settingscurrency"
	memberlevelapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/memberlevel/application"
	orderapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/application"
	orderrefund "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/application/refund"
	orderqueue "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/infrastructure/queueadapter"
	orderriskapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/orderrisk/application"
	orderrisklimiter "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/orderrisk/infrastructure/redislimiter"
	promotionapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/promotion/application"
	sitemapapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/sitemap/application"
	sitemapcontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/sitemap/contract"
	sitemapcache "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/sitemap/infrastructure/cacheadapter"
	sitemapcatalog "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/sitemap/infrastructure/catalogreader"
	walletapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/wallet/application"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/platform/database/gormdb"
	giftcardredeemgormuow "github.com/zxgad3wdasaq1e-ai/faka-58/internal/workflows/giftcardredeem/infrastructure/gormuow"
)

// initApplicationServices 装配内容、购物车、订单、履约和营销用例。
func (c *Container) initApplicationServices() {
	c.AdProxyService = adproxyapp.NewService(adproxygateway.New())
	postStore := gormstore.NewPostStore(gormdb.DB)
	postCategoryStore := gormstore.NewPostCategoryStore(gormdb.DB)
	c.ContentPostService = contentapp.NewPostService(
		postStore,
		postStore,
		postCategoryStore,
		contentapp.SystemClock{},
	)
	c.ContentPostCategoryService = contentapp.NewPostCategoryService(postCategoryStore)
	c.CategoryService = categoryapp.NewService(c.CategoryRepo)
	sitemapService, err := sitemapapp.NewService(
		sitemapcatalog.New(c.ProductRepo, c.CategoryRepo),
		sitemapcontract.PublishedPostReaderFunc(func(ctx context.Context, limit int) ([]sitemapcontract.PublishedPost, error) {
			posts, _, listErr := c.ContentPostService.ListPublic(ctx, contentapp.PublicPostQuery{
				Page:     1,
				PageSize: limit,
			})
			if listErr != nil {
				return nil, listErr
			}
			result := make([]sitemapcontract.PublishedPost, 0, len(posts))
			for _, post := range posts {
				result = append(result, sitemapcontract.PublishedPost{
					Slug:        post.Slug,
					CreatedAt:   post.CreatedAt,
					PublishedAt: post.PublishedAt,
				})
			}
			return result, nil
		}),
		sitemapcache.New(),
	)
	if err != nil {
		logger.Errorw("provider_init_sitemap_failed", "error", err)
		panic(err)
	}
	c.SitemapService = sitemapService
	c.CartService = cartapp.NewService(c.CartRepo, c.ProductRepo, c.ProductSKURepo, c.PromotionRepo, c.SettingService, c.Config.Order.UniformPricing)
	c.WalletService = walletapp.NewService(walletapp.Options{
		Repository: c.WalletRepo, Transactions: c.WalletRepo,
	})
	c.OrderRefundService = orderrefund.New(
		c.OrderStore,
		c.UserStore,
		c.AffiliateService,
		c.SettingService,
		c.WalletService,
		c.PaymentStore,
	)
	c.MemberLevelService = memberlevelapp.NewService(c.MemberLevelRepo, c.MemberLevelPriceRepo, c.MemberLevelUserRepo)
	c.OrderRiskControlService = orderriskapp.NewService(orderriskapp.Options{
		Settings:    c.SettingService,
		RateLimiter: orderrisklimiter.New(),
	})
	orderQueue := orderqueue.New(c.QueueClient)
	c.OrderService = orderapp.NewOrderService(orderapp.OrderServiceOptions{
		OrderStore:              c.OrderStore,
		UserStore:               c.UserStore,
		ProductStore:            c.ProductRepo,
		ProductSKUStore:         c.ProductSKURepo,
		CouponStore:             c.CouponRepo,
		CouponUsageStore:        c.CouponUsageRepo,
		PromotionRepo:           c.PromotionRepo,
		Queue:                   orderQueue,
		SettingService:          c.SettingService,
		DefaultEmailConfig:      c.Config.Email,
		WalletService:           c.WalletService,
		AffiliateService:        c.AffiliateService,
		MemberLevelService:      c.MemberLevelService,
		ResellerPricingResolver: c.ResellerPricingResolver,
		ResellerAccounting:      c.ResellerAccountingLedger,
		RiskControlService:      c.OrderRiskControlService,
		ExpireMinutes:           c.Config.Order.PaymentExpireMinutes,
		UniformPricing:          c.Config.Order.UniformPricing,
	})
	c.FulfillmentService = fulfillmentapp.New(fulfillmentapp.Options{
		OrderStore:            c.OrderStore,
		FulfillmentStore:      c.FulfillmentStore,
		OrderQueue:            orderQueue,
		BotNotifier:           fulfillmentqueue.NewBotNotifier(c.QueueClient),
		SettingService:        c.SettingService,
		DefaultEmailConfig:    c.Config.Email,
		ExternalIdentityStore: c.ExternalIdentityStore,
	})
	c.CardSecretService = cardsecretapp.NewService(cardsecretapp.ServiceOptions{
		Secrets:      c.CardSecretRepo,
		Batches:      c.CardSecretBatchRepo,
		Transactions: c.CardSecretRepo,
		Products:     c.ProductRepo,
		ProductSKUs:  c.ProductSKURepo,
	})
	c.GiftCardService = giftcardapp.NewService(giftcardapp.Options{
		Repo:     c.GiftCardRepo,
		Users:    c.UserStore,
		Currency: giftcardsettingscurrency.New(c.SettingService),
		Redeemer: giftcardredeemgormuow.New(c.GiftCardRepo, c.WalletService),
	})
	c.CouponAdminService = couponapp.NewAdminService(c.CouponRepo)
	c.PromotionAdminService = promotionapp.NewAdminService(c.PromotionRepo)
	c.ContentBannerService = contentapp.NewBannerService(
		gormstore.NewBannerStore(gormdb.DB),
		contentapp.SystemClock{},
	)
}
