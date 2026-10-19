package settingsstorefront

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dujiao-next/internal/constants"
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
	if len(homepage) != 8 {
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
