package affiliatebootstrap

import (
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/app/container"
	affiliatetransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/transport/http"
)

func NewStorefrontHandler(c *container.Container) *affiliatetransport.Handler {
	return affiliatetransport.NewHandler(c.AffiliateService)
}

func NewAdminHandler(c *container.Container) *affiliatetransport.AdminHandler {
	return affiliatetransport.NewAdminHandler(c.AffiliateService)
}

func NewChannelHandler(c *container.Container) *affiliatetransport.ChannelHandler {
	return affiliatetransport.NewChannelHandler(
		c.AffiliateService,
		affiliateChannelUserAdapter{auth: c.UserAuthService},
		c.SettingService,
	)
}
