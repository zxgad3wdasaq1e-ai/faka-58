package adminauthwiring

import (
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/app/container"
	captchahttp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/captcha/transport/http"
	adminauthtransport "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/adminauth/transport/http"
)

type Handlers struct {
	Login     *adminauthtransport.AdminLoginHandler
	TwoFA     *adminauthtransport.Admin2FAHandler
	UserTwoFA *adminauthtransport.AdminUser2FAHandler
}

func New(c *container.Container) Handlers {
	recorder := adminLoginRecorderAdapter{logs: c.AdminLoginLogService}
	return Handlers{
		Login: adminauthtransport.NewAdminLoginHandler(
			adminLoginAuthTransportAdapter{auth: c.AuthService},
			captchahttp.NewVerifier(c.CaptchaService),
			recorder,
		),
		TwoFA: adminauthtransport.NewAdmin2FAHandler(
			admin2FATOTPTransportAdapter{totp: c.TOTPService},
			admin2FAAuthTransportAdapter{auth: c.AuthService},
			admin2FAChallengeStoreAdapter{},
			recorder,
		),
		UserTwoFA: adminauthtransport.NewAdminUser2FAHandler(
			adminUser2FATransportAdapter{totp: c.UserTOTPService},
		),
	}
}
