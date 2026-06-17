package herosms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/model"
)

const providerCode = "herosms"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     func(model.SupplierRequestLog)
	mu         sync.RWMutex
}

type priceItem struct {
	CountryID string
	ServiceID string
	Cost      string
	Count     int
}

func New(apiKey, baseURL string, timeout time.Duration, logger func(model.SupplierRequestLog)) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}
}

func (c *Client) Name() string { return providerCode }

func (c *Client) Configure(apiKey, baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if apiKey != "" {
		c.apiKey = apiKey
	}
	if baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

func (c *Client) GetBalance(ctx context.Context) (*sms.ProviderBalance, error) {
	raw, status, err := c.call(ctx, "getBalance", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("herosms http %d: %s", status, raw)
	}
	if strings.HasPrefix(raw, "ACCESS_BALANCE:") {
		return &sms.ProviderBalance{Balance: strings.TrimPrefix(raw, "ACCESS_BALANCE:")}, nil
	}
	return nil, providerError(raw)
}

func (c *Client) RequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	query := url.Values{
		"service": {input.ServiceID},
	}
	if input.CountryID != "" {
		query.Set("country", input.CountryID)
	}
	if input.PoolID != "" {
		query.Set("operator", input.PoolID)
	}
	if input.MaxPrice > 0 {
		query.Set("maxPrice", strconv.FormatFloat(input.MaxPrice, 'f', 4, 64))
	}
	raw, status, err := c.call(ctx, "getNumber", query)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("herosms http %d: %s", status, raw)
	}
	if strings.HasPrefix(raw, "ACCESS_NUMBER:") {
		parts := strings.Split(raw, ":")
		if len(parts) >= 3 {
			phone := normalizePhone(parts[2])
			price, _ := c.GetPrice(ctx, sms.ProviderPriceInput{CountryID: input.CountryID, ServiceID: input.ServiceID})
			cost, _ := strconv.ParseFloat(firstNonEmpty(priceString(price), "0"), 64)
			return &sms.RequestNumberResult{
				SupplierOrderID: parts[1],
				PhoneNumber:     phone,
				Country:         input.CountryID,
				Service:         input.ServiceID,
				Cost:            cost,
				Raw:             json.RawMessage(strconv.Quote(raw)),
			}, nil
		}
	}
	return nil, providerError(raw)
}

func (c *Client) CheckSMS(ctx context.Context, input sms.CheckSMSInput) (*sms.SMSResult, error) {
	raw, status, err := c.call(ctx, "getStatus", url.Values{"id": {input.SupplierOrderID}})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("herosms http %d: %s", status, raw)
	}
	result := &sms.SMSResult{
		SupplierStatus: raw,
		Raw:            json.RawMessage(strconv.Quote(raw)),
	}
	switch {
	case strings.HasPrefix(raw, "STATUS_OK:"):
		code := strings.TrimPrefix(raw, "STATUS_OK:")
		result.Status = model.OrderSMSReceived
		result.VerificationCode = extractCode(code)
		result.SMSContent = code
	case strings.HasPrefix(raw, "STATUS_WAIT_RETRY:"):
		result.Status = model.OrderActive
		result.SMSContent = strings.TrimPrefix(raw, "STATUS_WAIT_RETRY:")
	case raw == "STATUS_WAIT_CODE" || raw == "STATUS_WAIT_RESEND":
		result.Status = model.OrderActive
	case raw == "STATUS_CANCEL":
		result.Status = model.OrderCancelled
	default:
		return nil, providerError(raw)
	}
	return result, nil
}

func (c *Client) CancelNumber(ctx context.Context, input sms.CancelNumberInput) (*sms.CancelResult, error) {
	raw, status, err := c.call(ctx, "setStatus", url.Values{
		"id":     {input.SupplierOrderID},
		"status": {"8"},
	})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("herosms http %d: %s", status, raw)
	}
	if raw == "ACCESS_CANCEL" || raw == "ACCESS_READY" {
		return &sms.CancelResult{Success: true, Message: raw, Raw: json.RawMessage(strconv.Quote(raw))}, nil
	}
	return nil, providerError(raw)
}

func (c *Client) GetCountries(ctx context.Context) ([]sms.ProviderCountry, error) {
	raw, status, err := c.call(ctx, "getCountries", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("herosms http %d: %s", status, raw)
	}
	if isPlainError(raw) {
		return nil, providerError(raw)
	}
	return parseCountries([]byte(raw))
}

func (c *Client) GetServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	services, err := c.services(ctx)
	if err != nil {
		return nil, err
	}
	if countryID == "" {
		return services, nil
	}
	available := map[string]bool{}
	prices, err := c.prices(ctx, countryID, "")
	if err != nil {
		return nil, err
	}
	for _, item := range prices {
		if item.CountryID == countryID && item.ServiceID != "" {
			available[item.ServiceID] = true
		}
	}
	filtered := make([]sms.ProviderService, 0, len(services))
	for _, service := range services {
		service.CountryCode = countryID
		if available[service.Code] {
			filtered = append(filtered, service)
		}
	}
	return filtered, nil
}

func (c *Client) GetPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	prices, err := c.prices(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	for _, item := range prices {
		if input.CountryID != "" && item.CountryID != "" && item.CountryID != input.CountryID {
			continue
		}
		if input.ServiceID != "" && item.ServiceID != "" && item.ServiceID != input.ServiceID {
			continue
		}
		return &sms.ProviderPrice{
			LowPrice:  item.Cost,
			HighPrice: item.Cost,
			Price:     item.Cost,
			Raw:       mustMarshal(item),
		}, nil
	}
	return nil, sms.NewProviderError(sms.ErrPriceNotFound, "herosms price not found")
}

func (c *Client) GetStock(ctx context.Context, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	prices, err := c.prices(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	total := 0
	for _, item := range prices {
		if input.CountryID != "" && item.CountryID != "" && item.CountryID != input.CountryID {
			continue
		}
		if input.ServiceID != "" && item.ServiceID != "" && item.ServiceID != input.ServiceID {
			continue
		}
		total += item.Count
	}
	return &sms.ProviderStock{Amount: total, Raw: mustMarshal(prices)}, nil
}

func (c *Client) services(ctx context.Context) ([]sms.ProviderService, error) {
	raw, status, err := c.call(ctx, "getServicesList", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("herosms http %d: %s", status, raw)
	}
	if isPlainError(raw) {
		return nil, providerError(raw)
	}
	return parseServices([]byte(raw))
}

func (c *Client) prices(ctx context.Context, countryID, serviceID string) ([]priceItem, error) {
	query := url.Values{}
	if countryID != "" {
		query.Set("country", countryID)
	}
	if serviceID != "" {
		query.Set("service", serviceID)
	}
	raw, status, err := c.call(ctx, "getPrices", query)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("herosms http %d: %s", status, raw)
	}
	if isPlainError(raw) {
		return nil, providerError(raw)
	}
	return parsePrices([]byte(raw), countryID, serviceID)
}

func (c *Client) call(ctx context.Context, action string, query url.Values) (string, int, error) {
	apiKey, baseURL := c.config()
	if query == nil {
		query = url.Values{}
	}
	query.Set("action", action)
	query.Set("api_key", apiKey)
	target := baseURL + "/stubs/handler_api.php?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log(action, query, 0, false, err.Error(), time.Since(start), "")
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	text := strings.TrimSpace(string(body))
	if err != nil {
		c.log(action, query, resp.StatusCode, false, err.Error(), time.Since(start), text)
		return text, resp.StatusCode, err
	}
	c.log(action, query, resp.StatusCode, resp.StatusCode < 400 && !isPlainError(text), "", time.Since(start), text)
	return text, resp.StatusCode, nil
}

func (c *Client) config() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey, c.baseURL
}

func (c *Client) log(action string, query url.Values, status int, success bool, message string, latency time.Duration, body string) {
	if c.logger == nil {
		return
	}
	safe := url.Values{}
	for key, values := range query {
		if key == "api_key" {
			safe.Set(key, "***")
			continue
		}
		for _, value := range values {
			safe.Add(key, value)
		}
	}
	c.logger(model.SupplierRequestLog{
		ProviderCode: providerCode,
		Action:       action,
		HTTPStatus:   status,
		Success:      success,
		ErrorMessage: message,
		LatencyMS:    latency.Milliseconds(),
		RequestBody:  safe.Encode(),
		ResponseBody: body,
	})
}

func parseCountries(raw []byte) ([]sms.ProviderCountry, error) {
	var rows map[string]struct {
		ID      json.RawMessage `json:"id"`
		Rus     string          `json:"rus"`
		Eng     string          `json:"eng"`
		Chn     string          `json:"chn"`
		Name    string          `json:"name"`
		Visible json.RawMessage `json:"visible"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		var list []map[string]interface{}
		if listErr := json.Unmarshal(raw, &list); listErr != nil {
			return nil, err
		}
		countries := make([]sms.ProviderCountry, 0, len(list))
		for _, row := range list {
			code := firstNonEmpty(toString(row["id"]), toString(row["code"]))
			if code == "" {
				continue
			}
			countries = append(countries, sms.ProviderCountry{
				Code:      code,
				Name:      firstNonEmpty(toString(row["eng"]), toString(row["name"]), toString(row["chn"]), toString(row["rus"]), code),
				ShortName: firstNonEmpty(toString(row["shortName"]), toString(row["short_name"]), code),
				Region:    firstNonEmpty(toString(row["chn"]), toString(row["rus"])),
			})
		}
		return countries, nil
	}
	countries := make([]sms.ProviderCountry, 0, len(rows))
	for key, row := range rows {
		code := firstNonEmpty(rawString(row.ID), key)
		if code == "" {
			continue
		}
		countries = append(countries, sms.ProviderCountry{
			Code:      code,
			Name:      firstNonEmpty(row.Eng, row.Chn, row.Rus, row.Name, code),
			ShortName: code,
			Region:    firstNonEmpty(row.Chn, row.Rus),
		})
	}
	return countries, nil
}

func parseServices(raw []byte) ([]sms.ProviderService, error) {
	var full struct {
		Full    map[string]string `json:"full"`
		Popular map[string]string `json:"popular"`
	}
	if err := json.Unmarshal(raw, &full); err == nil && len(full.Full) > 0 {
		return servicesFromMap(full.Full), nil
	}
	if err := json.Unmarshal(raw, &full); err == nil && len(full.Popular) > 0 {
		return servicesFromMap(full.Popular), nil
	}
	var rows map[string]string
	if err := json.Unmarshal(raw, &rows); err == nil && len(rows) > 0 {
		return servicesFromMap(rows), nil
	}
	var objectRows map[string]map[string]interface{}
	if err := json.Unmarshal(raw, &objectRows); err == nil && len(objectRows) > 0 {
		services := make([]sms.ProviderService, 0, len(objectRows))
		for key, row := range objectRows {
			code := firstNonEmpty(toString(row["code"]), toString(row["id"]), key)
			name := firstNonEmpty(toString(row["name"]), toString(row["title"]), toString(row["service"]), key)
			services = append(services, sms.ProviderService{Code: code, Name: name})
		}
		return services, nil
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(raw, &list); err == nil {
		services := make([]sms.ProviderService, 0, len(list))
		for _, row := range list {
			code := firstNonEmpty(toString(row["code"]), toString(row["id"]))
			name := firstNonEmpty(toString(row["name"]), toString(row["title"]), code)
			if code != "" {
				services = append(services, sms.ProviderService{Code: code, Name: name})
			}
		}
		return services, nil
	}
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		for _, key := range []string{"services", "items", "list", "data"} {
			if value, ok := wrapped[key]; ok {
				services, err := parseServices(value)
				if err == nil {
					return services, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("unsupported herosms services response: %s", string(raw))
}

func servicesFromMap(rows map[string]string) []sms.ProviderService {
	services := make([]sms.ProviderService, 0, len(rows))
	for code, name := range rows {
		services = append(services, sms.ProviderService{Code: code, Name: firstNonEmpty(name, code)})
	}
	return services
}

func parsePrices(raw []byte, countryID, serviceID string) ([]priceItem, error) {
	var root map[string]map[string]struct {
		Cost        json.RawMessage `json:"cost"`
		Price       json.RawMessage `json:"price"`
		RetailPrice json.RawMessage `json:"retail_price"`
		Count       json.RawMessage `json:"count"`
		Quantity    json.RawMessage `json:"quantity"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	var prices []priceItem
	for country, services := range root {
		for service, row := range services {
			item := priceItem{
				CountryID: firstNonEmpty(country, countryID),
				ServiceID: firstNonEmpty(service, serviceID),
				Cost:      firstNonEmpty(rawString(row.Cost), rawString(row.Price), rawString(row.RetailPrice)),
				Count:     firstNonZeroInt(rawInt(row.Count), rawInt(row.Quantity)),
			}
			prices = append(prices, item)
		}
	}
	return prices, nil
}

func providerError(raw string) error {
	message := strings.TrimSpace(raw)
	switch {
	case strings.Contains(message, "NO_NUMBERS"):
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	case strings.Contains(message, "NO_BALANCE"):
		return sms.NewProviderError(sms.ErrBalance, message)
	case strings.Contains(message, "BAD_KEY"), strings.Contains(message, "BAD_ACTION"):
		return sms.NewProviderError("AUTH_ERROR", message)
	case strings.Contains(message, "NO_ACTIVATION"), strings.Contains(message, "BAD_STATUS"):
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	case strings.Contains(message, "EARLY_CANCEL_DENIED"):
		return sms.NewProviderError(sms.ErrCannotCancel, message)
	default:
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
}

func isPlainError(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		return false
	}
	return strings.HasPrefix(raw, "BAD_") ||
		strings.HasPrefix(raw, "NO_") ||
		strings.HasPrefix(raw, "ERROR_") ||
		strings.Contains(raw, "DENIED")
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
	return text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func priceString(price *sms.ProviderPrice) string {
	if price == nil {
		return ""
	}
	return firstNonEmpty(price.Price, price.LowPrice, price.HighPrice)
}

func rawString(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return ""
	}
	return strings.Trim(value, `"`)
}

func rawInt(raw json.RawMessage) int {
	value := rawString(raw)
	if value == "" {
		return 0
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return int(parsed)
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}

func mustMarshal(value interface{}) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
