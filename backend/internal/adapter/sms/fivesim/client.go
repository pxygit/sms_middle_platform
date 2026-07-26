package fivesim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/model"
)

const providerCode = "5sim"
const defaultOperator = "any"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     func(model.SupplierRequestLog)
	mu         sync.RWMutex
}

type countryRow struct {
	ISO       json.RawMessage `json:"iso"`
	Prefix    json.RawMessage `json:"prefix"`
	TextEN    string          `json:"text_en"`
	TextRU    string          `json:"text_ru"`
	Name      string          `json:"name"`
	Title     string          `json:"title"`
	ShortName string          `json:"short_name"`
}

type productRow struct {
	Category string          `json:"Category"`
	Qty      json.RawMessage `json:"Qty"`
	Price    json.RawMessage `json:"Price"`
	Name     string          `json:"name"`
	Title    string          `json:"title"`
}

type orderResponse struct {
	ID        json.RawMessage `json:"id"`
	Phone     string          `json:"phone"`
	Operator  string          `json:"operator"`
	Product   string          `json:"product"`
	Country   string          `json:"country"`
	Price     json.RawMessage `json:"price"`
	Status    string          `json:"status"`
	Expires   string          `json:"expires"`
	CreatedAt string          `json:"created_at"`
	SMS       []messageRow    `json:"sms"`
}

type messageRow struct {
	Code      string `json:"code"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
	Date      string `json:"date"`
}

func New(apiKey, baseURL string, timeout time.Duration, logger func(model.SupplierRequestLog)) *Client {
	return &Client{
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}
}

func (c *Client) Name() string { return providerCode }

func (c *Client) Configure(apiKey, baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(apiKey) != "" {
		c.apiKey = strings.TrimSpace(apiKey)
	}
	if strings.TrimSpace(baseURL) != "" {
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
}

func (c *Client) GetBalance(ctx context.Context) (*sms.ProviderBalance, error) {
	var out struct {
		Balance json.RawMessage `json:"balance"`
	}
	raw, status, err := c.requestJSON(ctx, http.MethodGet, "/v1/user/profile", true, nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, providerError(status, raw)
	}
	balance := rawString(out.Balance)
	if balance == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "5sim balance field not found")
	}
	return &sms.ProviderBalance{Balance: balance}, nil
}

func (c *Client) RequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	priceLimit, priceLimitFound := c.priceLimit(ctx, input)
	if priceLimitFound && input.MaxPrice > 0 && priceLimit > input.MaxPrice {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "5sim price exceeds max price")
	}

	path := fmt.Sprintf("/v1/user/buy/activation/%s/%s/%s",
		url.PathEscape(input.CountryID),
		defaultOperator,
		url.PathEscape(input.ServiceID),
	)
	var out orderResponse
	raw, status, err := c.requestJSON(ctx, http.MethodGet, path, true, nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, providerError(status, raw)
	}
	orderID := rawString(out.ID)
	if orderID == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "5sim order id is missing")
	}
	phone := normalizePhone(out.Phone)
	if phone == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "5sim phone number is missing")
	}
	cost := parseFloat(rawString(out.Price))
	if cost <= 0 && priceLimitFound {
		cost = priceLimit
	}
	return &sms.RequestNumberResult{
		SupplierOrderID: orderID,
		PhoneNumber:     phone,
		Country:         firstNonEmpty(out.Country, input.CountryID),
		Service:         firstNonEmpty(out.Product, input.ServiceID),
		Cost:            cost,
		Raw:             raw,
	}, nil
}

func (c *Client) CheckSMS(ctx context.Context, input sms.CheckSMSInput) (*sms.SMSResult, error) {
	var out orderResponse
	raw, status, err := c.requestJSON(ctx, http.MethodGet, "/v1/user/check/"+url.PathEscape(input.SupplierOrderID), true, nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, providerError(status, raw)
	}
	result := &sms.SMSResult{
		Status:         model.OrderActive,
		SupplierStatus: out.Status,
		Raw:            raw,
	}
	if content, code := firstMessage(out.SMS); content != "" || code != "" {
		result.Status = model.OrderSMSReceived
		result.VerificationCode = firstNonEmpty(code, extractCode(content))
		result.SMSContent = firstNonEmpty(content, code)
		c.finish(ctx, input.SupplierOrderID)
		return result, nil
	}
	switch normalizeStatus(out.Status) {
	case "pending", "received":
		result.Status = model.OrderActive
	case "finished":
		result.Status = model.OrderCompleted
	case "canceled", "cancelled", "banned":
		result.Status = model.OrderCancelled
	case "timeout", "expired":
		result.Status = model.OrderExpired
	default:
		result.Status = model.OrderActive
	}
	return result, nil
}

func (c *Client) CancelNumber(ctx context.Context, input sms.CancelNumberInput) (*sms.CancelResult, error) {
	var out orderResponse
	raw, status, err := c.requestJSON(ctx, http.MethodGet, "/v1/user/cancel/"+url.PathEscape(input.SupplierOrderID), true, nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, providerError(status, raw)
	}
	return &sms.CancelResult{Success: true, Message: firstNonEmpty(out.Status, "cancelled"), Raw: raw}, nil
}

func (c *Client) GetCountries(ctx context.Context) ([]sms.ProviderCountry, error) {
	var rows map[string]countryRow
	raw, status, err := c.requestJSON(ctx, http.MethodGet, "/v1/guest/countries", false, nil, &rows)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, providerError(status, raw)
	}
	countries := make([]sms.ProviderCountry, 0, len(rows))
	for code, row := range rows {
		countryCode := strings.TrimSpace(code)
		if countryCode == "" {
			continue
		}
		iso := firstISO(row.ISO)
		countries = append(countries, sms.ProviderCountry{
			Code:      countryCode,
			Name:      firstNonEmpty(row.TextEN, row.Name, row.Title, countryCode),
			ShortName: firstNonEmpty(iso, row.ShortName, countryCode),
			Region:    row.TextRU,
			DialCode:  rawString(row.Prefix),
		})
	}
	sort.Slice(countries, func(i, j int) bool {
		return countries[i].Name < countries[j].Name
	})
	return countries, nil
}

func (c *Client) GetServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	products, err := c.products(ctx, countryID)
	if err != nil {
		return nil, err
	}
	services := make([]sms.ProviderService, 0, len(products))
	for code, product := range products {
		if code == "" || isUnsupportedProduct(product) {
			continue
		}
		services = append(services, sms.ProviderService{
			Code:        code,
			Name:        firstNonEmpty(product.Name, product.Title, code),
			CountryCode: countryID,
			Price:       rawString(product.Price),
			Stock:       rawInt(product.Qty),
		})
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
	return services, nil
}

func (c *Client) GetPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	product, err := c.product(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	price := rawString(product.Price)
	if price == "" {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "5sim price not found")
	}
	return &sms.ProviderPrice{
		LowPrice:  price,
		HighPrice: price,
		Price:     price,
		Raw:       mustMarshal(product),
	}, nil
}

func (c *Client) GetStock(ctx context.Context, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	product, err := c.product(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	return &sms.ProviderStock{Amount: rawInt(product.Qty), Raw: mustMarshal(product)}, nil
}

func (c *Client) products(ctx context.Context, countryID string) (map[string]productRow, error) {
	if strings.TrimSpace(countryID) == "" {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "5sim country is required")
	}
	var rows map[string]productRow
	raw, status, err := c.requestJSON(ctx, http.MethodGet,
		fmt.Sprintf("/v1/guest/products/%s/%s", url.PathEscape(countryID), defaultOperator),
		false,
		nil,
		&rows,
	)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, providerError(status, raw)
	}
	return rows, nil
}

func (c *Client) product(ctx context.Context, countryID, serviceID string) (productRow, error) {
	products, err := c.products(ctx, countryID)
	if err != nil {
		return productRow{}, err
	}
	product, ok := products[serviceID]
	if !ok || isUnsupportedProduct(product) {
		return productRow{}, sms.NewProviderError(sms.ErrPriceNotFound, "5sim product not found")
	}
	return product, nil
}

func (c *Client) priceLimit(ctx context.Context, input sms.RequestNumberInput) (float64, bool) {
	price, err := c.GetPrice(ctx, sms.ProviderPriceInput{CountryID: input.CountryID, ServiceID: input.ServiceID})
	if err != nil {
		return 0, false
	}
	value := parseFloat(firstNonEmpty(price.Price, price.LowPrice, price.HighPrice))
	return value, value > 0
}

func (c *Client) finish(ctx context.Context, orderID string) {
	var out orderResponse
	_, _, _ = c.requestJSON(ctx, http.MethodGet, "/v1/user/finish/"+url.PathEscape(orderID), true, nil, &out)
}

func (c *Client) requestJSON(ctx context.Context, method, path string, auth bool, query url.Values, out interface{}) (json.RawMessage, int, error) {
	apiKey, baseURL := c.config()
	if baseURL == "" {
		baseURL = "https://5sim.net"
	}
	if auth && apiKey == "" {
		return nil, 0, sms.NewProviderError("AUTH_ERROR", "5sim API key is empty")
	}
	target := baseURL + path
	if query != nil && len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log(method, path, query, auth, 0, false, err.Error(), time.Since(start), nil)
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log(method, path, query, auth, resp.StatusCode, false, err.Error(), time.Since(start), body)
		return body, resp.StatusCode, err
	}
	if out != nil && len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			c.log(method, path, query, auth, resp.StatusCode, false, err.Error(), time.Since(start), body)
			return body, resp.StatusCode, err
		}
	}
	c.log(method, path, query, auth, resp.StatusCode, resp.StatusCode < 400, "", time.Since(start), body)
	return body, resp.StatusCode, nil
}

func (c *Client) config() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey, c.baseURL
}

func (c *Client) log(method, path string, query url.Values, auth bool, status int, success bool, message string, latency time.Duration, body []byte) {
	if c.logger == nil {
		return
	}
	request := url.Values{}
	request.Set("method", method)
	request.Set("path", path)
	if auth {
		request.Set("authorization", "Bearer ***")
	}
	for key, values := range query {
		for _, value := range values {
			request.Add(key, value)
		}
	}
	c.logger(model.SupplierRequestLog{
		ProviderCode: providerCode,
		Action:       path,
		HTTPStatus:   status,
		Success:      success,
		ErrorMessage: message,
		LatencyMS:    latency.Milliseconds(),
		RequestBody:  request.Encode(),
		ResponseBody: string(body),
	})
}

func providerError(status int, raw []byte) error {
	message := extractErrorMessage(raw)
	lower := strings.ToLower(message)
	switch {
	case status == http.StatusTooManyRequests || strings.Contains(lower, "rate"):
		return sms.NewProviderError(sms.ErrRateLimited, message)
	case status == http.StatusUnauthorized || status == http.StatusForbidden ||
		strings.Contains(lower, "authorization") ||
		strings.Contains(lower, "api key") ||
		strings.Contains(lower, "token"):
		return sms.NewProviderError("AUTH_ERROR", message)
	case strings.Contains(lower, "balance") || strings.Contains(lower, "not enough money"):
		return sms.NewProviderError(sms.ErrBalance, message)
	case strings.Contains(lower, "no free phones") ||
		strings.Contains(lower, "no product") ||
		strings.Contains(lower, "product not found") ||
		strings.Contains(lower, "not enough rating") ||
		strings.Contains(lower, "not available"):
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	case strings.Contains(lower, "not found") || strings.Contains(lower, "order"):
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	case strings.Contains(lower, "cancel"):
		return sms.NewProviderError(sms.ErrCannotCancel, message)
	default:
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
}

func extractErrorMessage(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "5sim provider rejected request"
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err == nil {
		for _, key := range []string{"message", "msg", "error", "detail"} {
			if value := toString(payload[key]); value != "" {
				return value
			}
		}
	}
	return text
}

func firstMessage(messages []messageRow) (string, string) {
	for _, item := range messages {
		content := firstNonEmpty(item.Text, item.Code)
		code := firstNonEmpty(item.Code, extractCode(content))
		if content != "" || code != "" {
			return content, code
		}
	}
	return "", ""
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func isUnsupportedProduct(product productRow) bool {
	category := strings.ToLower(strings.TrimSpace(product.Category))
	return category != "" && category != "activation"
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" || strings.HasPrefix(phone, "+") {
		return phone
	}
	return "+" + phone
}

func extractCode(text string) string {
	if match := regexp.MustCompile(`\d{3,8}`).FindString(text); match != "" {
		return match
	}
	return strings.TrimSpace(text)
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		return strings.TrimSpace(stringValue)
	}
	var numberValue json.Number
	if err := json.Unmarshal(raw, &numberValue); err == nil {
		return strings.TrimSpace(numberValue.String())
	}
	var boolValue bool
	if err := json.Unmarshal(raw, &boolValue); err == nil {
		if boolValue {
			return "true"
		}
		return "false"
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return strings.TrimSpace(list[0])
	}
	return strings.TrimSpace(string(raw))
}

func firstISO(raw json.RawMessage) string {
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return strings.TrimSpace(list[0])
	}
	return rawString(raw)
}

func rawInt(raw json.RawMessage) int {
	value := rawString(raw)
	if value == "" {
		return 0
	}
	parsed, _ := strconv.Atoi(strings.Split(value, ".")[0])
	return parsed
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mustMarshal(value interface{}) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

var _ sms.SMSProvider = (*Client)(nil)
var _ sms.MetadataProvider = (*Client)(nil)
var _ sms.ConfigurableProvider = (*Client)(nil)
