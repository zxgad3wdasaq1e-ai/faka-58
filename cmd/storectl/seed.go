package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxResponseBytes = 5 * 1024 * 1024

func originURL(value string) (string, error) {
	for _, char := range value {
		if unicode.IsSpace(char) || char < 32 || char == 127 {
			return "", errors.New("API origin 不能包含空白或控制字符")
		}
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("需要不含凭据、路径、查询或片段的 HTTP(S) origin")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", errors.New("API origin 端口无效")
		}
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if !strings.EqualFold(u.Hostname(), "localhost") && (ip == nil || !ip.IsLoopback()) {
			return "", errors.New("远程 API 必须使用 HTTPS；HTTP 仅允许回环地址")
		}
	}
	u.Path, u.RawPath = "", ""
	return u.String(), nil
}

type apiEnvelope struct {
	StatusCode *int            `json:"status_code"`
	Data       json.RawMessage `json:"data"`
	Pagination json.RawMessage `json:"pagination"`
}

type adminAPI struct {
	origin string
	token  string
	client *http.Client
}

func newAdminAPI(origin string) *adminAPI {
	return &adminAPI{origin: origin, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (api *adminAPI) request(method, path string, payload any) (apiEnvelope, error) {
	var envelope apiEnvelope
	endpoint, _, _ := strings.Cut(path, "?")
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return envelope, errors.New("无法编码目录初始化请求")
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, api.origin+"/api/v1"+path, body)
	if err != nil {
		return envelope, errors.New("无法创建目录初始化请求")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if api.token != "" {
		req.Header.Set("Authorization", "Bearer "+api.token)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return envelope, fmt.Errorf("无法连接 %s %s；请检查 API 或 TLS", method, endpoint)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return envelope, fmt.Errorf("%s %s 返回 HTTP %d；请检查认证、权限或服务状态", method, endpoint, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(raw) > maxResponseBytes {
		return envelope, fmt.Errorf("%s %s 响应读取失败或超过大小限制", method, endpoint)
	}
	if !utf8.Valid(raw) || json.Unmarshal(raw, &envelope) != nil || envelope.StatusCode == nil {
		return envelope, fmt.Errorf("%s %s 没有返回有效 API JSON", method, endpoint)
	}
	if *envelope.StatusCode != 0 {
		return envelope, fmt.Errorf("%s %s 被拒绝 (status_code=%d)；请检查认证、RBAC、验证码或 2FA", method, endpoint, *envelope.StatusCode)
	}
	return envelope, nil
}

func jsonObject(raw json.RawMessage) (map[string]any, error) {
	var result map[string]any
	if json.Unmarshal(raw, &result) != nil || result == nil {
		return nil, errors.New("API 未返回所需对象")
	}
	return result, nil
}

func validToken(token string) bool {
	if token == "" {
		return false
	}
	for _, char := range token {
		if char <= 32 || char >= 127 {
			return false
		}
	}
	return true
}

func (api *adminAPI) login(username, password, token string) error {
	if token != "" {
		if !validToken(token) {
			return errors.New("管理员 Token 必须是没有空白的原始 ASCII Token")
		}
		api.token = token
		return nil
	}
	if username == "" || password == "" {
		return errors.New("缺少管理员登录凭据；请保留初始化配置或通过环境变量提供有效凭据")
	}
	response, err := api.request(http.MethodPost, "/admin/login", map[string]string{"username": username, "password": password})
	if err != nil {
		return err
	}
	data, err := jsonObject(response.Data)
	if err != nil {
		return errors.New("管理员登录响应无效")
	}
	if data["requires_totp"] == true {
		return errors.New("此管理员启用了 2FA；请完成正常登录后通过 XIOEN_ADMIN_TOKEN 提供 Token")
	}
	token, ok := data["token"].(string)
	if !ok || !validToken(token) {
		return errors.New("管理员登录没有返回有效 Token")
	}
	api.token = token
	return nil
}

func (api *adminAPI) listAll(path string) ([]map[string]any, error) {
	var result []map[string]any
	for page := 1; page <= 10000; page++ {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		response, err := api.request(http.MethodGet, fmt.Sprintf("%s%spage=%d&page_size=100", path, separator, page), nil)
		if err != nil {
			return nil, err
		}
		var rows []map[string]any
		if len(response.Data) > 0 && string(response.Data) != "null" {
			if json.Unmarshal(response.Data, &rows) != nil {
				return nil, errors.New("目录列表响应格式无效")
			}
		}
		for _, row := range rows {
			if row == nil {
				return nil, errors.New("目录列表包含无效对象")
			}
		}
		result = append(result, rows...)
		if len(response.Pagination) == 0 || string(response.Pagination) == "null" || string(response.Pagination) == "{}" {
			return result, nil
		}
		var pagination struct {
			TotalPages int `json:"total_page"`
		}
		if json.Unmarshal(response.Pagination, &pagination) != nil || pagination.TotalPages < 0 {
			return nil, errors.New("目录分页响应无效")
		}
		if page >= pagination.TotalPages {
			return result, nil
		}
		if len(rows) == 0 {
			return nil, errors.New("目录分页未安全完成")
		}
	}
	return nil, errors.New("目录分页超过安全上限")
}

func localized(cn, tw, en string) map[string]string {
	return map[string]string{"zh-CN": cn, "zh-TW": tw, "en-US": en}
}

func siteSettings(origin string) map[string]any {
	return map[string]any{
		"brand":    map[string]any{"site_name": "XIOEN AI", "site_url": origin, "site_description": localized("AI 订阅与数字商品服务", "AI 訂閱與數位商品服務", "AI subscriptions and digital services")},
		"currency": "CNY", "storefront_template": "getgpt", "template_mode": "card", "languages": []string{"zh-CN", "zh-TW", "en-US"},
		"seo": map[string]any{"title": localized("XIOEN AI · AI 订阅与数字商品", "XIOEN AI · AI 訂閱與數位商品", "XIOEN AI · AI subscriptions and digital services"), "description": localized("选购 AI 服务与数字商品，查询订单和交付进度。", "選購 AI 服務與數位商品，查詢訂單和交付進度。", "Explore AI services and digital goods. Track your order and delivery.")},
	}
}

func catalog(categoryID float64) []map[string]any {
	plans := []struct {
		slug, sku, name string
		price           int
		description     map[string]string
	}{
		{"chatgpt-plus", "PLUS-MONTH", "ChatGPT Plus", 179, localized("适合个人办公、学习与日常使用。", "適合個人辦公、學習與日常使用。", "For personal work, study, and everyday use.")},
		{"chatgpt-pro-100", "PRO100-MONTH", "ChatGPT Pro 100", 879, localized("面向经常编程、写作与研究的用户。", "適合經常編程、寫作與研究的使用者。", "For regular programming, writing, and research.")},
		{"chatgpt-pro-200", "PRO200-MONTH", "ChatGPT Pro 200", 1589, localized("面向高频使用与长期项目的进阶方案。", "適合高頻使用與長期專案的進階方案。", "An advanced plan for frequent use and ongoing projects.")},
	}
	details := localized("<p>此商品为月度服务方案。具体服务内容、可用地区与有效期以购买时的商品说明及实际交付为准。</p><p>完成付款后，请保留订单号，在订单详情查看交付内容和使用说明。请勿提交第三方账号密码、Session 或登录令牌。</p>", "<p>此商品為月度服務方案。實際服務內容、可用地區與有效期以購買時的商品說明及實際交付為準。</p><p>完成付款後，請保留訂單編號，在訂單詳情查看交付內容和使用說明。請勿提交第三方帳號密碼、Session 或登入權杖。</p>", "<p>This is a monthly service plan. Features, availability, and validity follow the product description and delivered service.</p><p>Keep your order number after payment and check the order page for delivery and instructions. Do not submit third-party passwords, sessions, or login tokens.</p>")
	var result []map[string]any
	for index, plan := range plans {
		result = append(result, map[string]any{
			"category_id": categoryID, "slug": plan.slug, "title": localized(plan.name+" 充值", plan.name+" 儲值", plan.name+" subscription"), "description": plan.description, "content": details,
			"price_amount": plan.price, "cost_price_amount": 0, "purchase_type": "guest", "fulfillment_type": "auto", "min_purchase_quantity": 1, "max_purchase_quantity": 10, "manual_stock_total": 0, "stock_display_mode": "exact", "is_active": true, "is_affiliate_enabled": false, "images": []string{}, "tags": []string{}, "sort_order": 30 - index*10,
			"skus": []map[string]any{{"sku_code": plan.sku, "spec_values": localized("1 个月", "1 個月", "1 month"), "price_amount": plan.price, "cost_price_amount": 0, "manual_stock_total": 0, "is_active": true, "sort_order": 0}},
		})
	}
	return result
}

func findSlug(rows []map[string]any, slug string) map[string]any {
	for _, row := range rows {
		if row["slug"] == slug {
			return row
		}
	}
	return nil
}

func seedCatalog(api *adminAPI, publicOrigin string, output io.Writer) error {
	response, err := api.request(http.MethodGet, "/admin/settings?key=site_config", nil)
	if err != nil {
		return err
	}
	settings, err := jsonObject(response.Data)
	if err != nil {
		return errors.New("站点配置 API 未返回有效对象")
	}
	if len(settings) == 0 {
		if _, err := api.request(http.MethodPut, "/admin/settings", map[string]any{"key": "site_config", "value": siteSettings(publicOrigin)}); err != nil {
			return err
		}
		fmt.Fprintln(output, "已初始化 XIOEN AI 站点配置。")
	} else {
		fmt.Fprintln(output, "沿用现有站点设置。")
	}
	rows, err := api.listAll("/admin/categories")
	if err != nil {
		return err
	}
	category := findSlug(rows, "chatgpt")
	if category == nil {
		response, err = api.request(http.MethodPost, "/admin/categories", map[string]any{"slug": "chatgpt", "parent_id": 0, "sort_order": 100, "name": localized("ChatGPT 服务", "ChatGPT 服務", "ChatGPT services")})
		if err != nil {
			rows, listErr := api.listAll("/admin/categories")
			if listErr != nil {
				return err
			}
			category = findSlug(rows, "chatgpt")
			if category == nil {
				return err
			}
		} else {
			category, err = jsonObject(response.Data)
			if err != nil {
				return errors.New("分类创建 API 未返回有效对象")
			}
		}
	}
	categoryID, ok := category["id"].(float64)
	if !ok || categoryID <= 0 || categoryID != math.Trunc(categoryID) || categoryID > 1<<53 {
		return errors.New("分类 API 未返回有效 ID")
	}
	rows, err = api.listAll("/admin/products")
	if err != nil {
		return err
	}
	for _, product := range catalog(categoryID) {
		slug := product["slug"].(string)
		if findSlug(rows, slug) != nil {
			fmt.Fprintln(output, "沿用现有商品:", slug)
			continue
		}
		if _, err := api.request(http.MethodPost, "/admin/products", product); err != nil {
			latest, listErr := api.listAll("/admin/products")
			if listErr != nil || findSlug(latest, slug) == nil {
				return err
			}
			fmt.Fprintln(output, "沿用现有商品:", slug)
		} else {
			fmt.Fprintln(output, "已创建商品（零库存）:", slug)
		}
		rows = append(rows, product)
	}
	fmt.Fprintln(output, "目录已就绪；请在后台导入真实库存并配置支付渠道后再销售。")
	return nil
}

func seedFromConfig(root, baseURL string, output io.Writer) error {
	origin, err := originURL(baseURL)
	if err != nil {
		return err
	}
	var config struct {
		Bootstrap struct {
			Username string `json:"default_admin_username"`
			Password string `json:"default_admin_password"`
		} `json:"bootstrap"`
		CORS struct {
			Origins []string `json:"allowed_origins"`
		} `json:"cors"`
	}
	if err := readJSON(filepath.Join(root, "config.yml"), &config); err != nil {
		return err
	}
	publicOrigin := origin
	for _, configured := range config.CORS.Origins {
		if validated, err := originURL(configured); err == nil {
			publicOrigin = validated
			break
		}
	}
	if state, err := readState(root); err == nil {
		if validated, err := originURL(state.Origin); err == nil {
			publicOrigin = validated
		}
	}
	username, password, token := os.Getenv("XIOEN_ADMIN_USERNAME"), os.Getenv("XIOEN_ADMIN_PASSWORD"), os.Getenv("XIOEN_ADMIN_TOKEN")
	if username == "" {
		username = config.Bootstrap.Username
	}
	if password == "" {
		password = config.Bootstrap.Password
	}
	api := newAdminAPI(origin)
	if err := api.login(username, password, token); err != nil {
		return err
	}
	return seedCatalog(api, publicOrigin, output)
}
