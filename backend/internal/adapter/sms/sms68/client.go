package sms68

import (
	"context"
	"encoding/json"
	"errors"
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

const providerCode = "68sms"

const (
	countryCanada = "33"
	countryUS     = "188"
)

type Client struct {
	apiKey        string
	baseURL       string
	metadataToken string
	httpClient    *http.Client
	logger        func(model.SupplierRequestLog)
	mu            sync.RWMutex
}

type apiEnvelope struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

type balanceData struct {
	Balance json.RawMessage `json:"balance"`
}

type numberRow struct {
	Number json.RawMessage `json:"number"`
	Key    string          `json:"key"`
}

type messageRow struct {
	FromNumber json.RawMessage `json:"fromNumber"`
	ToNumber   json.RawMessage `json:"toNumber"`
	Time       string          `json:"time"`
	Message    string          `json:"message"`
}

type storeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total int        `json:"total"`
		List  []storeRow `json:"list"`
	} `json:"data"`
}

type storeRow struct {
	ID             int    `json:"id"`
	AppID          int    `json:"app_id"`
	CountryID      int    `json:"country_id"`
	CardType       int    `json:"card_type"`
	Price          string `json:"price"`
	PriceRenew     string `json:"price_renew"`
	Surplus        int    `json:"surplus"`
	Status         int    `json:"status"`
	NameEN         string `json:"name_en"`
	NameCN         string `json:"name_cn"`
	AppCode        string `json:"app_code"`
	LimitCount     int    `json:"limit_count"`
	DescEN         string `json:"desc_en"`
	DescCN         string `json:"desc_cn"`
	RateStatistics int    `json:"rate_statistics"`
	FreeTimes      int    `json:"free_times"`
}

type activityResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Time []activityRule `json:"time"`
	} `json:"data"`
}

type activityRule struct {
	ID                int    `json:"id"`
	ActCN             string `json:"act_cn"`
	ActEN             string `json:"act_en"`
	ActRuleID         int    `json:"act_rule_id"`
	MinValue          int    `json:"min_value"`
	MaxValue          int    `json:"max_value"`
	Count             int    `json:"count"`
	CountryID         int    `json:"country_id"`
	CardType          int    `json:"card_type"`
	CommissionPercent string `json:"commission_percent"`
	DiscountPercent   string `json:"discount_percent"`
	DiscountType      int    `json:"discount_type"`
	IsValid           int    `json:"is_valid"`
	StartTime         int64  `json:"start_time"`
	EndTime           int64  `json:"end_time"`
}

type segmentResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type serviceQuote struct {
	Store    *storeRow       `json:"store,omitempty"`
	Activity *activityRule   `json:"activity,omitempty"`
	Segment  json.RawMessage `json:"segment,omitempty"`
}

type loginCredential struct {
	Token         string
	Cookie        string
	Communication string
}

func New(apiKey, baseURL, metadataToken string, timeout time.Duration, logger func(model.SupplierRequestLog)) *Client {
	return &Client{
		apiKey:        apiKey,
		baseURL:       strings.TrimRight(baseURL, "/"),
		metadataToken: metadataToken,
		httpClient:    &http.Client{Timeout: timeout},
		logger:        logger,
	}
}

func (c *Client) Name() string { return providerCode }

func (c *Client) Configure(apiKey, baseURL string) {
	c.ConfigureAdvanced(apiKey, baseURL, "")
}

func (c *Client) ConfigureAdvanced(apiKey, baseURL, metadataToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if apiKey != "" {
		c.apiKey = strings.TrimSpace(apiKey)
	}
	if baseURL != "" {
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	if metadataToken != "" {
		c.metadataToken = strings.TrimSpace(metadataToken)
	}
}

func (c *Client) ProviderKind() sms.ProviderKind {
	return sms.ProviderKind{
		Kind:               "long_lived",
		ManualCheck:        true,
		MessageURLTemplate: "https://api.68sms.com/api/msg/get?key={token}",
	}
}

func (c *Client) GetMessageURL(token string) string {
	_, baseURL, _ := c.config()
	if token == "" {
		return ""
	}
	return baseURL + "/api/msg/get?key=" + url.QueryEscape(token)
}

func (c *Client) GetBalance(ctx context.Context) (*sms.ProviderBalance, error) {
	var out apiEnvelope
	raw, status, err := c.getAPI(ctx, "/api/user/balance", nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("68sms http %d: %s", status, raw)
	}
	if !successCode(out.Code) {
		return nil, providerError(statusCode(out.Code), firstNonEmpty(out.Message, out.Msg, string(raw)))
	}
	var data balanceData
	if err := json.Unmarshal(out.Data, &data); err != nil {
		return nil, err
	}
	return &sms.ProviderBalance{Balance: rawString(data.Balance)}, nil
}

func (c *Client) RequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	operator := defaultOperatorID(input.PoolID)
	rule, err := c.activeActivityRule(ctx, input.CountryID, input.ServiceID, operator)
	if err != nil {
		return nil, err
	}
	if _, err := c.segment(ctx, input.CountryID, input.ServiceID, operator, rule.ActRuleID); err != nil {
		return nil, err
	}
	query := url.Values{
		"appId":        {input.ServiceID},
		"countryId":    {input.CountryID},
		"operatorId":   {operator},
		"quantity":     {"1"},
		"numberType":   {"1"},
		"smsType":      {"1"},
		"validityType": {strconv.Itoa(rule.ActRuleID)},
	}
	var out apiEnvelope
	raw, status, err := c.requestAPI(ctx, http.MethodPost, "/api/number/get", query, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("68sms http %d: %s", status, raw)
	}
	if !successCode(out.Code) {
		return nil, providerError(statusCode(out.Code), firstNonEmpty(out.Message, out.Msg, string(raw)))
	}
	var numbers []numberRow
	if err := json.Unmarshal(out.Data, &numbers); err != nil {
		return nil, err
	}
	if len(numbers) == 0 || rawString(numbers[0].Number) == "" || strings.TrimSpace(numbers[0].Key) == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "68sms missing phone or message key")
	}
	phone := rawString(numbers[0].Number)
	token := strings.TrimSpace(numbers[0].Key)
	price, _ := c.GetPrice(ctx, sms.ProviderPriceInput{CountryID: input.CountryID, ServiceID: input.ServiceID, PoolID: operator})
	cost, _ := strconv.ParseFloat(priceString(price), 64)
	return &sms.RequestNumberResult{
		SupplierOrderID:     token,
		SupplierToken:       token,
		PhoneNumber:         normalizePhone(phone),
		PhoneCountryCode:    phoneCountryCode(phone),
		PhoneNationalNumber: phoneNationalNumber(phone),
		Country:             input.CountryID,
		Service:             input.ServiceID,
		Cost:                cost,
		Raw:                 raw,
	}, nil
}

func (c *Client) CheckSMS(ctx context.Context, input sms.CheckSMSInput) (*sms.SMSResult, error) {
	return &sms.SMSResult{Status: model.OrderActive, SupplierStatus: "manual_check"}, nil
}

func (c *Client) CheckManualSMS(ctx context.Context, input sms.ManualSMSInput) (*sms.SMSResult, error) {
	token := strings.TrimSpace(input.SupplierToken)
	if token == "" {
		return nil, sms.NewProviderError(sms.ErrOrderNotFound, "68sms message key is missing")
	}
	var out apiEnvelope
	raw, status, err := c.getAPIWithKey(ctx, "/api/msg/get", url.Values{"key": {token}}, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("68sms http %d: %s", status, raw)
	}
	code := statusCode(out.Code)
	if code == "10018" {
		return &sms.SMSResult{
			Status:         model.OrderActive,
			SupplierStatus: code,
			Raw:            raw,
		}, nil
	}
	if !successCode(out.Code) {
		return nil, providerError(code, firstNonEmpty(out.Message, out.Msg, string(raw)))
	}
	var messages []messageRow
	if err := json.Unmarshal(out.Data, &messages); err != nil {
		return nil, err
	}
	if len(messages) == 0 || strings.TrimSpace(messages[0].Message) == "" {
		return &sms.SMSResult{
			Status:         model.OrderActive,
			SupplierStatus: "10018",
			Raw:            raw,
		}, nil
	}
	content := strings.TrimSpace(messages[0].Message)
	return &sms.SMSResult{
		Status:           model.OrderSMSReceived,
		SupplierStatus:   code,
		VerificationCode: extractCode(content),
		SMSContent:       content,
		Raw:              raw,
	}, nil
}

func (c *Client) CancelNumber(ctx context.Context, input sms.CancelNumberInput) (*sms.CancelResult, error) {
	return nil, sms.NewProviderError(sms.ErrCannotCancel, "68sms long-lived numbers do not support cancellation")
}

func (c *Client) GetCountries(ctx context.Context) ([]sms.ProviderCountry, error) {
	return []sms.ProviderCountry{
		{Code: countryCanada, Name: "Canada", ShortName: "CA", DialCode: "1"},
		{Code: countryUS, Name: "United States", ShortName: "US", DialCode: "1"},
	}, nil
}

func (c *Client) GetServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	rows, err := c.storeRows(ctx, countryID)
	if err != nil {
		return nil, err
	}
	services := make([]sms.ProviderService, 0, len(rows))
	for _, row := range rows {
		if row.AppID == 0 {
			continue
		}
		services = append(services, sms.ProviderService{
			ID:          row.AppID,
			Code:        strconv.Itoa(row.AppID),
			Name:        firstNonEmpty(row.NameEN, row.NameCN, row.AppCode, strconv.Itoa(row.AppID)),
			CountryCode: strconv.Itoa(row.CountryID),
			CountryName: countryName(strconv.Itoa(row.CountryID)),
			Price:       row.Price,
			Stock:       row.Surplus,
		})
	}
	return services, nil
}

func (c *Client) GetPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	row, err := c.findStoreRow(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	return &sms.ProviderPrice{
		LowPrice:    row.Price,
		HighPrice:   row.Price,
		Price:       row.Price,
		SuccessRate: float64(row.RateStatistics),
		Raw:         mustMarshal(row),
	}, nil
}

func (c *Client) GetStock(ctx context.Context, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	quote, err := c.quote(ctx, input.CountryID, input.ServiceID, defaultOperatorID(input.PoolID))
	if err != nil {
		var providerErr *sms.ProviderError
		if errors.As(err, &providerErr) && providerErr.Code == sms.ErrOutOfStock {
			return &sms.ProviderStock{Amount: 0}, nil
		}
		return nil, err
	}
	return &sms.ProviderStock{Amount: quote.Activity.Count, Raw: mustMarshal(quote)}, nil
}

func (c *Client) quote(ctx context.Context, countryID, serviceID, operatorID string) (*serviceQuote, error) {
	row, err := c.findStoreRow(ctx, countryID, serviceID)
	if err != nil {
		return nil, err
	}
	rule, err := c.activeActivityRule(ctx, countryID, strconv.Itoa(row.AppID), operatorID)
	if err != nil {
		return nil, err
	}
	segment, err := c.segment(ctx, countryID, strconv.Itoa(row.AppID), operatorID, rule.ActRuleID)
	if err != nil {
		return nil, err
	}
	return &serviceQuote{Store: row, Activity: rule, Segment: segment}, nil
}

func (c *Client) findStoreRow(ctx context.Context, countryID, serviceID string) (*storeRow, error) {
	rows, err := c.storeRows(ctx, countryID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if serviceID == "" || strconv.Itoa(row.AppID) == serviceID || strconv.Itoa(row.ID) == serviceID || row.AppCode == serviceID {
			return &row, nil
		}
	}
	return nil, sms.NewProviderError(sms.ErrPriceNotFound, "68sms service not found")
}

func (c *Client) storeRows(ctx context.Context, countryID string) ([]storeRow, error) {
	if countryID == "" {
		countryID = countryUS
	}
	_, _, credentialText := c.config()
	credential := parseLoginCredential(credentialText)
	if credential.Token == "" {
		return nil, errors.New("68sms login credential is not configured")
	}
	query := url.Values{
		"searchContent": {""},
		"countryId":     {countryID},
		"cardType":      {"1"},
	}
	raw, status, err := c.getStore(ctx, "/admin/app/store/list", query, credential)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("68sms store http %d: %s", status, raw)
	}
	var out storeResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, sms.NewProviderError("AUTH_ERROR", "68sms login credential is invalid or expired")
	}
	if out.Code != 10000 && out.Code != 0 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Message, string(raw)))
	}
	return out.Data.List, nil
}

func (c *Client) activeActivityRule(ctx context.Context, countryID, serviceID, operatorID string) (*activityRule, error) {
	if countryID == "" {
		countryID = countryUS
	}
	if serviceID == "" {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "68sms service is required")
	}
	query := url.Values{
		"appId":      {serviceID},
		"countryId":  {countryID},
		"operatorId": {operatorID},
		"cardType":   {"1"},
		"smsType":    {"1"},
	}
	raw, status, err := c.getStoreWithCredential(ctx, "/admin/api/activity", query)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("68sms activity http %d: %s", status, raw)
	}
	var out activityResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, sms.NewProviderError("AUTH_ERROR", "68sms login credential is invalid or expired")
	}
	if out.Code != 10000 && out.Code != 0 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Message, string(raw)))
	}
	var selected *activityRule
	for index := range out.Data.Time {
		rule := out.Data.Time[index]
		if rule.ActRuleID <= 0 || rule.Count <= 0 {
			continue
		}
		if selected == nil || rule.ActRuleID > selected.ActRuleID {
			selected = &rule
		}
	}
	if selected == nil {
		return nil, sms.NewProviderError(sms.ErrOutOfStock, "68sms has no available validity rule for this service")
	}
	return selected, nil
}

func (c *Client) segment(ctx context.Context, countryID, serviceID, operatorID string, actRuleID int) (json.RawMessage, error) {
	query := url.Values{
		"type":       {"1"},
		"smsType":    {"1"},
		"appId":      {serviceID},
		"countryId":  {countryID},
		"operatorId": {operatorID},
		"cardType":   {"1"},
		"actRuleId":  {strconv.Itoa(actRuleID)},
	}
	raw, status, err := c.getStoreWithCredential(ctx, "/admin/did/segment", query)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("68sms segment http %d: %s", status, raw)
	}
	var out segmentResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, sms.NewProviderError("AUTH_ERROR", "68sms login credential is invalid or expired")
	}
	if out.Code != 10000 && out.Code != 0 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Message, string(raw)))
	}
	return out.Data, nil
}

func defaultOperatorID(poolID string) string {
	if strings.TrimSpace(poolID) != "" {
		return strings.TrimSpace(poolID)
	}
	return "2"
}

func (c *Client) getStoreWithCredential(ctx context.Context, path string, query url.Values) (json.RawMessage, int, error) {
	_, _, credentialText := c.config()
	credential := parseLoginCredential(credentialText)
	if credential.Token == "" {
		return nil, 0, errors.New("68sms login credential is not configured")
	}
	return c.getStore(ctx, path, query, credential)
}

func (c *Client) getAPI(ctx context.Context, path string, query url.Values, out interface{}) (json.RawMessage, int, error) {
	apiKey, _, _ := c.config()
	if query == nil {
		query = url.Values{}
	}
	query.Set("key", apiKey)
	return c.requestAPI(ctx, http.MethodGet, path, query, out)
}

func (c *Client) getAPIWithKey(ctx context.Context, path string, query url.Values, out interface{}) (json.RawMessage, int, error) {
	return c.requestAPI(ctx, http.MethodGet, path, query, out)
}

func (c *Client) requestAPI(ctx context.Context, method, path string, query url.Values, out interface{}) (json.RawMessage, int, error) {
	_, baseURL, _ := c.config()
	if query == nil {
		query = url.Values{}
	}
	target := baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
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
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			c.log(path, query, resp.StatusCode, false, err.Error(), time.Since(start), body)
			return body, resp.StatusCode, err
		}
	}
	c.log(path, query, resp.StatusCode, resp.StatusCode < 400, "", time.Since(start), body)
	return body, resp.StatusCode, nil
}

func (c *Client) getStore(ctx context.Context, path string, query url.Values, credential loginCredential) (json.RawMessage, int, error) {
	_, _, _ = c.config()
	target := "https://www.68sms.com" + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh")
	req.Header.Set("Referer", "https://www.68sms.com/app/store")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Token", credential.Token)
	if credential.Communication != "" {
		req.Header.Set("Communication", credential.Communication)
	} else {
		req.Header.Set("Communication", "wgwZhI971t0obO+Pj/BJA==")
	}
	if credential.Cookie != "" {
		req.Header.Set("Cookie", credential.Cookie)
	}
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
	c.log(path, query, resp.StatusCode, resp.StatusCode < 400, "", time.Since(start), body)
	return body, resp.StatusCode, nil
}

func (c *Client) config() (string, string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey, c.baseURL, c.metadataToken
}

func (c *Client) log(action string, query url.Values, status int, success bool, message string, latency time.Duration, body []byte) {
	if c.logger == nil {
		return
	}
	safe := url.Values{}
	for key, values := range query {
		if key == "key" {
			safe.Set(key, "***")
			continue
		}
		if strings.EqualFold(key, "token") {
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

func successCode(raw json.RawMessage) bool {
	return statusCode(raw) == "10000"
}

func statusCode(raw json.RawMessage) string {
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

func providerError(code string, message string) error {
	switch code {
	case "10010":
		return sms.NewProviderError(sms.ErrOutOfStock, firstNonEmpty(message, "68sms stock is insufficient"))
	case "10011":
		return sms.NewProviderError(sms.ErrBalance, firstNonEmpty(message, "68sms balance is insufficient"))
	case "10019":
		return sms.NewProviderError(sms.ErrProviderRejected, firstNonEmpty(message, "68sms data error"))
	case "10022":
		return sms.NewProviderError("AUTH_ERROR", firstNonEmpty(message, "68sms unauthorized"))
	case "10001":
		return sms.NewProviderError(sms.ErrProviderRejected, firstNonEmpty(message, "68sms parameter error"))
	case "10018":
		return sms.NewProviderError(sms.ErrOrderNotFound, firstNonEmpty(message, "68sms record not found"))
	case "10100":
		return sms.NewProviderError(sms.ErrProviderRejected, firstNonEmpty(message, "68sms request failed"))
	}
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "balance") || strings.Contains(message, "余额"):
		return sms.NewProviderError(sms.ErrBalance, message)
	case strings.Contains(normalized, "stock") || strings.Contains(normalized, "surplus") || strings.Contains(message, "库存") || strings.Contains(message, "号码"):
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	case strings.Contains(normalized, "token") || strings.Contains(normalized, "key") || strings.Contains(message, "登录"):
		return sms.NewProviderError("AUTH_ERROR", message)
	default:
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
}

func parseLoginCredential(text string) loginCredential {
	credential := loginCredential{}
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "token":
			credential.Token = value
		case "cookie":
			credential.Cookie = value
		case "communication":
			credential.Communication = value
		}
	}
	if credential.Token == "" {
		trimmed := strings.TrimSpace(text)
		if trimmed != "" && !strings.Contains(trimmed, ":") {
			credential.Token = strings.Join(strings.Fields(trimmed), "")
		}
	}
	credential.Token = strings.Join(strings.Fields(credential.Token), "")
	return credential
}

func isWaitingMessage(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	return normalized == "" ||
		strings.Contains(normalized, "wait") ||
		strings.Contains(normalized, "empty") ||
		strings.Contains(normalized, "no sms") ||
		strings.Contains(normalized, "not found") ||
		strings.Contains(message, "暂无") ||
		strings.Contains(message, "没有")
}

func extractCode(text string) string {
	if match := regexp.MustCompile(`\d{3,8}`).FindString(text); match != "" {
		return match
	}
	return strings.TrimSpace(text)
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" || strings.HasPrefix(phone, "+") {
		return phone
	}
	return "+" + phone
}

func phoneCountryCode(phone string) string {
	phone = strings.TrimPrefix(strings.TrimSpace(phone), "+")
	if strings.HasPrefix(phone, "1") && len(phone) >= 11 {
		return "1"
	}
	return ""
}

func phoneNationalNumber(phone string) string {
	phone = strings.TrimPrefix(strings.TrimSpace(phone), "+")
	if strings.HasPrefix(phone, "1") && len(phone) >= 11 {
		return phone[1:]
	}
	return phone
}

func countryName(countryID string) string {
	switch countryID {
	case countryCanada:
		return "Canada"
	case countryUS:
		return "United States"
	default:
		return countryID
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
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return strings.TrimSpace(text)
		}
	}
	return strings.Trim(value, `"`)
}

func mustMarshal(value interface{}) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
