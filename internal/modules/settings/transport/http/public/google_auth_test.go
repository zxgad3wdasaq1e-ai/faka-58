package publicconfighttp

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/shared/jsonmap"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type googleAuthPublicStub map[string]interface{}

func (s googleAuthPublicStub) PublicConfig() map[string]interface{} {
	return map[string]interface{}(s)
}

func TestResolveGoogleAuthPublicConfig(t *testing.T) {
	tests := []struct {
		name     string
		source   GoogleAuthPublic
		fallback GoogleAuthFallback
		want     map[string]interface{}
	}{
		{
			name:     "fallback enabled",
			fallback: GoogleAuthFallback{Enabled: true, ClientID: " fallback-client "},
			want:     map[string]interface{}{"enabled": true, "client_id": "fallback-client"},
		},
		{
			name:     "service overrides fallback",
			source:   googleAuthPublicStub{"enabled": true, "client_id": " runtime-client ", "unexpected": "private"},
			fallback: GoogleAuthFallback{Enabled: false, ClientID: "fallback-client"},
			want:     map[string]interface{}{"enabled": true, "client_id": "runtime-client"},
		},
		{
			name:     "missing client id fails closed",
			source:   googleAuthPublicStub{"enabled": true, "client_id": ""},
			fallback: GoogleAuthFallback{Enabled: true, ClientID: "fallback-client"},
			want:     map[string]interface{}{"enabled": false, "client_id": ""},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveGoogleAuthPublicConfig(test.source, test.fallback); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("public config = %#v, want %#v", got, test.want)
			}
		})
	}
}

type featureConfigCache struct{ data map[string]interface{} }

func (*featureConfigCache) CacheKey(*uint) string { return "public-config" }
func (s *featureConfigCache) GetJSON(_ context.Context, _ string, dest *map[string]interface{}) (bool, error) {
	if s.data == nil {
		return false, nil
	}
	*dest = s.data
	return true, nil
}
func (s *featureConfigCache) SetJSON(_ context.Context, _ string, value interface{}, _ time.Duration) error {
	s.data = value.(map[string]interface{})
	return nil
}

type featureSettings struct{}

func (featureSettings) GetConfig(defaults map[string]interface{}) (map[string]interface{}, error) {
	return defaults, nil
}
func (featureSettings) GetWalletRechargeChannelIDs() []uint { return nil }
func (featureSettings) GetWalletOnlyPayment() bool          { return false }
func (featureSettings) GetAffiliateSettingMap() (map[string]interface{}, error) {
	return map[string]interface{}{"enabled": true}, nil
}
func (featureSettings) GetSMTPEnabled() bool                           { return false }
func (featureSettings) GetRegistrationEnabled(bool) (bool, error)      { return true, nil }
func (featureSettings) GetEmailVerificationEnabled(bool) (bool, error) { return true, nil }
func (featureSettings) GetRegistrationInviteOnly() (bool, error)       { return false, nil }
func (featureSettings) GetRegistrationEmailDomainPolicy() (bool, []string, error) {
	return false, nil, nil
}
func (featureSettings) GetByKey(string) (interface{}, error)            { return nil, nil }
func (featureSettings) GetActiveHomeAnnouncement() (jsonmap.JSON, bool) { return nil, false }

type featurePayments struct{}

func (featurePayments) GetOrderPaymentChannels() ([]map[string]interface{}, error) { return nil, nil }

func TestPublicConfigUsesStartupFeaturesOnCacheMissAndHit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := &featureConfigCache{}
	for _, features := range []StoreFeatures{
		{UniformPricing: true, ResellerEnabled: false},
		{UniformPricing: false, ResellerEnabled: true},
		{UniformPricing: true, ResellerEnabled: false},
	} {
		handler := NewHandler(cache, featureSettings{}, featurePayments{}, nil, nil, TelegramAuthFallback{}, nil, GoogleAuthFallback{}, nil, features)
		router := gin.New()
		router.GET("/config", handler.GetConfig)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/config", nil))
		var payload struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Data["uniform_pricing"] != features.UniformPricing || payload.Data["reseller_enabled"] != features.ResellerEnabled {
			t.Fatalf("features=%#v response=%s", features, recorder.Body.String())
		}
		affiliate, _ := payload.Data["affiliate"].(map[string]interface{})
		if affiliate["enabled"] != true {
			t.Fatalf("affiliate should remain enabled: %s", recorder.Body.String())
		}
	}
}
