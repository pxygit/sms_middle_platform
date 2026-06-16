package firefox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/model"
)

type Client struct {
	credential string
	baseURL    string
	httpClient *http.Client
	logger     func(model.SupplierRequestLog)
	mu         sync.RWMutex
}

type itemRow struct {
	ID        int
	Name      string
	Country   string
	CountryID int
	Price     string
	Stock     int
}

func New(credential, baseURL string, timeout time.Duration, logger func(model.SupplierRequestLog)) *Client {
	return &Client{
		credential: credential,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}
}

func (c *Client) Name() string { return "firefox" }

func (c *Client) Configure(apiKey, baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if apiKey != "" {
		c.credential = apiKey
	}
	if baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

func (c *Client) GetBalance(ctx context.Context) (*sms.ProviderBalance, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	raw, status, err := c.call(ctx, "myInfo", url.Values{"token": {token}})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("firefox http %d: %s", status, raw)
	}
	parts := strings.Split(raw, "|")
	if len(parts) >= 2 && parts[0] == "1" {
		return &sms.ProviderBalance{Balance: parts[1]}, nil
	}
	return nil, providerError(raw)
}

func (c *Client) RequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"token": {token},
		"iid":   {input.ServiceID},
	}
	if input.CountryID != "" {
		form.Set("country", input.CountryID)
	}
	if input.MaxPrice > 0 {
		form.Set("maxPrice", strconv.FormatFloat(input.MaxPrice, 'f', 4, 64))
	}
	raw, status, err := c.call(ctx, "getPhone", form)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("firefox http %d: %s", status, raw)
	}
	parts := strings.Split(raw, "|")
	if len(parts) >= 8 && parts[0] == "1" {
		return &sms.RequestNumberResult{
			SupplierOrderID:     parts[1],
			PhoneNumber:         buildPhone(parts[4], parts[7]),
			PhoneCountryCode:    parts[4],
			PhoneNationalNumber: parts[7],
			Raw:                 json.RawMessage(strconv.Quote(raw)),
		}, nil
	}
	err = phoneError(raw)
	if isOutOfStock(err) {
		return nil, sms.NewProviderError(sms.ErrOutOfStock, err.Error())
	}
	return nil, err
}

func (c *Client) CheckSMS(ctx context.Context, input sms.CheckSMSInput) (*sms.SMSResult, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	raw, status, err := c.call(ctx, "getPhoneCode", url.Values{
		"token": {token},
		"pkey":  {input.SupplierOrderID},
	})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("firefox http %d: %s", status, raw)
	}
	parts := strings.Split(raw, "|")
	if len(parts) >= 3 && parts[0] == "1" {
		return &sms.SMSResult{
			Status:           model.OrderSMSReceived,
			SupplierStatus:   parts[0],
			VerificationCode: parts[1],
			SMSContent:       parts[2],
			Raw:              json.RawMessage(strconv.Quote(raw)),
		}, nil
	}
	switch failureCode(raw) {
	case "-3":
		return &sms.SMSResult{Status: model.OrderActive, SupplierStatus: "-3", SMSContent: raw, Raw: json.RawMessage(strconv.Quote(raw))}, nil
	case "-4":
		return &sms.SMSResult{Status: model.OrderCancelled, SupplierStatus: "-4", SMSContent: raw, Raw: json.RawMessage(strconv.Quote(raw))}, nil
	default:
		return nil, codeError(raw)
	}
}

func (c *Client) CancelNumber(ctx context.Context, input sms.CancelNumberInput) (*sms.CancelResult, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	raw, status, err := c.call(ctx, "setRel", url.Values{
		"token": {token},
		"pkey":  {input.SupplierOrderID},
	})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("firefox http %d: %s", status, raw)
	}
	if strings.HasPrefix(raw, "1|") || strings.TrimSpace(raw) == "1" {
		return &sms.CancelResult{Success: true, Message: raw, Raw: json.RawMessage(strconv.Quote(raw))}, nil
	}
	return nil, releaseError(raw)
}

func (c *Client) GetCountries(ctx context.Context) ([]sms.ProviderCountry, error) {
	items, err := c.items(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[int]sms.ProviderCountry{}
	for _, item := range items {
		if item.CountryID == 0 {
			continue
		}
		seen[item.CountryID] = sms.ProviderCountry{
			ID:        item.CountryID,
			Name:      item.Country,
			ShortName: strconv.Itoa(item.CountryID),
			Region:    "",
		}
	}
	countries := make([]sms.ProviderCountry, 0, len(seen))
	for _, country := range seen {
		countries = append(countries, country)
	}
	return countries, nil
}

func (c *Client) GetServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	items, err := c.items(ctx)
	if err != nil {
		return nil, err
	}
	services := make([]sms.ProviderService, 0, len(items))
	seen := map[int]bool{}
	for _, item := range items {
		if countryID != "" && item.CountryID != 0 && strconv.Itoa(item.CountryID) != countryID {
			continue
		}
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		services = append(services, sms.ProviderService{
			ID:          item.ID,
			Name:        item.Name,
			CountryID:   item.CountryID,
			CountryName: item.Country,
			Price:       item.Price,
			Stock:       item.Stock,
		})
	}
	return services, nil
}

func (c *Client) GetPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	item, err := c.findItem(ctx, input)
	if err != nil {
		return nil, err
	}
	return &sms.ProviderPrice{Price: item.Price, LowPrice: item.Price, HighPrice: item.Price, SuccessRate: 0}, nil
}

func (c *Client) GetStock(ctx context.Context, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	item, err := c.findItem(ctx, sms.ProviderPriceInput(input))
	if err != nil {
		return &sms.ProviderStock{Amount: 0}, nil
	}
	return &sms.ProviderStock{Amount: item.Stock}, nil
}

func (c *Client) findItem(ctx context.Context, input sms.ProviderPriceInput) (*itemRow, error) {
	items, err := c.items(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if strconv.Itoa(item.ID) == input.ServiceID && (input.CountryID == "" || item.CountryID == 0 || strconv.Itoa(item.CountryID) == input.CountryID) {
			return &item, nil
		}
	}
	return nil, sms.NewProviderError(sms.ErrPriceNotFound, "firefox item not found")
}

func (c *Client) items(ctx context.Context) ([]itemRow, error) {
	raw, status, err := c.call(ctx, "getItem", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("firefox http %d: %s", status, raw)
	}
	if strings.HasPrefix(raw, "0|") || strings.HasPrefix(raw, "-") {
		return nil, providerError(raw)
	}
	return parseItems(raw)
}

func (c *Client) token(ctx context.Context) (string, error) {
	credential, _ := c.config()
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return "", errors.New("firefox credential is not configured")
	}
	if !strings.Contains(credential, ":") {
		return credential, nil
	}
	parts := strings.SplitN(credential, ":", 2)
	raw, status, err := c.call(ctx, "login", url.Values{
		"ApiName":  {parts[0]},
		"PassWord": {parts[1]},
	})
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("firefox http %d: %s", status, raw)
	}
	tokenParts := strings.Split(raw, "|")
	if len(tokenParts) >= 2 && tokenParts[0] == "1" {
		return tokenParts[1], nil
	}
	return "", providerError(raw)
}

func (c *Client) call(ctx context.Context, action string, query url.Values) (string, int, error) {
	_, baseURL := c.config()
	if query == nil {
		query = url.Values{}
	}
	query.Set("act", action)
	target := baseURL + "/yhapi.ashx?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "text/plain, application/json, */*")
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
	c.log(action, query, resp.StatusCode, resp.StatusCode < 400 && !strings.HasPrefix(text, "0|") && !strings.HasPrefix(text, "-"), "", time.Since(start), text)
	return text, resp.StatusCode, nil
}

func (c *Client) config() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.credential, c.baseURL
}

func (c *Client) log(action string, query url.Values, status int, success bool, message string, latency time.Duration, body string) {
	if c.logger == nil {
		return
	}
	safe := url.Values{}
	for key, values := range query {
		if key == "PassWord" || key == "token" {
			safe.Set(key, "***")
			continue
		}
		for _, value := range values {
			safe.Add(key, value)
		}
	}
	c.logger(model.SupplierRequestLog{
		ProviderCode: "firefox",
		Action:       action,
		HTTPStatus:   status,
		Success:      success,
		ErrorMessage: message,
		LatencyMS:    latency.Milliseconds(),
		RequestBody:  safe.Encode(),
		ResponseBody: body,
	})
}

func providerError(raw string) error {
	code := failureCode(raw)
	message := strings.TrimSpace(raw)
	switch code {
	case "-1", "-2", "-5", "-7", "-9":
		return sms.NewProviderError("AUTH_ERROR", message)
	case "-8":
		return sms.NewProviderError(sms.ErrBalance, message)
	case "-4":
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	case "-6":
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	default:
		if seconds, err := strconv.Atoi(code); err == nil && seconds > 0 {
			return sms.NewProviderError(sms.ErrCannotCancel, message)
		}
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
}

func phoneError(raw string) error {
	code := failureCode(raw)
	message := strings.TrimSpace(raw)
	switch code {
	case "-1":
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	case "-2":
		return sms.NewProviderError("AUTH_ERROR", message)
	case "-8", "-9":
		return sms.NewProviderError(sms.ErrBalance, message)
	default:
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
}

func codeError(raw string) error {
	code := failureCode(raw)
	message := strings.TrimSpace(raw)
	switch code {
	case "-1":
		return sms.NewProviderError("AUTH_ERROR", message)
	case "-2", "-4", "-5":
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	default:
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
}

func releaseError(raw string) error {
	code := failureCode(raw)
	message := strings.TrimSpace(raw)
	if seconds, err := strconv.Atoi(code); err == nil && seconds > 0 {
		return sms.NewProviderError(sms.ErrCannotCancel, message)
	}
	switch code {
	case "-1":
		return sms.NewProviderError("AUTH_ERROR", message)
	case "-2", "-3", "-6":
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	case "-4", "-5":
		return sms.NewProviderError(sms.ErrCannotCancel, message)
	default:
		return sms.NewProviderError(sms.ErrProviderRejected, message)
	}
}

func failureCode(raw string) string {
	parts := strings.Split(raw, "|")
	if len(parts) >= 2 && parts[0] == "0" {
		return parts[1]
	}
	return parts[0]
}

func isOutOfStock(err error) bool {
	var providerErr *sms.ProviderError
	return errors.As(err, &providerErr) && providerErr.Code == sms.ErrOutOfStock
}

func buildPhone(countryCode, national string) string {
	if countryCode != "" && national != "" && !strings.HasPrefix(national, "+") {
		return "+" + countryCode + national
	}
	return national
}

func parseItems(raw string) ([]itemRow, error) {
	var rows []struct {
		ItemID       int             `json:"Item_ID"`
		ItemName     string          `json:"Item_Name"`
		ItemUPrice   json.RawMessage `json:"Item_UPrice"`
		CountryID    int             `json:"Country_ID"`
		CountryTitle string          `json:"Country_Title"`
		Stock        int             `json:"Stock"`
		StockCount   int             `json:"Stock_Count"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	items := make([]itemRow, 0, len(rows))
	for _, row := range rows {
		price := strings.Trim(string(row.ItemUPrice), `"`)
		stock := row.Stock
		if stock == 0 {
			stock = row.StockCount
		}
		items = append(items, itemRow{
			ID:        row.ItemID,
			Name:      row.ItemName,
			Country:   row.CountryTitle,
			CountryID: row.CountryID,
			Price:     price,
			Stock:     stock,
		})
	}
	return items, nil
}
