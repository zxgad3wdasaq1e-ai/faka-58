package settingsbootstrap

import (
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/config"
	telegramauthapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/identity/telegramauth/application"
	settingsapp "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/application"
	settingssecurity "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/schema/security"
)

type settingsTelegramAuthAdapter struct {
	settings     *settingsapp.Service
	cfg          *config.Config
	telegramAuth *telegramauthapp.Service
}

func (a settingsTelegramAuthAdapter) GetTelegramAuthSetting() (settingssecurity.TelegramAuthSetting, error) {
	return a.settings.GetTelegramAuthSetting(a.cfg.TelegramAuth)
}

func (a settingsTelegramAuthAdapter) PatchTelegramAuthSetting(patch settingssecurity.TelegramAuthSettingPatch) (settingssecurity.TelegramAuthSetting, error) {
	return a.settings.PatchTelegramAuthSetting(a.cfg.TelegramAuth, patch)
}

func (a settingsTelegramAuthAdapter) ApplyRuntime(setting settingssecurity.TelegramAuthSetting) {
	a.cfg.TelegramAuth = settingssecurity.TelegramAuthSettingToConfig(setting)
	if a.telegramAuth != nil {
		a.telegramAuth.SetConfig(a.cfg.TelegramAuth)
	}
}
