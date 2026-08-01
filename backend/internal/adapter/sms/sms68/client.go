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
	countryUK     = "187"
	simVirtual    = "1"
	simPhysical   = "2"
)

type Client struct {
	apiKey               string
	baseURL              string
	metadataToken        string
	httpClient           *http.Client
	logger               func(model.SupplierRequestLog)
	communicationUpdater func(string, string) error
	mu                   sync.RWMutex
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
	ActRuleID int             `json:"act_rule_id"`
	MinValue  int             `json:"min_value"`
	MaxValue  int             `json:"max_value"`
	Count     int             `json:"count"`
	Raw       json.RawMessage `json:"-"`
}

type serviceConfigMetadata struct {
	ValidityType string `json:"validityType"`
	SIMType      string `json:"simType"`
	OperatorID   string `json:"operatorId"`
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

// SetCommunicationUpdater persists provider-issued Communication rotations.
func (c *Client) SetCommunicationUpdater(updater func(string, string) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.communicationUpdater = updater
}

func (c *Client) ProviderKind() sms.ProviderKind {
	return sms.ProviderKind{
		Kind:               "long_lived",
		ManualCheck:        true,
		MessageURLTemplate: "https://api.68sms.com/api/sms/get?key={token}",
	}
}

func (c *Client) GetMessageURL(token string) string {
	_, baseURL, _ := c.config()
	if token == "" {
		return ""
	}
	return baseURL + "/api/sms/get?key=" + url.QueryEscape(token)
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
	apiKey, _, _ := c.config()
	if strings.TrimSpace(apiKey) == "" {
		return nil, sms.NewProviderError("AUTH_ERROR", "68sms api key is not configured")
	}

	simType := simTypeFromMetadata(input.Metadata)
	operator := operatorIDFromMetadata(input.Metadata)
	var lastErr error
	for _, validityType := range validityTypes(input.Metadata) {
		query := url.Values{
			"key":          {apiKey},
			"appId":        {input.ServiceID},
			"countryId":    {input.CountryID},
			"operatorId":   {operator},
			"simType":      {simType},
			"quantity":     {"1"},
			"numberType":   {"1"},
			"smsType":      {"1"},
			"validityType": {validityType},
			"segment":      {""},
			"segmentBlock": {""},
		}
		var out apiEnvelope
		raw, status, err := c.requestAPI(ctx, http.MethodGet, "/api/number/get", query, &out)
		if err != nil {
			return nil, err
		}
		if status >= 400 {
			return nil, fmt.Errorf("68sms http %d: %s", status, raw)
		}
		if !successCode(out.Code) {
			err := providerError(statusCode(out.Code), firstNonEmpty(out.Message, out.Msg, string(raw)))
			var providerErr *sms.ProviderError
			if errors.As(err, &providerErr) {
				switch providerErr.Code {
				case "AUTH_ERROR", sms.ErrBalance, sms.ErrRateLimited:
					return nil, err
				}
			}
			lastErr = err
			continue
		}
		var numbers []numberRow
		if err := json.Unmarshal(out.Data, &numbers); err != nil {
			return nil, err
		}
		if len(numbers) == 0 || rawString(numbers[0].Number) == "" || strings.TrimSpace(numbers[0].Key) == "" {
			lastErr = sms.NewProviderError(sms.ErrOutOfStock, "68sms returned no available number")
			continue
		}
		phone := rawString(numbers[0].Number)
		token := strings.TrimSpace(numbers[0].Key)
		return &sms.RequestNumberResult{
			SupplierOrderID:     token,
			SupplierToken:       token,
			PhoneNumber:         normalizePhone(phone),
			PhoneCountryCode:    phoneCountryCode(phone),
			PhoneNationalNumber: phoneNationalNumber(phone),
			Country:             input.CountryID,
			Service:             input.ServiceID,
			Cost:                input.MaxPrice,
			Raw:                 raw,
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, sms.NewProviderError(sms.ErrOutOfStock, "68sms has no available number")
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
	raw, status, err := c.getAPIWithKey(ctx, "/api/sms/get", url.Values{"key": {token}}, &out)
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
	return c.GetCountriesWithScope(ctx, sms.MetadataScope{SIMType: simVirtual})
}

func (c *Client) GetServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	return c.GetServicesWithScope(ctx, countryID, sms.MetadataScope{SIMType: simVirtual})
}

func (c *Client) MetadataScopes() []sms.MetadataScope {
	return []sms.MetadataScope{{SIMType: simVirtual}, {SIMType: simPhysical}}
}

func (c *Client) GetCountriesWithScope(ctx context.Context, scope sms.MetadataScope) ([]sms.ProviderCountry, error) {
	simType := normalizeSIMType(scope.SIMType)
	candidates := []sms.ProviderCountry{
		{Code: countryCanada, Name: "Canada", ShortName: "CA", DialCode: "1", SIMType: simType},
		{Code: countryUS, Name: "United States", ShortName: "US", DialCode: "1", SIMType: simType},
		{Code: countryUK, Name: "United Kingdom", ShortName: "GB", DialCode: "44", SIMType: simType},
	}
	countries := make([]sms.ProviderCountry, 0, len(candidates))
	var requestErrs []error
	for _, country := range candidates {
		rows, err := c.storeRows(ctx, country.Code, simType)
		if err != nil {
			requestErrs = append(requestErrs, fmt.Errorf("country %s: %w", country.Code, err))
			continue
		}
		if len(rows) > 0 {
			countries = append(countries, country)
		}
	}
	if len(requestErrs) > 0 {
		return nil, errors.Join(requestErrs...)
	}
	return countries, nil
}

func (c *Client) GetServicesWithScope(ctx context.Context, countryID string, scope sms.MetadataScope) ([]sms.ProviderService, error) {
	simType := normalizeSIMType(scope.SIMType)
	rows, err := c.storeRows(ctx, countryID, simType)
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
			SIMType:     simType,
		})
	}
	return services, nil
}

func (c *Client) GetValidityOptions(ctx context.Context, input sms.ValidityOptionsInput) ([]sms.ProviderValidityOption, error) {
	countryID := strings.TrimSpace(input.CountryID)
	if countryID == "" {
		countryID = countryUS
	}
	serviceID := strings.TrimSpace(input.ServiceID)
	if serviceID == "" {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "68sms service is required")
	}
	query := url.Values{
		"appId":      {serviceID},
		"countryId":  {countryID},
		"operatorId": {defaultOperatorID(input.SIMType)},
		"simType":    {normalizeSIMType(input.SIMType)},
		"cardType":   {"1"},
		"smsType":    {"1"},
	}
	raw, status, err := c.getStoreWithCredential(ctx, "/admin/api/activity", query)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, storeHTTPError("activity", status, raw)
	}
	var out activityResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, sms.NewProviderError("AUTH_ERROR", "68sms login credential is invalid or expired")
	}
	if out.Code != 10000 && out.Code != 0 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Message, string(raw)))
	}
	options := make([]sms.ProviderValidityOption, 0, len(out.Data.Time))
	for _, rule := range out.Data.Time {
		if rule.ActRuleID <= 0 {
			continue
		}
		options = append(options, sms.ProviderValidityOption{
			Value:   strconv.Itoa(rule.ActRuleID),
			Label:   fmt.Sprintf("%d-%d", rule.MinValue, rule.MaxValue),
			MinDays: rule.MinValue,
			MaxDays: rule.MaxValue,
			Stock:   rule.Count,
			Raw:     mustMarshal(rule),
		})
	}
	return options, nil
}
func (c *Client) GetPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	row, err := c.findStoreRow(ctx, input.CountryID, input.ServiceID, input.SIMType)
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
	return &sms.ProviderStock{Amount: 0}, nil
}
func (c *Client) findStoreRow(ctx context.Context, countryID, serviceID, simType string) (*storeRow, error) {
	rows, err := c.storeRows(ctx, countryID, normalizeSIMType(simType))
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

func (c *Client) storeRows(ctx context.Context, countryID, simType string) ([]storeRow, error) {
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
		"simType":       {normalizeSIMType(simType)},
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

func validityTypes(metadata json.RawMessage) []string {
	values := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range values {
			if existing == value {
				return
			}
		}
		values = append(values, value)
	}

	var parsed serviceConfigMetadata
	if len(metadata) > 0 && json.Unmarshal(metadata, &parsed) == nil {
		add(parsed.ValidityType)
	}
	for _, value := range []string{"4", "3", "2", "1"} {
		add(value)
	}
	return values
}

func simTypeFromMetadata(metadata json.RawMessage) string {
	var parsed serviceConfigMetadata
	if len(metadata) > 0 && json.Unmarshal(metadata, &parsed) == nil {
		return normalizeSIMType(parsed.SIMType)
	}
	return simVirtual
}

func normalizeSIMType(value string) string {
	if strings.TrimSpace(value) == simPhysical {
		return simPhysical
	}
	return simVirtual
}

func storeHTTPError(action string, status int, raw json.RawMessage) error {
	body := strings.TrimSpace(string(raw))
	if status == http.StatusUnauthorized || status == http.StatusForbidden || status >= http.StatusInternalServerError || body == "" {
		return sms.NewProviderError("AUTH_ERROR", fmt.Sprintf("68sms %s request failed with HTTP %d. Please check whether the login credential Token, Cookie, and Communication are complete and not expired", action, status))
	}
	return fmt.Errorf("68sms %s http %d: %s", action, status, raw)
}
func defaultOperatorID(simType string) string {
	if normalizeSIMType(simType) == simPhysical {
		return "5"
	}
	return "2"
}

func operatorIDFromMetadata(metadata json.RawMessage) string {
	var parsed serviceConfigMetadata
	if len(metadata) > 0 && json.Unmarshal(metadata, &parsed) == nil {
		if operatorID := strings.TrimSpace(parsed.OperatorID); operatorID != "" {
			return operatorID
		}
		return defaultOperatorID(parsed.SIMType)
	}
	return defaultOperatorID(simVirtual)
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
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		c.updateCommunication(firstNonEmpty(resp.Header.Get("Communication"), resp.Trailer.Get("Communication")))
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

func formatLoginCredential(credential loginCredential) string {
	parts := make([]string, 0, 3)
	if credential.Token != "" {
		parts = append(parts, "Token: "+credential.Token)
	}
	if credential.Cookie != "" {
		parts = append(parts, "Cookie: "+credential.Cookie)
	}
	if credential.Communication != "" {
		parts = append(parts, "Communication: "+credential.Communication)
	}
	return strings.Join(parts, "\n")
}

func (c *Client) updateCommunication(value string) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 1024 || strings.ContainsAny(value, "\r\n") {
		return
	}

	c.mu.Lock()
	credential := parseLoginCredential(c.metadataToken)
	if credential.Token == "" || credential.Communication == value {
		c.mu.Unlock()
		return
	}
	credential.Communication = value
	updatedCredential := formatLoginCredential(credential)
	c.metadataToken = updatedCredential
	updater := c.communicationUpdater
	c.mu.Unlock()

	if updater == nil {
		return
	}
	if err := updater(value, updatedCredential); err != nil {
		c.log("communication_refresh", nil, 0, false, err.Error(), 0, nil)
		return
	}
	c.log("communication_refresh", nil, http.StatusOK, true, "", 0, nil)
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
	case countryUK:
		return "United Kingdom"
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
