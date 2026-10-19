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
	settingHomepageHeroPhrasesMaxCount = 8
	settingHomepageReviewsMaxCount     = 24
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
	}
	for _, field := range []string{"reviews_enabled", "reviews_demo"} {
		if setting, ok := value[field]; ok && setting != nil {
			result[field] = settingsvalue.ParseBool(setting)
		}
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
