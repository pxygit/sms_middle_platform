package lubansms

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

const providerCode = "lubansms"
const listPageSize = 200

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     func(model.SupplierRequestLog)
	mu         sync.RWMutex
}

type apiResponse struct {
	Code      int             `json:"code"`
	Msg       json.RawMessage `json:"msg"`
	Balance   string          `json:"balance"`
	Number    string          `json:"number"`
	RequestID json.RawMessage `json:"request_id"`
	SMSCode   string          `json:"sms_code"`
}

type serviceRow struct {
	ServiceID     string          `json:"service_id"`
	CountryNameZH string          `json:"country_name_zh"`
	CountryNameEN string          `json:"country_name_en"`
	ServiceName   string          `json:"service_name"`
	Provider      string          `json:"provider"`
	Cost          json.RawMessage `json:"cost"`
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
	var out apiResponse
	raw, status, err := c.getJSON(ctx, "getBalance", nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("lubansms http %d: %s", status, raw)
	}
	if out.Code != 0 {
		return nil, providerError(out.Code, rawString(out.Msg))
	}
	return &sms.ProviderBalance{Balance: out.Balance}, nil
}

func (c *Client) RequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	query := url.Values{"service_id": {input.ServiceID}}
	var out apiResponse
	raw, status, err := c.getJSON(ctx, "getNumber", query, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("lubansms http %d: %s", status, raw)
	}
	if out.Code != 0 {
		return nil, providerError(out.Code, rawString(out.Msg))
	}
	requestID := rawString(out.RequestID)
	if requestID == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "lubansms missing request_id")
	}
	price, _ := c.GetPrice(ctx, sms.ProviderPriceInput{CountryID: input.CountryID, ServiceID: input.ServiceID})
	cost, _ := strconv.ParseFloat(firstNonEmpty(priceString(price), "0"), 64)
	return &sms.RequestNumberResult{
		SupplierOrderID: requestID,
		PhoneNumber:     normalizePhone(out.Number),
		Country:         input.CountryID,
		Service:         input.ServiceID,
		Cost:            cost,
		Raw:             raw,
	}, nil
}

func (c *Client) CheckSMS(ctx context.Context, input sms.CheckSMSInput) (*sms.SMSResult, error) {
	var out apiResponse
	raw, status, err := c.getJSON(ctx, "getSms", url.Values{"request_id": {input.SupplierOrderID}}, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("lubansms http %d: %s", status, raw)
	}
	message := rawString(out.Msg)
	result := &sms.SMSResult{
		SupplierStatus: message,
		Raw:            raw,
	}
	switch {
	case out.Code == 0 && message == "success":
		result.Status = model.OrderSMSReceived
		result.VerificationCode = firstNonEmpty(out.SMSCode, extractCode(rawString(out.Msg)))
		result.SMSContent = out.SMSCode
	case out.Code == 0 && message == "wait":
		result.Status = model.OrderActive
	case out.Code == 400 && message == "wrong_status":
		result.Status = model.OrderExpired
	default:
		return nil, providerError(out.Code, message)
	}
	return result, nil
}

func (c *Client) CancelNumber(ctx context.Context, input sms.CancelNumberInput) (*sms.CancelResult, error) {
	var out apiResponse
	raw, status, err := c.getJSON(ctx, "setStatus", url.Values{
		"request_id": {input.SupplierOrderID},
		"status":     {"reject"},
	}, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("lubansms http %d: %s", status, raw)
	}
	message := rawString(out.Msg)
	if out.Code == 0 && (message == "" || message == "success") {
		return &sms.CancelResult{Success: true, Message: message, Raw: raw}, nil
	}
	return nil, providerError(out.Code, message)
}

func (c *Client) GetCountries(ctx context.Context) ([]sms.ProviderCountry, error) {
	var out apiResponse
	raw, status, err := c.getJSON(ctx, "countries", nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("lubansms http %d: %s", status, raw)
	}
	if out.Code != 0 {
		return nil, providerError(out.Code, rawString(out.Msg))
	}
	var rows []struct {
		ID     string `json:"id"`
		NameEN string `json:"name_en"`
		NameCN string `json:"name_cn"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(out.Msg, &rows); err != nil {
		return nil, err
	}
	countries := make([]sms.ProviderCountry, 0, len(rows))
	for _, row := range rows {
		code := firstNonEmpty(row.NameEN, row.NameCN, row.Code, row.ID)
		if code == "" {
			continue
		}
		countries = append(countries, sms.ProviderCountry{
			Code:      code,
			Name:      firstNonEmpty(row.NameEN, row.NameCN, row.Code),
			ShortName: firstNonEmpty(row.Code, row.ID),
			Region:    row.NameCN,
		})
	}
	return countries, nil
}

func (c *Client) GetServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	rows, err := c.listAll(ctx, countryID)
	if err != nil {
		return nil, err
	}
	services := make([]sms.ProviderService, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ServiceID == "" || seen[row.ServiceID] {
			continue
		}
		seen[row.ServiceID] = true
		services = append(services, sms.ProviderService{
			Code:        row.ServiceID,
			Name:        firstNonEmpty(row.ServiceName, row.ServiceID),
			CountryCode: firstNonEmpty(row.CountryNameEN, countryID),
			CountryName: firstNonEmpty(row.CountryNameEN, row.CountryNameZH),
			Price:       rawString(row.Cost),
		})
	}
	return services, nil
}

func (c *Client) GetPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	rows, err := c.list(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	var selected *serviceRow
	for index, row := range rows {
		if input.ServiceID != "" && row.ServiceID != "" && row.ServiceID != input.ServiceID {
			continue
		}
		if selected == nil || priceLess(rawString(row.Cost), rawString(selected.Cost)) {
			selected = &rows[index]
		}
	}
	if selected == nil {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "lubansms price not found")
	}
	price := rawString(selected.Cost)
	return &sms.ProviderPrice{
		LowPrice:  price,
		HighPrice: price,
		Price:     price,
		Raw:       mustMarshal(selected),
	}, nil
}

func (c *Client) GetStock(ctx context.Context, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	rows, err := c.list(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	amount := 0
	for _, row := range rows {
		if input.ServiceID != "" && row.ServiceID != "" && row.ServiceID != input.ServiceID {
			continue
		}
		amount++
	}
	return &sms.ProviderStock{Amount: amount, Raw: mustMarshal(rows)}, nil
}

func (c *Client) list(ctx context.Context, countryID, serviceID string) ([]serviceRow, error) {
	return c.listPage(ctx, countryID, serviceID, 1, listPageSize)
}

func (c *Client) listAll(ctx context.Context, countryID string) ([]serviceRow, error) {
	var all []serviceRow
	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		rows, err := c.listPage(ctx, countryID, "", page, listPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if len(rows) < listPageSize {
			break
		}
	}
	return all, nil
}

func (c *Client) listPage(ctx context.Context, countryID, serviceID string, page, pageSize int) ([]serviceRow, error) {
	query := url.Values{
		"language":  {"en"},
		"page":      {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(pageSize)},
	}
	if countryID != "" {
		query.Set("country", countryID)
	}
	if serviceID != "" {
		query.Set("service", serviceID)
	}
	var out apiResponse
	raw, status, err := c.getJSON(ctx, "List", query, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("lubansms http %d: %s", status, raw)
	}
	if out.Code != 0 {
		return nil, providerError(out.Code, rawString(out.Msg))
	}
	var rows []serviceRow
	if err := json.Unmarshal(out.Msg, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out interface{}) (json.RawMessage, int, error) {
	apiKey, baseURL := c.config()
	if query == nil {
		query = url.Values{}
	}
	query.Set("apikey", apiKey)
	target := baseURL + "/v2/api/" + path + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log(path, query, 0, false, err.Error(), time.Since(start), nil)
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log(path, query, resp.StatusCode, false, err.Error(), time.Since(start), body)
		return body, resp.StatusCode, err
	}
	if err := json.Unmarshal(body, out); err != nil {
		c.log(path, query, resp.StatusCode, false, err.Error(), time.Since(start), body)
		return body, resp.StatusCode, err
	}
	c.log(path, query, resp.StatusCode, resp.StatusCode < 400, "", time.Since(start), body)
	return body, resp.StatusCode, nil
}

func (c *Client) config() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey, c.baseURL
}

func (c *Client) log(action string, query url.Values, status int, success bool, message string, latency time.Duration, body []byte) {
	if c.logger == nil {
		return
	}
	safe := url.Values{}
	for key, values := range query {
		if key == "apikey" {
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
		ResponseBody: string(body),
	})
}

func providerError(code int, message string) error {
	message = strings.TrimSpace(message)
	switch {
	case code == 401 || strings.Contains(message, "余额不足") || strings.Contains(strings.ToLower(message), "balance"):
		return sms.NewProviderError(sms.ErrBalance, message)
	case strings.Contains(message, "apikey"):
		return sms.NewProviderError("AUTH_ERROR", message)
	case strings.Contains(message, "没有可用号码"), strings.Contains(strings.ToLower(message), "no number"):
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	case strings.Contains(message, "wrong_status"):
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	case strings.Contains(message, "无法关闭"), strings.Contains(strings.ToLower(message), "close error"):
		return sms.NewProviderError(sms.ErrCannotCancel, message)
	case strings.Contains(message, "not found"), strings.Contains(message, "error"):
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	default:
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
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

func priceLess(left, right string) bool {
	leftPrice, leftErr := strconv.ParseFloat(left, 64)
	rightPrice, rightErr := strconv.ParseFloat(right, 64)
	if leftErr != nil {
		return false
	}
	if rightErr != nil {
		return true
	}
	return leftPrice < rightPrice
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

func mustMarshal(value interface{}) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
