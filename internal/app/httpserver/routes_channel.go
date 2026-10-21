package httpserver

import (
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/app/container"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/app/httpserver/middleware"
	affiliatetransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/affiliate/transport/http"
	channeltransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/channelapi/transport/http"
	giftcardtransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/giftcard/transport/http"
	memberleveltransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/memberlevel/transport/http"
	telegramtransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/telegram/channelbot/transport/http"
	wallettransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/wallet/transport/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func registerChannelRoutes(
	apiV1 *gin.RouterGroup,
	c *container.Container,
	channelHandler *channeltransport.Handler,
	channelMemberLevelHandler *memberleveltransport.ChannelHandler,
	channelGiftCardHandler *giftcardtransport.ChannelHandler,
	channelAffiliateHandler *affiliatetransport.ChannelHandler,
	channelTelegramBotHandler *telegramtransport.ChannelBotHandler,
	channelWalletHandler *wallettransport.ChannelHandler,
	redisClient *redis.Client,
	channelAPIRule middleware.RateLimitRule,
) {
	// 渠道 API（Telegram Bot 等外部服务调用）
	channelAPI := apiV1.Group("/channel")
	channelAPI.Use(middleware.RateLimitMiddleware(redisClient, channelAPIRule, middleware.KeyByIPAndHeader("Dujiao-Next-Channel-Key")))
	channelAPI.Use(middleware.ChannelAPIAuthMiddleware(c))
	{
		telegramtransport.RegisterChannelBotRoutes(channelAPI, channelTelegramBotHandler)
		channeltransport.RegisterRoutes(channelAPI, channelHandler)
		affiliatetransport.RegisterChannelRoutes(channelAPI, channelAffiliateHandler)

		// Catalog 端点（商品浏览）
		memberleveltransport.RegisterChannelRoutes(channelAPI, channelMemberLevelHandler)

		// Order / Payment 端点（购买流程）

		// Wallet 端点（钱包）
		wallettransport.RegisterChannelRoutes(channelAPI, channelWalletHandler)
		giftcardtransport.RegisterChannelRoutes(channelAPI, channelGiftCardHandler)
	}
}
