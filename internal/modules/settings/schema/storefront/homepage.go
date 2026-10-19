package settingsstorefront

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"

	"github.com/dujiao-next/internal/constants"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
)

const (
	settingHomepageHeroPhrasesMaxCount      = 8
	settingHomepageReviewsMaxCount          = 24
	settingHomepageActivityNamesMaxCount    = 40
	settingHomepageActivityMessagesMaxCount = 12
)

var settingHomepageReviewIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// NormalizeHomepage 只保留主题需要的文本和交互参数，不接受 HTML、图片或链接字段。
func NormalizeHomepage(raw interface{}) map[string]interface{} {
	value, _ := raw.(map[string]interface{})
	result := map[string]interface{}{
		"hero_phrases":             normalizeHomepageHeroPhrases(value["hero_phrases"]),
		"hero_interval_ms":         normalizeHomepageInteger(value["hero_interval_ms"], 1000, 10000, 2600),
		"reviews_enabled":          true,
		"reviews_demo":             true,
		"reviews_title":            normalizeHomepageLocalizedText(value["reviews_title"], 200),
		"reviews_subtitle":         normalizeHomepageLocalizedText(value["reviews_subtitle"], 500),
		"reviews_duration_seconds": normalizeHomepageInteger(value["reviews_duration_seconds"], 20, 120, 55),
		"reviews":                  normalizeHomepageReviews(value["reviews"]),
		"activity_enabled":         true,
		"activity_interval_ms":     normalizeHomepageInteger(value["activity_interval_ms"], 2000, 15000, 4200),
		"activity_names":           normalizeHomepageActivityNames(value["activity_names"]),
		"activity_messages":        normalizeHomepageActivityMessages(value["activity_messages"]),
	}
	for _, field := range []string{"reviews_enabled", "reviews_demo", "activity_enabled"} {
		if setting, ok := value[field]; ok && setting != nil {
			result[field] = settingsvalue.ParseBool(setting)
		}
	}
	return result
}

func normalizeHomepageActivityNames(raw interface{}) []string {
	result := make([]string, 0, settingHomepageActivityNamesMaxCount)
	for _, name := range settingsvalue.NormalizeStringList(raw) {
		name = settingsvalue.NormalizeTextWithRuneLimit(name, 30)
		if name == "" {
			continue
		}
		result = append(result, name)
		if len(result) == settingHomepageActivityNamesMaxCount {
			break
		}
	}
	if len(result) == 0 {
		return []string{"晚风送信", "小林同学", "wx_kai109", "青山不老", "Mia_2026", "小周周", "13752861042", "星河旅人", "chenyu_88", "阿橙"}
	}
	return result
}

func normalizeHomepageActivityMessages(raw interface{}) []interface{} {
	items, _ := raw.([]interface{})
	result := make([]interface{}, 0, settingHomepageActivityMessagesMaxCount)
	for _, item := range items {
		message, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		text := normalizeHomepageLocalizedText(message["text"], 200)
		if homepageLocalizedTextIsEmpty(text) {
			continue
		}
		kind := settingsvalue.NormalizeTextWithRuneLimit(message["kind"], 20)
		if kind != "order" && kind != "presence" && kind != "notice" {
			kind = "notice"
		}
		result = append(result, map[string]interface{}{"kind": kind, "text": text})
		if len(result) == settingHomepageActivityMessagesMaxCount {
			break
		}
	}
	if len(result) == 0 {
		return defaultHomepageActivityMessages()
	}
	return result
}

func defaultHomepageActivityMessages() []interface{} {
	defaults := []struct {
		kind string
		zhCN string
		zhTW string
		enUS string
	}{
		{"order", "{name} 下单了 ChatGPT Plus 订阅", "{name} 下單了 ChatGPT Plus 訂閱", "{name} ordered a ChatGPT Plus subscription"},
		{"presence", "目前有 {count} 位用户正在充值", "目前有 {count} 位使用者正在儲值", "{count} users are currently recharging"},
		{"notice", "正规渠道充值，不成功100%退款", "正規管道儲值，不成功100%退款", "Official-channel recharges. Full refund if unsuccessful."},
		{"notice", "支持 ChatGPT、Claude、Cursor、Gemini 等主流工具", "支援 ChatGPT、Claude、Cursor、Gemini 等主流工具", "Supports ChatGPT, Claude, Cursor, Gemini and other popular tools"},
		{"notice", "7×24 小时全天候自动充值服务！", "7×24 小時全天候自動儲值服務！", "24/7 automatic recharge service!"},
	}
	result := make([]interface{}, 0, len(defaults))
	for _, message := range defaults {
		result = append(result, map[string]interface{}{
			"kind": message.kind,
			"text": map[string]interface{}{"zh-CN": message.zhCN, "zh-TW": message.zhTW, "en-US": message.enUS},
		})
	}
	return result
}

func normalizeHomepageLocalizedText(raw interface{}, maxRunes int) map[string]interface{} {
	result := normalizeAnnouncementLocalizedField(raw)
	for _, language := range constants.SupportedLocales {
		result[language] = settingsvalue.NormalizeTextWithRuneLimit(result[language], maxRunes)
	}
	return result
}

func homepageLocalizedTextIsEmpty(value map[string]interface{}) bool {
	for _, language := range constants.SupportedLocales {
		if value[language] != "" {
			return false
		}
	}
	return true
}

func normalizeHomepageHeroPhrases(raw interface{}) []interface{} {
	items, _ := raw.([]interface{})
	result := make([]interface{}, 0, settingHomepageHeroPhrasesMaxCount)
	for _, item := range items {
		phrase := normalizeHomepageLocalizedText(item, 100)
		if homepageLocalizedTextIsEmpty(phrase) {
			continue
		}
		result = append(result, phrase)
		if len(result) == settingHomepageHeroPhrasesMaxCount {
			break
		}
	}
	return result
}

func normalizeHomepageReviews(raw interface{}) []interface{} {
	items, _ := raw.([]interface{})
	result := make([]interface{}, 0, settingHomepageReviewsMaxCount)
	seenIDs := make(map[string]bool, settingHomepageReviewsMaxCount)
	for _, item := range items {
		review, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name := settingsvalue.NormalizeTextWithRuneLimit(review["name"], 60)
		product := settingsvalue.NormalizeTextWithRuneLimit(review["product"], 80)
		content := normalizeHomepageLocalizedText(review["content"], 1000)
		if name == "" && product == "" && homepageLocalizedTextIsEmpty(content) {
			continue
		}

		id := settingsvalue.NormalizeTextWithRuneLimit(review["id"], 64)
		if !settingHomepageReviewIDPattern.MatchString(id) || seenIDs[id] {
			for suffix := len(result) + 1; ; suffix++ {
				id = fmt.Sprintf("review-%d", suffix)
				if !seenIDs[id] {
					break
				}
			}
		}
		seenIDs[id] = true
		result = append(result, map[string]interface{}{
			"id":      id,
			"name":    name,
			"product": product,
			"rating":  normalizeHomepageInteger(review["rating"], 1, 5, 5),
			"content": content,
		})
		if len(result) == settingHomepageReviewsMaxCount {
			break
		}
	}
	return result
}

func normalizeHomepageInteger(raw interface{}, minimum, maximum, fallback int) int {
	// 共用解析器兼容表单整数字符串，但这些动画/评分参数不接受小数截断。
	switch number := raw.(type) {
	case float64:
		if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number {
			return fallback
		}
	case json.Number:
		if _, err := number.Int64(); err != nil {
			return fallback
		}
	}
	value, err := settingsvalue.ParseInt(raw)
	if err != nil || value < minimum {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}
