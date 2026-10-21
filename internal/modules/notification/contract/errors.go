package contract

import (
	"errors"

	settingsmessaging "github.com/zxgad3wdasaq1e-ai/faka-58/internal/modules/settings/schema/messaging"
)

var (
	ErrConfigInvalid          = settingsmessaging.ErrNotificationConfigInvalid
	ErrSendFailed             = errors.New("notification send failed")
	ErrEventInvalid           = errors.New("notification event invalid")
	ErrInvalidEmail           = errors.New("invalid email")
	ErrEmailServiceDisabled   = errors.New("email service disabled")
	ErrEmailNotConfigured     = errors.New("email service not configured")
	ErrEmailRecipientRejected = errors.New("email recipient rejected")
)
