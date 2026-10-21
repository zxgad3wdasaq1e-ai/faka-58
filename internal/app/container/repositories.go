package container

import (
	"fmt"

	affiliategormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/infrastructure/gormstore"
	apicredentialgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/apicredential/infrastructure/gormstore"
	auditloggormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/auditlog/infrastructure/gormstore"
	cardsecretgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/cardsecret/infrastructure/gormstore"
	cartgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/cart/infrastructure/gormstore"
	categorygormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/catalog/category/infrastructure/gormstore"
	mappinggormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/catalog/mapping/infrastructure/gormstore"
	productgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/catalog/product/store/gormstore"
	channelclientstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/channelclient/infrastructure/gormstore"
	coupongormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/coupon/infrastructure/gormstore"
	dashboardgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/dashboard/infrastructure/gormstore"
	downstreamcallbackgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/downstreamcallback/infrastructure/gormstore"
	fulfillmentgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/fulfillment/infrastructure/gormstore"
	giftcardgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/giftcard/infrastructure/gormstore"
	adminstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/admin/infrastructure/gormstore"
	emailverificationstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitystore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/externalidentity/infrastructure/gormstore"
	userstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/user/infrastructure/gormstore"
	memberlevelgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/memberlevel/infrastructure/gormstore"
	notificationgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/notification/infrastructure/gormstore"
	ordergormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/order/infrastructure/gormstore"
	paymentgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/payment/infrastructure/gormstore"
	procurementgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/procurement/infrastructure/gormstore"
	promotiongormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/promotion/infrastructure/gormstore"
	reconciliationgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reconciliation/infrastructure/gormstore"
	resellergormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reseller/infrastructure/gormstore"
	settingsstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/infrastructure/gormstore"
	siteconnectiongormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/siteconnection/infrastructure/gormstore"
	broadcaststore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/telegram/broadcast/infrastructure/gormstore"
	walletgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/wallet/infrastructure/gormstore"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/platform/database/gormdb"
)

func (c *Container) initRepositories() error {
	db := gormdb.DB
	c.AdminStore = adminstore.New(db)
	c.UserStore = userstore.New(db)
	c.ExternalIdentityStore = externalidentitystore.New(db)
	c.EmailVerificationStore = emailverificationstore.New(db)
	orderStore := ordergormstore.New(db, c.Config.App.SecretKey)
	if _, err := orderStore.BackfillGuestCredentialHashes(); err != nil {
		return fmt.Errorf("backfill guest order credentials: %w", err)
	}
	c.OrderStore = orderStore
	c.PaymentStore = paymentgormstore.New(db, c.Config.App.SecretKey)
	c.PaymentChannelStore = paymentgormstore.NewChannelStore(db)
	c.CardSecretRepo = cardsecretgormstore.New(db)
	c.CardSecretBatchRepo = cardsecretgormstore.NewBatch(db)
	c.GiftCardRepo = giftcardgormstore.New(db)
	c.FulfillmentStore = fulfillmentgormstore.New(db)
	c.ProductRepo = productgormstore.NewProductStore(db)
	c.ProductSKURepo = productgormstore.NewSKUStore(db)
	c.CartRepo = cartgormstore.New(db)
	c.CouponRepo = coupongormstore.New(db)
	c.CouponUsageRepo = coupongormstore.NewUsageStore(db)
	c.PromotionRepo = promotiongormstore.New(db)
	c.WalletRepo = walletgormstore.New(db)
	c.CategoryRepo = categorygormstore.NewCategoryStore(db)
	c.SettingRepo = settingsstore.New(db)
	c.UserLoginLogRepo = auditloggormstore.NewUserLoginStore(db)
	c.AuthzAuditLogRepo = auditloggormstore.NewAuthzStore(db)
	c.NotificationLogRepo = notificationgormstore.NewLogStore(db)
	c.AdminLoginLogRepo = auditloggormstore.NewAdminLoginStore(db)
	c.DashboardRepo = dashboardgormstore.New(db)
	c.AffiliateRepo = affiliategormstore.New(db)
	c.ResellerStore = resellergormstore.New(db)
	c.ApiCredentialRepo = apicredentialgormstore.New(db)
	c.SiteConnectionRepo = siteconnectiongormstore.New(db)
	c.ProductMappingRepo = mappinggormstore.NewMappingStore(db)
	c.SKUMappingRepo = mappinggormstore.NewSKUMappingStore(db)
	c.ProcurementOrderRepo = procurementgormstore.New(db)
	c.DownstreamOrderRefRepo = downstreamcallbackgormstore.New(db)
	c.ReconciliationJobRepo = reconciliationgormstore.NewJobStore(db)
	c.ReconciliationItemRepo = reconciliationgormstore.NewItemStore(db)
	c.ChannelClientStore = channelclientstore.New(db)
	c.TelegramBroadcastRepo = broadcaststore.New(db)
	c.MemberLevelRepo = memberlevelgormstore.NewLevelStore(db)
	c.MemberLevelPriceRepo = memberlevelgormstore.NewPriceStore(db)
	c.MemberLevelUserRepo = memberlevelgormstore.NewUserStore(db)
	return nil
}
