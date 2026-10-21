package settingsstorefront

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/zxgad3wdasaq1e-ai/faka-58/internal/constants"
)

func TestHomepageNormalizesBoundsAndDropsUnknownFields(t *testing.T) {
	phrases := []interface{}{"invalid", map[string]interface{}{"unsupported": "drop"}}
	reviews := []interface{}{nil, "invalid", map[string]interface{}{}}
	for i := 0; i < 30; i++ {
		phrases = append(phrases, map[string]interface{}{"zh-CN": strings.Repeat("好", 110), "unknown": "drop"})
		reviews = append(reviews, map[string]interface{}{
			"id": "duplicated", "name": strings.Repeat("人", 70), "product": strings.Repeat("品", 90),
			"rating": 99, "content": map[string]interface{}{"zh-CN": strings.Repeat("评", 1100), "unknown": "drop"},
			"avatar": "https://example.com/avatar.png", "url": "https://example.com", "html": "<b>drop</b>",
		})
	}
	homepage := NormalizeHomepage(map[string]interface{}{
		"hero_phrases": phrases, "hero_interval_ms": 10001,
		"reviews_duration_seconds": 10,
		"reviews_title":            map[string]interface{}{"zh-CN": strings.Repeat("题", 210), "unknown": "drop"},
		"reviews_subtitle":         map[string]interface{}{"zh-TW": strings.Repeat("副", 510)},
		"reviews":                  reviews, "html": "<script>drop()</script>", "image": "/uploads/test.png",
	})
	if len(homepage) != 12 {
		t.Fatalf("unexpected homepage fields: %#v", homepage)
	}
	if homepage["hero_interval_ms"] != 10000 || homepage["reviews_duration_seconds"] != 55 {
		t.Fatalf("animation bounds not enforced: %#v", homepage)
	}
	boundedPhrases := homepage["hero_phrases"].([]interface{})
	if len(boundedPhrases) != 8 {
		t.Fatalf("phrase count = %d, want 8", len(boundedPhrases))
	}
	assertHomepageLocalizedLength(t, boundedPhrases[0], "zh-CN", 100)
	assertHomepageLocalizedLength(t, homepage["reviews_title"], "zh-CN", 200)
	assertHomepageLocalizedLength(t, homepage["reviews_subtitle"], "zh-TW", 500)
	boundedReviews := homepage["reviews"].([]interface{})
	if len(boundedReviews) != 24 {
		t.Fatalf("review count = %d, want 24", len(boundedReviews))
	}
	seenIDs := map[string]bool{}
	for _, item := range boundedReviews {
		review := item.(map[string]interface{})
		if len(review) != 5 || review["rating"] != 5 {
			t.Fatalf("unexpected review fields: %#v", review)
		}
		id := review["id"].(string)
		if !settingHomepageReviewIDPattern.MatchString(id) || seenIDs[id] {
			t.Fatalf("review ID is invalid or duplicated: %q", id)
		}
		seenIDs[id] = true
		if utf8.RuneCountInString(review["name"].(string)) != 60 || utf8.RuneCountInString(review["product"].(string)) != 80 {
			t.Fatalf("name/product rune bounds not enforced: %#v", review)
		}
		assertHomepageLocalizedLength(t, review["content"], "zh-CN", 1000)
	}
}

func TestHomepageActivityDefaults(t *testing.T) {
	wantNames := []string{"晚风送信", "小林同学", "wx_kai109", "青山不老", "Mia_2026", "小周周", "13752861042", "星河旅人", "chenyu_88", "阿橙"}
	wantMessages := []interface{}{
		map[string]interface{}{"kind": "order", "text": map[string]interface{}{"zh-CN": "{name} 下单了 ChatGPT Plus 订阅", "zh-TW": "{name} 下單了 ChatGPT Plus 訂閱", "en-US": "{name} ordered a ChatGPT Plus subscription"}},
		map[string]interface{}{"kind": "presence", "text": map[string]interface{}{"zh-CN": "目前有 {count} 位用户正在充值", "zh-TW": "目前有 {count} 位使用者正在儲值", "en-US": "{count} users are currently recharging"}},
		map[string]interface{}{"kind": "notice", "text": map[string]interface{}{"zh-CN": "正规渠道充值，不成功100%退款", "zh-TW": "正規管道儲值，不成功100%退款", "en-US": "Official-channel recharges. Full refund if unsuccessful."}},
		map[string]interface{}{"kind": "notice", "text": map[string]interface{}{"zh-CN": "支持 ChatGPT、Claude、Cursor、Gemini 等主流工具", "zh-TW": "支援 ChatGPT、Claude、Cursor、Gemini 等主流工具", "en-US": "Supports ChatGPT, Claude, Cursor, Gemini and other popular tools"}},
		map[string]interface{}{"kind": "notice", "text": map[string]interface{}{"zh-CN": "7×24 小时全天候自动充值服务！", "zh-TW": "7×24 小時全天候自動儲值服務！", "en-US": "24/7 automatic recharge service!"}},
	}
	for _, input := range []interface{}{
		nil,
		map[string]interface{}{},
		map[string]interface{}{"activity_enabled": nil, "activity_names": nil, "activity_messages": nil},
		map[string]interface{}{"activity_names": []interface{}{nil, " ", 12}, "activity_messages": []interface{}{nil, "invalid", map[string]interface{}{"text": map[string]interface{}{"unknown": "drop"}}}},
	} {
		got := NormalizeHomepage(input)
		if got["activity_enabled"] != true || got["activity_interval_ms"] != 4200 {
			t.Fatalf("activity defaults mismatch: %#v", got)
		}
		if !reflect.DeepEqual(got["activity_names"], wantNames) || !reflect.DeepEqual(got["activity_messages"], wantMessages) {
			t.Fatalf("activity fallback names/messages mismatch: %#v", got)
		}
	}
	// 每次归一化独立分配默认值，调用方修改不能影响后续请求。
	first := NormalizeHomepage(nil)
	first["activity_names"].([]string)[0] = "modified"
	first["activity_messages"].([]interface{})[0].(map[string]interface{})["text"].(map[string]interface{})["zh-CN"] = "modified"
	second := NormalizeHomepage(nil)
	if !reflect.DeepEqual(second["activity_names"], wantNames) || !reflect.DeepEqual(second["activity_messages"], wantMessages) {
		t.Fatal("activity defaults were shared across calls")
	}
}

func TestHomepageActivityNormalizesLimitsAndKinds(t *testing.T) {
	names := []interface{}{nil, 12, " "}
	messages := []interface{}{nil, "invalid", map[string]interface{}{"text": map[string]interface{}{"en-US": " "}}}
	for i := 0; i < 45; i++ {
		names = append(names, " "+strings.Repeat("名", 35)+" ")
		messages = append(messages, map[string]interface{}{
			"kind": "invalid", "text": map[string]interface{}{"zh-CN": strings.Repeat("播", 210), "unknown": "drop"},
			"html": "drop", "image": "drop",
		})
	}
	got := NormalizeHomepage(map[string]interface{}{
		"activity_enabled": false, "activity_names": names, "activity_messages": messages, "activity_interval_ms": 15001,
	})
	if got["activity_enabled"] != false || got["activity_interval_ms"] != 15000 {
		t.Fatalf("activity switch/interval normalization mismatch: %#v", got)
	}
	boundedNames := got["activity_names"].([]string)
	if len(boundedNames) != 40 || utf8.RuneCountInString(boundedNames[0]) != 30 {
		t.Fatalf("activity names limits mismatch: %#v", boundedNames)
	}
	boundedMessages := got["activity_messages"].([]interface{})
	if len(boundedMessages) != 12 {
		t.Fatalf("activity messages count = %d, want 12", len(boundedMessages))
	}
	for _, raw := range boundedMessages {
		message := raw.(map[string]interface{})
		if len(message) != 2 || message["kind"] != "notice" {
			t.Fatalf("activity message unknown fields/kind not normalized: %#v", message)
		}
		assertHomepageLocalizedLength(t, message["text"], "zh-CN", 200)
	}
	for _, kind := range []string{"order", "presence", "notice"} {
		messages := NormalizeHomepage(map[string]interface{}{"activity_messages": []interface{}{
			map[string]interface{}{"kind": " " + kind + " ", "text": map[string]interface{}{"en-US": " {name} {count} "}},
		}})["activity_messages"].([]interface{})
		message := messages[0].(map[string]interface{})
		if message["kind"] != kind || message["text"].(map[string]interface{})["en-US"] != "{name} {count}" {
			t.Fatalf("valid activity kind/placeholders changed: %#v", message)
		}
	}
	stringNames := NormalizeHomepage(map[string]interface{}{"activity_names": []string{" ", " Alice "}})["activity_names"]
	if !reflect.DeepEqual(stringNames, []string{"Alice"}) {
		t.Fatalf("typed activity names mismatch: %#v", stringNames)
	}
}

func TestHomepageActivityIntervalBoundaries(t *testing.T) {
	for _, test := range []struct {
		input interface{}
		want  int
	}{
		{nil, 4200}, {"bad", 4200}, {1999, 4200}, {2000, 2000}, {15000, 15000}, {15001, 15000},
		{2500.5, 4200}, {json.Number("2500.5"), 4200}, {" 5000 ", 5000},
	} {
		got := NormalizeHomepage(map[string]interface{}{"activity_interval_ms": test.input})
		if got["activity_interval_ms"] != test.want {
			t.Errorf("activity interval %#v = %v, want %d", test.input, got["activity_interval_ms"], test.want)
		}
	}
}

func TestHomepageIntegerDefaultsAndLimits(t *testing.T) {
	for _, test := range []struct {
		input interface{}
		want  int
	}{
		{nil, 2600}, {"bad", 2600}, {999, 2600}, {1000, 1000}, {10000, 10000}, {10001, 10000},
		{2500.5, 2600}, {math.NaN(), 2600}, {math.Inf(1), 2600}, {json.Number("3500"), 3500},
		{json.Number("3500.5"), 2600}, {" 4000 ", 4000}, {true, 2600},
	} {
		if got := normalizeHomepageInteger(test.input, 1000, 10000, 2600); got != test.want {
			t.Errorf("input %#v: got %d, want %d", test.input, got, test.want)
		}
	}
	for _, input := range []interface{}{map[string]interface{}{}, map[string]interface{}{"reviews_enabled": nil, "reviews_demo": nil}} {
		got := NormalizeHomepage(input)
		if got["reviews_enabled"] != true || got["reviews_demo"] != true {
			t.Fatalf("missing/null boolean defaults = %#v", got)
		}
	}
}

func assertHomepageLocalizedLength(t *testing.T, raw interface{}, locale string, want int) {
	t.Helper()
	value := raw.(map[string]interface{})
	if len(value) != len(constants.SupportedLocales) || utf8.RuneCountInString(value[locale].(string)) != want {
		t.Fatalf("localized text bounds/locales mismatch: %#v", value)
	}
}
