package container

import (
	catalogmappingbootstrap "github.com/zxgad3wdasaq1e-ai/faka-58/internal/bootstrap/catalogmapping"
	telegrambroadcast "github.com/zxgad3wdasaq1e-ai/faka-58/internal/bootstrap/telegrambroadcast"
	uploadbootstrap "github.com/zxgad3wdasaq1e-ai/faka-58/internal/bootstrap/upload"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/logger"
	apicredentialapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/apicredential/application"
	auditlogapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/auditlog/application"
	channelclientapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/channelclient/application"
	contentapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/content/application"
	localfilestore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/content/infrastructure/filestore/local"
	contentgormstore "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/content/infrastructure/gormstore"
	dashboardapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/dashboard/application"
	downstreamcallbackapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/downstreamcallback/application"
	downstreamcallbackcontract "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/downstreamcallback/contract"
	downstreamcallbackclient "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/downstreamcallback/infrastructure/callbackclient"
	downstreamcallbackcredentialreader "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/downstreamcallback/infrastructure/credentialreader"
	downstreamcallbackorderreader "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/downstreamcallback/infrastructure/orderreader"
	downstreamcallbackqueue "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/downstreamcallback/infrastructure/queueadapter"
	notificationapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/notification/application"
	notificationasyncqueue "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/notification/infrastructure/asyncqueue"
	notificationfeishu "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/notification/infrastructure/feishu"
	paymentapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/payment/application"
	paymentqueue "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/payment/infrastructure/queueadapter"
	procurementapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/procurement/application"
	procurementmapping "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/procurement/infrastructure/mappingreader"
	procurementnotification "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/procurement/infrastructure/notificationadapter"
	procurementorder "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/procurement/infrastructure/orderreader"
	procurementqueue "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/procurement/infrastructure/queueadapter"
	procurementupstream "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/procurement/infrastructure/upstreamgateway"
	reconciliationapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reconciliation/application"
	reconciliationnotification "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reconciliation/infrastructure/notificationadapter"
	reconciliationprocurement "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reconciliation/infrastructure/procurementreader"
	reconciliationqueue "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reconciliation/infrastructure/queueadapter"
	reconciliationupstream "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/reconciliation/infrastructure/upstreamreader"
	siteconnectionapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/siteconnection/application"
	broadcastapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/telegram/broadcast/application"
	notifyapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/telegram/notify/application"
	notifybotapi "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/telegram/notify/infrastructure/botapi"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/platform/database/gormdb"
)

// initIntegrationServices 装配通知、站点对接、支付、采购、渠道与 Telegram 集成。
func (c *Container) initIntegrationServices() {
	c.UserLoginLogService = auditlogapp.NewUserLoginService(c.UserLoginLogRepo)
	c.AuthzAuditService = auditlogapp.NewAuthzService(c.AuthzAuditLogRepo)
	c.AdminLoginLogService = auditlogapp.NewAdminLoginService(c.AdminLoginLogRepo)
	c.NotificationLogService = notificationapp.NewLogService(c.NotificationLogRepo)
	c.DashboardService = dashboardapp.NewService(c.DashboardRepo, c.SettingService)
	telegramNotifyService := notifyapp.NewService(c.SettingService, c.Config.TelegramAuth, notifybotapi.New())
	c.NotificationService = notificationapp.NewService(
		c.SettingService,
		c.EmailSender,
		notificationasyncqueue.New(c.QueueClient),
		c.DashboardService,
		c.NotificationLogService,
		telegramNotifyService,
		notificationfeishu.New(),
	)
	c.ApiCredentialService = apicredentialapp.NewService(c.ApiCredentialRepo)
	c.SiteConnectionService = siteconnectionapp.NewService(c.SiteConnectionRepo, c.Config.App.SecretKey, "uploads")
	mediaFiles, err := uploadbootstrap.NewMediaFiles(c.Config.Upload, localfilestore.New())
	if err != nil {
		logger.Errorw("provider_init_media_files_failed", "error", err)
		panic(err)
	}
	mediaCore := contentapp.NewMediaService(
		contentgormstore.NewMediaStore(gormdb.DB),
		mediaFiles,
		contentapp.WarningLoggerFunc(logger.Warnw),
	)
	c.ContentMediaService = mediaCore
	productMappingService, err := catalogmappingbootstrap.New(catalogmappingbootstrap.Dependencies{
		Mappings:    c.ProductMappingRepo,
		SKUMappings: c.SKUMappingRepo,
		Products:    c.ProductRepo,
		SKUs:        c.ProductSKURepo,
		Categories:  c.CategoryRepo,
		Connections: c.SiteConnectionService,
		Media:       mediaCore,
	})
	if err != nil {
		logger.Errorw("provider_init_product_mapping_failed", "error", err)
		panic(err)
	}
	c.ProductMappingService = productMappingService
	c.ProductMappingService.SetCategoryCreator(c.CategoryService)
	c.ProductMappingService.SetSettings(c.SettingService)
	c.SiteConnectionService.SetMarkupReapplier(c.ProductMappingService)
	c.OrderService.SetProductMappingService(c.ProductMappingService)
	var downstreamQueue downstreamcallbackcontract.CallbackQueue
	if c.QueueClient != nil {
		downstreamQueue = downstreamcallbackqueue.New(c.QueueClient)
	}
	c.DownstreamCallbackService = downstreamcallbackapp.NewService(downstreamcallbackapp.Options{
		References:  c.DownstreamOrderRefRepo,
		Orders:      downstreamcallbackorderreader.New(c.OrderStore),
		Credentials: downstreamcallbackcredentialreader.New(c.ApiCredentialRepo),
		Queue:       downstreamQueue,
		Deliverer:   downstreamcallbackclient.New(),
	})
	c.PaymentService = paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:              c.OrderStore,
		ProductRepo:             c.ProductRepo,
		ProductSKURepo:          c.ProductSKURepo,
		PaymentStore:            c.PaymentStore,
		ChannelStore:            c.PaymentChannelStore,
		WalletRepo:              c.WalletRepo,
		UserStore:               c.UserStore,
		ExternalIdentityStore:   c.ExternalIdentityStore,
		Queue:                   paymentqueue.New(c.QueueClient),
		WalletService:           c.WalletService,
		SettingService:          c.SettingService,
		DefaultEmailConfig:      c.Config.Email,
		ExpireMinutes:           c.Config.Order.PaymentExpireMinutes,
		AffiliateService:        c.AffiliateService,
		NotificationService:     c.NotificationService,
		PaymentProviderRegistry: c.PaymentProviderRegistry,
		ResellerAccounting:      c.ResellerAccountingLedger,
	})
	c.ProcurementOrderService = procurementapp.NewService(procurementapp.Options{
		Repository:         c.ProcurementOrderRepo,
		Orders:             procurementorder.New(c.OrderStore),
		ProductMappings:    procurementmapping.NewProducts(c.ProductMappingRepo),
		SKUMappings:        procurementmapping.NewSKUs(c.SKUMappingRepo),
		Connections:        procurementupstream.New(c.SiteConnectionService),
		Queue:              procurementqueue.New(c.QueueClient),
		OrderLifecycle:     c.ProcurementOrderRepo.NewLifecycle(c.QueueClient, c.SettingService, c.Config.Email),
		DownstreamCallback: c.DownstreamCallbackService,
		BotNotifier:        c.FulfillmentService,
		Notifications:      procurementnotification.New(c.NotificationService),
	})
	c.ReconciliationService = reconciliationapp.NewService(reconciliationapp.Options{
		Jobs: c.ReconciliationJobRepo, Items: c.ReconciliationItemRepo,
		Procurements:  reconciliationprocurement.New(c.ProcurementOrderRepo),
		Upstream:      reconciliationupstream.New(c.SiteConnectionService),
		Queue:         reconciliationqueue.New(c.QueueClient),
		Notifications: reconciliationnotification.New(c.NotificationService),
	})
	c.ChannelClientService = channelclientapp.NewService(c.ChannelClientStore, c.Config.App.SecretKey)
	c.TelegramBroadcastService = broadcastapp.NewService(
		c.TelegramBroadcastRepo,
		telegrambroadcast.NewUserDirectory(c.ExternalIdentityStore),
		telegrambroadcast.NewBotTokenResolver(c.ChannelClientService),
		telegrambroadcast.NewDispatcher(c.QueueClient),
		telegramNotifyService,
	)
}
