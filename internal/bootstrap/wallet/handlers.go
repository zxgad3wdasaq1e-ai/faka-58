package walletbootstrap

import (
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/app/container"
	channeluserwiring "github.com/zxgad3wdasaq1e-ai/faka-58/internal/bootstrap/channeluser"
	wallettransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/wallet/transport/http"
)

type Handlers struct {
	User    *wallettransport.UserHandler
	Admin   *wallettransport.AdminHandler
	Channel *wallettransport.ChannelHandler
}

func New(c *container.Container) Handlers {
	wallets := walletTransportAdapter{wallets: c.WalletService, payments: c.PaymentService}
	return Handlers{
		User: wallettransport.NewUserHandler(
			wallets, wallets, c.UserStore, c.SettingService,
		),
		Admin: wallettransport.NewAdminHandler(
			wallets, c.UserStore, c.PaymentChannelStore, c.PaymentStore, c.SettingService,
		),
		Channel: wallettransport.NewChannelHandler(
			wallets,
			wallets,
			channeluserwiring.NewSimpleProvisioner(c.UserAuthService),
			c.SettingService,
		),
	}
}
