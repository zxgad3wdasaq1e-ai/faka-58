package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func seedConfigFixture(t *testing.T) string {
	t.Helper()
	for _, key := range []string{"FAKA_58_ADMIN_USERNAME", "FAKA_58_ADMIN_PASSWORD", "FAKA_58_ADMIN_TOKEN", "XIOEN_ADMIN_USERNAME", "XIOEN_ADMIN_PASSWORD", "XIOEN_ADMIN_TOKEN"} {
		t.Setenv(key, "")
	}
	root := t.TempDir()
	config := map[string]any{"bootstrap": map[string]string{"default_admin_username": "fixture-admin", "default_admin_password": "private-fixture-password"}, "cors": map[string]any{"allowed_origins": []string{"https://public.example"}}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(root, "config.yml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAdminEnvironmentPrefersCurrentKeysAndSupportsExistingDeployments(t *testing.T) {
	for _, suffix := range []string{"USERNAME", "PASSWORD", "TOKEN"} {
		current := "FAKA_58_ADMIN_" + suffix
		t.Setenv(current, "")
		if err := os.Unsetenv(current); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XIOEN_ADMIN_"+suffix, "legacy-value")
		if got := adminEnvironment(suffix); got != "legacy-value" {
			t.Fatalf("existing %s environment was ignored", suffix)
		}
		t.Setenv(current, "current-value")
		if got := adminEnvironment(suffix); got != "current-value" {
			t.Fatalf("new %s environment did not take priority", suffix)
		}
		t.Setenv(current, "")
		if got := adminEnvironment(suffix); got != "" {
			t.Fatalf("explicit empty %s unexpectedly fell back", suffix)
		}
	}
}

func respondAPI(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": data})
}

func TestSeedRealAPIContractIsIdempotentAndPreservesCustomization(t *testing.T) {
	root := seedConfigFixture(t)
	settings := map[string]any{}
	categories := []map[string]any{}
	products := []map[string]any{}
	writes := map[string]int{}
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/admin/login" {
			loginCalls++
			var login map[string]string
			if json.NewDecoder(r.Body).Decode(&login) != nil || login["username"] != "fixture-admin" || login["password"] != "private-fixture-password" {
				t.Error("seed did not authenticate using private bootstrap config")
			}
			respondAPI(w, map[string]string{"token": "private-fixture-token"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer private-fixture-token" {
			t.Error("seed request omitted authorization")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/admin/settings":
			if r.URL.Query().Get("key") != "site_config" {
				t.Error("unexpected settings key")
			}
			respondAPI(w, settings)
		case "PUT /api/v1/admin/settings":
			var payload struct {
				Key   string         `json:"key"`
				Value map[string]any `json:"value"`
			}
			if json.NewDecoder(r.Body).Decode(&payload) != nil || payload.Key != "site_config" {
				t.Error("invalid settings payload")
			}
			settings = payload.Value
			writes["settings"]++
			respondAPI(w, settings)
		case "GET /api/v1/admin/categories":
			respondAPI(w, categories)
		case "POST /api/v1/admin/categories":
			var category map[string]any
			_ = json.NewDecoder(r.Body).Decode(&category)
			if category["slug"] != "chatgpt" {
				t.Error("unexpected category contract")
			}
			category["id"] = float64(42)
			categories = append(categories, category)
			writes["categories"]++
			respondAPI(w, category)
		case "GET /api/v1/admin/products":
			respondAPI(w, products)
		case "POST /api/v1/admin/products":
			var product map[string]any
			_ = json.NewDecoder(r.Body).Decode(&product)
			if product["fulfillment_type"] != "auto" || product["purchase_type"] != "guest" || product["manual_stock_total"] != float64(0) || product["category_id"] != float64(42) {
				t.Error("unexpected real product API contract")
			}
			skus, ok := product["skus"].([]any)
			if !ok || len(skus) != 1 || skus[0].(map[string]any)["manual_stock_total"] != float64(0) {
				t.Error("catalog must create one zero-stock SKU")
			}
			products = append(products, product)
			writes["products"]++
			respondAPI(w, product)
		default:
			t.Errorf("unexpected seed mutation: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := seedFromConfig(root, server.URL, &output); err != nil {
		t.Fatal(err)
	}
	if settings["brand"].(map[string]any)["site_url"] != "https://public.example" || settings["storefront_template"] != "faka-58" {
		t.Fatal("public brand origin must not become the internal API loopback address")
	}
	if len(products) != 3 || products[0]["price_amount"] != float64(179) || products[1]["price_amount"] != float64(879) || products[2]["price_amount"] != float64(1589) {
		t.Fatal("seed catalog does not match the original product contract")
	}
	settings["brand"] = map[string]any{"site_name": "Customized store"}
	products[0]["price_amount"] = float64(888)
	products[0]["manual_stock_total"] = float64(15)
	if err := seedFromConfig(root, server.URL, &output); err != nil {
		t.Fatal(err)
	}
	if loginCalls != 2 || !reflect.DeepEqual(writes, map[string]int{"settings": 1, "categories": 1, "products": 3}) {
		t.Fatal("repeated seed performed an unwanted mutation")
	}
	if settings["brand"].(map[string]any)["site_name"] != "Customized store" || products[0]["price_amount"] != float64(888) || products[0]["manual_stock_total"] != float64(15) {
		t.Fatal("seed overwrote existing site, product or inventory customization")
	}
	for _, secret := range []string{"private-fixture-password", "private-fixture-token"} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("seed emitted private credentials")
		}
	}
}

func TestSeedErrorsNeverExposeResponseBodies(t *testing.T) {
	for _, body := range []string{`{"status_code":500,"data":{"token":"private-server-token"},"message":"private-server-token"}`, `{"status_code":"private-server-token"}`, `private-server-token`, `{"status_code":0,"data":{"token":"private-server-token","requires_totp":true}}`, `{"status_code":0,"data":null}`} {
		t.Run(strconvLabel(body), func(t *testing.T) {
			root := seedConfigFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
			defer server.Close()
			var output bytes.Buffer
			err := seedFromConfig(root, server.URL, &output)
			if err == nil || strings.Contains(err.Error(), "private-server-token") || strings.Contains(output.String(), "private-server-token") {
				t.Fatal("private or invalid server response was accepted or disclosed")
			}
		})
	}
}

func strconvLabel(value string) string {
	if len(value) > 40 {
		return value[:40]
	}
	return value
}

func TestSeedNeverFollowsAuthenticationRedirects(t *testing.T) {
	root := seedConfigFixture(t)
	forwarded := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded++
		respondAPI(w, map[string]string{"token": "private-token"})
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	err := seedFromConfig(root, origin.URL, io.Discard)
	if err == nil || forwarded != 0 {
		t.Fatal("authentication redirect forwarded private credentials")
	}
}

func TestSeedRejectsOversizedAndMalformedListResponses(t *testing.T) {
	responses := []string{strings.Repeat("x", maxResponseBytes+1), `{"status_code":0,"data":{}}`, `{"status_code":0,"data":[null]}`, `{"status_code":0,"data":[],"pagination":{"total_page":2}}`, `{"status_code":0,"data":[],"pagination":{"total_page":"private-token"}}`}
	for index, body := range responses {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
		api := newAdminAPI(server.URL)
		_, err := api.listAll("/admin/products")
		server.Close()
		if err == nil || strings.Contains(err.Error(), "private-token") {
			t.Errorf("malformed response %d accepted or exposed", index)
		}
	}
}

func TestSeedListsEveryPage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		page := r.URL.Query().Get("page")
		_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": []map[string]string{{"slug": "page-" + page}}, "pagination": map[string]int{"total_page": 2}})
	}))
	defer server.Close()
	rows, err := newAdminAPI(server.URL).listAll("/admin/products")
	if err != nil || requests != 2 || len(rows) != 2 || rows[1]["slug"] != "page-2" {
		t.Fatal("paginated products were not fully read")
	}
}

func TestSeedOriginValidationAndTokenValidation(t *testing.T) {
	for _, origin := range []string{"http://example.com", "https://user:private@example.com", "https://example.com/path", "https://example.com?token=private", "https://example.com#private", "http://127.0.0.1:99999", "https://example.com\nSECRET=value"} {
		if _, err := originURL(origin); err == nil {
			t.Error("unsafe API origin accepted")
		}
	}
	for _, origin := range []string{"https://example.com", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		if _, err := originURL(origin); err != nil {
			t.Errorf("valid origin rejected: %v", err)
		}
	}
	api := newAdminAPI("http://127.0.0.1:1")
	if err := api.login("", "", "private-token\nHeader=value"); err == nil {
		t.Fatal("token containing control characters accepted")
	}
	if err := api.login("", "", "private-token"); err != nil || api.token != "private-token" {
		t.Fatal("existing administrator token could not be used")
	}
}
