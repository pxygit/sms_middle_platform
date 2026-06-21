package sms62us

import (
	"bytes"
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

const (
	providerCode             = "62-us"
	accountPasswordBaseURL   = "https://www.62-us.com"
	accountTokenRefreshSkew  = 5 * time.Minute
	accountTokenFallbackLife = 30 * 24 * time.Hour
)

type Client struct {
	apiKey                string
	baseURL               string
	authMode              string
	account               string
	password              string
	accountToken          string
	accountTokenExpiresAt time.Time
	httpClient            *http.Client
	logger                func(model.SupplierRequestLog)
	mu                    sync.RWMutex
}

type apiEnvelope struct {
	Code      json.RawMessage `json:"code"`
	Msg       string          `json:"msg"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Time      json.RawMessage `json:"time"`
	Data      json.RawMessage `json:"data"`
}

type goodsRow struct {
	GoodsID     string
	PingtaiID   string
	PingtaiText string
	Country     string
	CountryText string
	EndDay      string
	Price       string
	Stock       int
	Raw         json.RawMessage
}

type serviceConfigMetadata struct {
	EndDay        string `json:"endDay"`
	ValidityType  string `json:"validityType"`
	ValidityLabel string `json:"validityLabel"`
	ValidityStock int    `json:"validityStock"`
}

func New(apiKey, baseURL string, timeout time.Duration, logger func(model.SupplierRequestLog)) *Client {
	return &Client{
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		authMode:   "openapi_token",
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}
}

func (c *Client) Name() string { return providerCode }

func (c *Client) Configure(apiKey, baseURL string) {
	c.ConfigureAdvanced(apiKey, baseURL, "")
}

func (c *Client) ConfigureAdvanced(apiKey, baseURL, metadataToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(apiKey) != "" {
		c.apiKey = strings.TrimSpace(apiKey)
	}
	if strings.TrimSpace(baseURL) != "" {
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	credential := parseCredential(metadataToken)
	if credential.AuthMode != "" {
		c.authMode = credential.AuthMode
	}
	if credential.Account != "" {
		if c.account != credential.Account {
			c.accountToken = ""
			c.accountTokenExpiresAt = time.Time{}
		}
		c.account = credential.Account
	}
	if credential.Password != "" {
		if c.password != credential.Password {
			c.accountToken = ""
			c.accountTokenExpiresAt = time.Time{}
		}
		c.password = credential.Password
	}
	if c.authMode == "" {
		if c.apiKey != "" {
			c.authMode = "openapi_token"
		} else {
			c.authMode = "account_password"
		}
	}
}

func (c *Client) ProviderKind() sms.ProviderKind {
	return sms.ProviderKind{
		Kind:               "long_lived",
		ManualCheck:        true,
		MessageURLTemplate: "https://api.62-us.com/api/v1/msg?token={token}",
	}
}

func (c *Client) GetMessageURL(token string) string {
	if c.useAccountPassword() {
		return accountMessageURL(token)
	}
	_, _, baseURL, _ := c.config()
	if strings.TrimSpace(token) == "" {
		return ""
	}
	return baseURL + "/api/v1/msg?token=" + url.QueryEscape(strings.TrimSpace(token))
}

func (c *Client) GetBalance(ctx context.Context) (*sms.ProviderBalance, error) {
	if c.useAccountPassword() {
		return c.accountBalance(ctx)
	}
	var out apiEnvelope
	raw, status, err := c.request(ctx, http.MethodGet, "/api/v1/info", nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if !successCode(out.Code) {
		return nil, providerError(statusCode(out.Code), firstNonEmpty(out.Msg, out.Message, string(raw)))
	}
	balance := extractBalance(out.Data)
	if balance == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "62-us balance field not found in account info response")
	}
	return &sms.ProviderBalance{Balance: balance}, nil
}

func (c *Client) RequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	if c.useAccountPassword() {
		return c.accountRequestNumber(ctx, input)
	}
	endDay := selectedEndDay(input.Metadata)
	if endDay == "" {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "62-us number validity is required")
	}
	query := url.Values{
		"goods_id": {fmt.Sprintf("%s-%s-%s", input.ServiceID, input.CountryID, endDay)},
		"num":      {"1"},
	}
	var out apiEnvelope
	raw, status, err := c.request(ctx, http.MethodPost, "/api/v1/get", query, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if !successCode(out.Code) {
		return nil, providerError(statusCode(out.Code), firstNonEmpty(out.Msg, out.Message, string(raw)))
	}
	orderID := extractOrderID(out.Data)
	token, phone := extractFirstToken(out.Data)
	tokenRaw := json.RawMessage(nil)
	if token == "" || phone == "" {
		if orderID == "" {
			return nil, sms.NewProviderError(sms.ErrProviderRejected, "62-us order id not found in get response")
		}
		var err error
		token, phone, tokenRaw, err = c.firstOrderToken(ctx, orderID)
		if err != nil {
			return nil, err
		}
	}
	if token == "" || phone == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "62-us order token or phone number is missing")
	}
	if orderID == "" {
		orderID = token
	}
	cost := input.MaxPrice
	if price, err := c.GetPrice(ctx, sms.ProviderPriceInput{CountryID: input.CountryID, ServiceID: input.ServiceID, PoolID: endDay}); err == nil {
		if parsed := parseFloat(firstNonEmpty(price.Price, price.LowPrice, price.HighPrice)); parsed > 0 {
			cost = parsed
		}
	}
	combinedRaw := raw
	if len(tokenRaw) > 0 {
		combinedRaw = mustMarshal(map[string]json.RawMessage{"order": raw, "tokens": tokenRaw})
	}
	return &sms.RequestNumberResult{
		SupplierOrderID:     orderID,
		SupplierToken:       token,
		PhoneNumber:         normalizePhone(phone),
		PhoneCountryCode:    phoneCountryCode(phone),
		PhoneNationalNumber: phoneNationalNumber(phone),
		Country:             input.CountryID,
		Service:             input.ServiceID,
		Cost:                cost,
		Raw:                 combinedRaw,
	}, nil
}

func (c *Client) CheckSMS(ctx context.Context, input sms.CheckSMSInput) (*sms.SMSResult, error) {
	return &sms.SMSResult{Status: model.OrderActive, SupplierStatus: "manual_check"}, nil
}

func (c *Client) CheckManualSMS(ctx context.Context, input sms.ManualSMSInput) (*sms.SMSResult, error) {
	if c.useAccountPassword() {
		return c.accountCheckManualSMS(ctx, input)
	}
	token := strings.TrimSpace(input.SupplierToken)
	if token == "" {
		return nil, sms.NewProviderError(sms.ErrOrderNotFound, "62-us order token is missing")
	}
	var out apiEnvelope
	raw, status, err := c.request(ctx, http.MethodGet, "/api/v1/msg", url.Values{"token": {token}, "limit": {"5"}}, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	code := statusCode(out.Code)
	if !successCode(out.Code) {
		if code == "40411" || code == "42206" {
			return nil, providerError(code, firstNonEmpty(out.Msg, out.Message, string(raw)))
		}
		return nil, providerError(code, firstNonEmpty(out.Msg, out.Message, string(raw)))
	}
	content := extractMessage(out.Data)
	if strings.TrimSpace(content) == "" {
		return &sms.SMSResult{Status: model.OrderActive, SupplierStatus: firstNonEmpty(code, "waiting_sms"), Raw: raw}, nil
	}
	return &sms.SMSResult{
		Status:           model.OrderSMSReceived,
		SupplierStatus:   firstNonEmpty(code, "1"),
		VerificationCode: extractCode(content),
		SMSContent:       content,
		Raw:              raw,
	}, nil
}

func (c *Client) CancelNumber(ctx context.Context, input sms.CancelNumberInput) (*sms.CancelResult, error) {
	return nil, sms.NewProviderError(sms.ErrCannotCancel, "62-us long-lived numbers do not support cancellation")
}

func (c *Client) GetCatalog(ctx context.Context) (*sms.ProviderCatalog, error) {
	if c.useAccountPassword() {
		return c.accountCatalog(ctx)
	}
	goods, err := c.goods(ctx, "", "")
	if err != nil {
		return nil, err
	}
	countriesByID := map[string]sms.ProviderCountry{}
	servicesByKey := map[string]sms.ProviderService{}
	for _, item := range goods {
		if item.Country != "" {
			countriesByID[item.Country] = sms.ProviderCountry{
				Code:      item.Country,
				Name:      firstNonEmpty(item.CountryText, countryName(item.Country), item.Country),
				ShortName: item.Country,
				DialCode:  dialCode(item.Country),
			}
		}
		if item.PingtaiID != "" && item.Country != "" {
			key := item.Country + ":" + item.PingtaiID
			servicesByKey[key] = sms.ProviderService{
				Code:        item.PingtaiID,
				Name:        firstNonEmpty(item.PingtaiText, item.PingtaiID),
				CountryCode: item.Country,
				CountryName: firstNonEmpty(item.CountryText, countryName(item.Country), item.Country),
				Price:       item.Price,
				Stock:       item.Stock,
			}
		}
	}
	countries := make([]sms.ProviderCountry, 0, len(countriesByID))
	for _, country := range countriesByID {
		countries = append(countries, country)
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].Name < countries[j].Name })
	services := make([]sms.ProviderService, 0, len(servicesByKey))
	for _, service := range servicesByKey {
		services = append(services, service)
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].CountryCode == services[j].CountryCode {
			return services[i].Name < services[j].Name
		}
		return services[i].CountryCode < services[j].CountryCode
	})
	return &sms.ProviderCatalog{Countries: countries, Services: services}, nil
}

func (c *Client) GetCountries(ctx context.Context) ([]sms.ProviderCountry, error) {
	catalog, err := c.GetCatalog(ctx)
	if err != nil {
		return nil, err
	}
	return catalog.Countries, nil
}

func (c *Client) GetServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	if c.useAccountPassword() {
		return c.accountServices(ctx, countryID)
	}
	goods, err := c.goods(ctx, "", countryID)
	if err != nil {
		return nil, err
	}
	seen := map[string]sms.ProviderService{}
	for _, item := range goods {
		if item.PingtaiID == "" {
			continue
		}
		seen[item.PingtaiID] = sms.ProviderService{
			Code:        item.PingtaiID,
			Name:        firstNonEmpty(item.PingtaiText, item.PingtaiID),
			CountryCode: firstNonEmpty(item.Country, countryID),
			CountryName: firstNonEmpty(item.CountryText, countryName(firstNonEmpty(item.Country, countryID)), firstNonEmpty(item.Country, countryID)),
			Price:       item.Price,
			Stock:       item.Stock,
		}
	}
	services := make([]sms.ProviderService, 0, len(seen))
	for _, service := range seen {
		services = append(services, service)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services, nil
}

func (c *Client) GetValidityOptions(ctx context.Context, input sms.ValidityOptionsInput) ([]sms.ProviderValidityOption, error) {
	if c.useAccountPassword() {
		return c.accountValidityOptions(ctx, input)
	}
	goods, err := c.goodsDetail(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	if len(goods) == 0 {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "62-us goods detail not found")
	}
	optionsByDay := map[string]sms.ProviderValidityOption{}
	for _, item := range goods {
		day := firstNonEmpty(item.EndDay, endDayFromGoodsID(item.GoodsID))
		if day == "" {
			continue
		}
		stock := item.Stock
		option := sms.ProviderValidityOption{
			Value:   day,
			Label:   day + " days",
			MinDays: parseInt(day),
			MaxDays: parseInt(day),
			Stock:   stock,
			Raw:     item.Raw,
		}
		if existing, ok := optionsByDay[day]; !ok || option.Stock > existing.Stock {
			optionsByDay[day] = option
		}
	}
	options := make([]sms.ProviderValidityOption, 0, len(optionsByDay))
	for _, option := range optionsByDay {
		options = append(options, option)
	}
	sort.Slice(options, func(i, j int) bool { return options[i].MinDays < options[j].MinDays })
	return options, nil
}

func (c *Client) GetPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	if c.useAccountPassword() {
		return c.accountPrice(ctx, input)
	}
	goods, err := c.goodsDetail(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	var selected *goodsRow
	for index, item := range goods {
		if input.PoolID != "" {
			day := firstNonEmpty(item.EndDay, endDayFromGoodsID(item.GoodsID))
			if day != input.PoolID {
				continue
			}
		}
		if item.Price == "" {
			continue
		}
		if selected == nil || priceLess(item.Price, selected.Price) {
			selected = &goods[index]
		}
	}
	if selected == nil {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "62-us price not found")
	}
	return &sms.ProviderPrice{LowPrice: selected.Price, HighPrice: selected.Price, Price: selected.Price, Raw: selected.Raw}, nil
}

func (c *Client) GetStock(ctx context.Context, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	if c.useAccountPassword() {
		return c.accountStock(ctx, input)
	}
	goods, err := c.goodsDetail(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	total := 0
	for _, item := range goods {
		if input.PoolID != "" {
			day := firstNonEmpty(item.EndDay, endDayFromGoodsID(item.GoodsID))
			if day != input.PoolID {
				continue
			}
		}
		total += item.Stock
	}
	return &sms.ProviderStock{Amount: total, Raw: mustMarshal(goods)}, nil
}

func (c *Client) useAccountPassword() bool {
	mode, _, _, _ := c.config()
	return mode == "account_password"
}

func accountPasswordPendingError() error {
	return sms.NewProviderError("AUTH_MODE_PENDING", "62-us account/password API mode is configured but this operation is not connected yet")
}

type accountLoginResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Time string `json:"time"`
	Data struct {
		Token    string `json:"token"`
		UserInfo struct {
			Token     string `json:"token"`
			ExpireAt  int64  `json:"expiretime"`
			ExpiresIn int64  `json:"expires_in"`
		} `json:"userinfo"`
	} `json:"data"`
}

type accountBalanceResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Time string `json:"time"`
	Data struct {
		Balance string `json:"balance"`
	} `json:"data"`
}

type accountBuyResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Time string `json:"time"`
	Data struct {
		OrderID interface{} `json:"order_id"`
	} `json:"data"`
}

type accountDownloadURLResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Time string `json:"time"`
	Data struct {
		Download string `json:"download"`
	} `json:"data"`
}

type accountServiceRow struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Price string `json:"price"`
	Stock string `json:"stock"`
}

type accountCountryRow struct {
	Label string          `json:"label"`
	Value json.RawMessage `json:"value"`
}

type accountPriceStoreRow struct {
	ID         int             `json:"id"`
	Country    string          `json:"country"`
	EndDay     string          `json:"endday"`
	Price      string          `json:"price"`
	PingtaiID  int             `json:"pingtai_id"`
	Store      int             `json:"store"`
	EndDayName string          `json:"enddayname"`
	Raw        json.RawMessage `json:"-"`
}

type accountDataEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Time string          `json:"time"`
	Data json.RawMessage `json:"data"`
}

type accountJSONValues map[string]interface{}

func (c *Client) accountRequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	selected, err := c.accountSelectEndDay(ctx, input)
	if err != nil {
		return nil, err
	}
	orderID, buyRaw, err := c.accountBuyNumber(ctx, input.CountryID, input.ServiceID, selected.EndDay)
	if err != nil {
		return nil, err
	}
	downloadPath, downloadRaw, err := c.accountDownloadURL(ctx, orderID)
	if err != nil {
		return nil, err
	}
	fileRaw, err := c.accountDownloadNumberFile(ctx, downloadPath)
	if err != nil {
		return nil, err
	}
	phone, messageURL, err := parseAccountNumberFile(fileRaw)
	if err != nil {
		return nil, err
	}
	cost := input.MaxPrice
	if parsed := parseFloat(selected.Price); parsed > 0 {
		cost = parsed
	}
	return &sms.RequestNumberResult{
		SupplierOrderID:     orderID,
		SupplierToken:       messageURL,
		PhoneNumber:         normalizePhone(phone),
		PhoneCountryCode:    phoneCountryCode(phone),
		PhoneNationalNumber: phoneNationalNumber(phone),
		Country:             input.CountryID,
		Service:             input.ServiceID,
		Cost:                cost,
		Raw: mustMarshal(map[string]interface{}{
			"buy":        json.RawMessage(buyRaw),
			"download":   json.RawMessage(downloadRaw),
			"endday":     selected.EndDay,
			"messageUrl": messageURL,
		}),
	}, nil
}

func (c *Client) accountCheckManualSMS(ctx context.Context, input sms.ManualSMSInput) (*sms.SMSResult, error) {
	messageURL := accountMessageURL(input.SupplierToken)
	if messageURL == "" {
		return nil, sms.NewProviderError(sms.ErrOrderNotFound, "62-us message URL is missing")
	}
	raw, status, err := c.accountExternalGet(ctx, messageURL)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	content := extractMessage(raw)
	if content == "" && jsonRawHasEmptyData(raw) {
		return &sms.SMSResult{Status: model.OrderActive, SupplierStatus: "waiting_sms", Raw: raw}, nil
	}
	if content == "" {
		content = strings.Trim(strings.TrimSpace(string(raw)), `"`)
	}
	if isNoSMSContent(content) {
		return &sms.SMSResult{Status: model.OrderActive, SupplierStatus: "waiting_sms", Raw: raw}, nil
	}
	return &sms.SMSResult{
		Status:           model.OrderSMSReceived,
		SupplierStatus:   "received",
		VerificationCode: extractCode(content),
		SMSContent:       content,
		Raw:              raw,
	}, nil
}

func (c *Client) accountBalance(ctx context.Context) (*sms.ProviderBalance, error) {
	var out accountBalanceResponse
	raw, status, err := c.accountRequest(ctx, http.MethodGet, "/api/user/fetchBalance", nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if out.Code != 1 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Msg, string(raw)))
	}
	if strings.TrimSpace(out.Data.Balance) == "" {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, "62-us balance field not found")
	}
	return &sms.ProviderBalance{Balance: strings.TrimSpace(out.Data.Balance)}, nil
}

func (c *Client) accountCatalog(ctx context.Context) (*sms.ProviderCatalog, error) {
	countries, err := c.accountCountries(ctx)
	if err != nil {
		return nil, err
	}
	services, err := c.accountServiceRows(ctx)
	if err != nil {
		return nil, err
	}
	catalogServices := make([]sms.ProviderService, 0, len(countries)*len(services))
	for _, country := range countries {
		countryID := firstNonEmpty(country.Code, country.ShortName)
		for _, service := range services {
			catalogServices = append(catalogServices, sms.ProviderService{
				ID:          service.ID,
				Code:        strconv.Itoa(service.ID),
				Name:        service.Name,
				CountryCode: countryID,
				CountryName: country.Name,
				Price:       service.Price,
				Stock:       stockTextToAmount(service.Stock),
			})
		}
	}
	return &sms.ProviderCatalog{Countries: countries, Services: catalogServices}, nil
}

func (c *Client) accountServices(ctx context.Context, countryID string) ([]sms.ProviderService, error) {
	countries, _ := c.accountCountries(ctx)
	countryName := ""
	for _, country := range countries {
		if firstNonEmpty(country.Code, country.ShortName) == strings.TrimSpace(countryID) {
			countryName = country.Name
			break
		}
	}
	rows, err := c.accountServiceRows(ctx)
	if err != nil {
		return nil, err
	}
	services := make([]sms.ProviderService, 0, len(rows))
	for _, row := range rows {
		services = append(services, sms.ProviderService{
			ID:          row.ID,
			Code:        strconv.Itoa(row.ID),
			Name:        row.Name,
			CountryCode: strings.TrimSpace(countryID),
			CountryName: countryName,
			Price:       row.Price,
			Stock:       stockTextToAmount(row.Stock),
		})
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services, nil
}

func (c *Client) accountValidityOptions(ctx context.Context, input sms.ValidityOptionsInput) ([]sms.ProviderValidityOption, error) {
	rows, err := c.accountPriceStore(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	options := make([]sms.ProviderValidityOption, 0, len(rows))
	for _, row := range rows {
		if row.EndDay == "" {
			continue
		}
		minDays, maxDays := validityRange(row.EndDay, row.EndDayName)
		options = append(options, sms.ProviderValidityOption{
			Value:   row.EndDay,
			Label:   firstNonEmpty(row.EndDayName, row.EndDay),
			MinDays: minDays,
			MaxDays: maxDays,
			Stock:   row.Store,
			Raw:     row.Raw,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		return parseInt(options[i].Value) < parseInt(options[j].Value)
	})
	return options, nil
}

func (c *Client) accountPrice(ctx context.Context, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	row, err := c.accountSelectedPriceStore(ctx, input.CountryID, input.ServiceID, input.PoolID)
	if err != nil {
		return nil, err
	}
	return &sms.ProviderPrice{
		LowPrice:  row.Price,
		HighPrice: row.Price,
		Price:     row.Price,
		Raw:       row.Raw,
	}, nil
}

func (c *Client) accountStock(ctx context.Context, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	row, err := c.accountSelectedPriceStore(ctx, input.CountryID, input.ServiceID, input.PoolID)
	if err != nil {
		return nil, err
	}
	return &sms.ProviderStock{Amount: row.Store, Raw: row.Raw}, nil
}

func (c *Client) accountSelectEndDay(ctx context.Context, input sms.RequestNumberInput) (*accountPriceStoreRow, error) {
	rows, err := c.accountPriceStore(ctx, input.CountryID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	preferred := selectedEndDay(input.Metadata)
	if preferred == "" {
		preferred = strings.TrimSpace(input.PoolID)
	}
	if preferred != "" {
		for index := range rows {
			if rows[index].EndDay == preferred && rows[index].Store > 0 {
				return &rows[index], nil
			}
		}
	}
	for _, endDay := range []string{"4", "3", "2", "1"} {
		for index := range rows {
			if rows[index].EndDay == endDay && rows[index].Store > 0 {
				return &rows[index], nil
			}
		}
	}
	return nil, sms.NewProviderError(sms.ErrOutOfStock, "62-us number stock is empty")
}

func (c *Client) accountBuyNumber(ctx context.Context, countryID, serviceID, endDay string) (string, json.RawMessage, error) {
	body := accountJSONValues{
		"country_id":   jsonNumberOrString(countryID),
		"pingtai_id":   jsonNumberOrString(serviceID),
		"endday":       jsonNumberOrString(endDay),
		"num":          1,
		"sipprice_id":  0,
		"first_number": []string{},
	}
	var out accountBuyResponse
	raw, status, err := c.accountRequestJSON(ctx, http.MethodPost, "/api/functions/getBuyCount", nil, body, &out)
	if err != nil {
		return "", raw, err
	}
	if status >= 400 {
		return "", raw, httpError(status, raw)
	}
	if out.Code != 1 {
		return "", raw, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Msg, string(raw)))
	}
	orderID := toString(out.Data.OrderID)
	if orderID == "" {
		return "", raw, sms.NewProviderError(sms.ErrProviderRejected, "62-us order id not found")
	}
	return orderID, raw, nil
}

func (c *Client) accountDownloadURL(ctx context.Context, orderID string) (string, json.RawMessage, error) {
	var out accountDownloadURLResponse
	raw, status, err := c.accountRequest(ctx, http.MethodGet, "/api/functions/getDownloadUrl", url.Values{"order_id": {orderID}}, &out)
	if err != nil {
		return "", raw, err
	}
	if status >= 400 {
		return "", raw, httpError(status, raw)
	}
	if out.Code != 1 {
		return "", raw, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Msg, string(raw)))
	}
	download := strings.TrimSpace(out.Data.Download)
	if download == "" {
		return "", raw, sms.NewProviderError(sms.ErrProviderRejected, "62-us download URL not found")
	}
	return normalizeAccountDownloadURL(download), raw, nil
}

func (c *Client) accountDownloadNumberFile(ctx context.Context, downloadPath string) (json.RawMessage, error) {
	raw, status, err := c.accountDownload(ctx, downloadPath)
	if err != nil {
		return raw, err
	}
	if status >= 400 {
		return raw, httpError(status, raw)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return raw, sms.NewProviderError(sms.ErrProviderRejected, "62-us number file is empty")
	}
	if looksLikeJSONError(raw) {
		return raw, providerError("", string(raw))
	}
	return raw, nil
}

func (c *Client) accountSelectedPriceStore(ctx context.Context, countryID, serviceID, endDay string) (*accountPriceStoreRow, error) {
	rows, err := c.accountPriceStore(ctx, countryID, serviceID)
	if err != nil {
		return nil, err
	}
	var selected *accountPriceStoreRow
	for index := range rows {
		row := &rows[index]
		if endDay != "" && row.EndDay != endDay {
			continue
		}
		if selected == nil || priceLess(row.Price, selected.Price) {
			selected = row
		}
	}
	if selected == nil {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "62-us price store not found")
	}
	return selected, nil
}

func (c *Client) accountCountries(ctx context.Context) ([]sms.ProviderCountry, error) {
	var out accountDataEnvelope
	raw, status, err := c.accountRequest(ctx, http.MethodGet, "/api/functions/getcountry", nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if out.Code != 1 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Msg, string(raw)))
	}
	var rows []accountCountryRow
	if err := json.Unmarshal(out.Data, &rows); err != nil {
		return nil, err
	}
	countries := make([]sms.ProviderCountry, 0, len(rows))
	for _, row := range rows {
		value := jsonValueString(row.Value)
		if value == "" {
			continue
		}
		countries = append(countries, sms.ProviderCountry{
			Code:      value,
			Name:      firstNonEmpty(row.Label, countryName(value), value),
			ShortName: value,
			DialCode:  dialCode(value),
		})
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].Name < countries[j].Name })
	return countries, nil
}

func (c *Client) accountServiceRows(ctx context.Context) ([]accountServiceRow, error) {
	var out accountDataEnvelope
	raw, status, err := c.accountRequest(ctx, http.MethodGet, "/api/functions/pingtai", nil, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if out.Code != 1 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Msg, string(raw)))
	}
	var rows []accountServiceRow
	if err := json.Unmarshal(out.Data, &rows); err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows, nil
}

func (c *Client) accountPriceStore(ctx context.Context, countryID, serviceID string) ([]accountPriceStoreRow, error) {
	if strings.TrimSpace(countryID) == "" || strings.TrimSpace(serviceID) == "" {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "62-us country and service are required")
	}
	body := accountJSONValues{
		"pingtai_id":      jsonNumberOrString(serviceID),
		"country":         strings.TrimSpace(countryID),
		"with_store_list": 1,
	}
	var out accountDataEnvelope
	raw, status, err := c.accountRequestJSON(ctx, http.MethodPost, "/api/functions/getPriceStore", nil, body, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if out.Code != 1 {
		return nil, providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Msg, string(raw)))
	}
	return parseAccountPriceStoreRows(out.Data)
}

func (c *Client) accountRequest(ctx context.Context, method, path string, query url.Values, out interface{}) (json.RawMessage, int, error) {
	token, err := c.accountLoginToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	urlQuery := url.Values{"token": {token}}
	body := url.Values{}
	if query != nil {
		for key, values := range query {
			for _, value := range values {
				if method == http.MethodGet {
					urlQuery.Add(key, value)
				} else {
					body.Add(key, value)
				}
			}
		}
	}
	return c.accountRawRequest(ctx, method, path, urlQuery, valuesToJSON(body), out)
}

func (c *Client) accountRequestJSON(ctx context.Context, method, path string, query url.Values, body accountJSONValues, out interface{}) (json.RawMessage, int, error) {
	token, err := c.accountLoginToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("token", token)
	return c.accountRawRequest(ctx, method, path, query, body, out)
}

func (c *Client) accountLoginToken(ctx context.Context) (string, error) {
	mode, _, _, credential := c.config()
	if mode != "account_password" {
		return "", sms.NewProviderError("AUTH_ERROR", "62-us account/password mode is not enabled")
	}
	now := time.Now()
	c.mu.RLock()
	if c.accountToken != "" && now.Add(accountTokenRefreshSkew).Before(c.accountTokenExpiresAt) {
		token := c.accountToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()
	if credential.Account == "" || credential.Password == "" {
		return "", sms.NewProviderError("AUTH_ERROR", "62-us account and password are not configured")
	}
	body := url.Values{
		"account":  {credential.Account},
		"password": {credential.Password},
	}
	var out accountLoginResponse
	raw, status, err := c.accountRawRequest(ctx, http.MethodPost, "/api/user/login", nil, valuesToJSON(body), &out)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", httpError(status, raw)
	}
	if out.Code != 1 {
		return "", providerError(strconv.Itoa(out.Code), firstNonEmpty(out.Msg, string(raw)))
	}
	token := firstNonEmpty(out.Data.Token, out.Data.UserInfo.Token)
	if token == "" {
		return "", sms.NewProviderError("AUTH_ERROR", "62-us login token not found")
	}
	expiresAt := time.Now().Add(accountTokenFallbackLife)
	if out.Data.UserInfo.ExpireAt > 0 {
		expiresAt = time.Unix(out.Data.UserInfo.ExpireAt, 0)
	} else if out.Data.UserInfo.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(out.Data.UserInfo.ExpiresIn) * time.Second)
	}
	c.mu.Lock()
	c.accountToken = token
	c.accountTokenExpiresAt = expiresAt
	c.mu.Unlock()
	return token, nil
}

func (c *Client) accountRawRequest(ctx context.Context, method, path string, query url.Values, bodyValues accountJSONValues, out interface{}) (json.RawMessage, int, error) {
	_, _, baseURL, _ := c.config()
	if query == nil {
		query = url.Values{}
	}
	target := baseURL + path
	var body io.Reader
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	if method != http.MethodGet {
		rawBody, _ := json.Marshal(bodyValues)
		body = bytes.NewReader(rawBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log(path, mergeLogValues(query, jsonValuesToLogValues(bodyValues)), 0, false, err.Error(), "", time.Since(start), nil)
		return nil, 0, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log(path, mergeLogValues(query, jsonValuesToLogValues(bodyValues)), resp.StatusCode, false, err.Error(), "", time.Since(start), bodyBytes)
		return bodyBytes, resp.StatusCode, err
	}
	if out != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			c.log(path, mergeLogValues(query, jsonValuesToLogValues(bodyValues)), resp.StatusCode, false, err.Error(), "", time.Since(start), bodyBytes)
			return bodyBytes, resp.StatusCode, err
		}
	}
	c.log(path, mergeLogValues(query, jsonValuesToLogValues(bodyValues)), resp.StatusCode, resp.StatusCode < 400, "", "", time.Since(start), bodyBytes)
	return bodyBytes, resp.StatusCode, nil
}

func (c *Client) accountDownload(ctx context.Context, downloadPath string) (json.RawMessage, int, error) {
	_, _, baseURL, _ := c.config()
	target := strings.TrimSpace(downloadPath)
	if target == "" {
		return nil, 0, sms.NewProviderError(sms.ErrProviderRejected, "62-us download URL is empty")
	}
	if strings.HasPrefix(target, "/") {
		target = baseURL + target
	} else if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(target, "/")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "text/plain, application/octet-stream, */*")
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log("/api/functions/download", url.Values{"url": {target}}, 0, false, err.Error(), "", time.Since(start), nil)
		return nil, 0, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log("/api/functions/download", url.Values{"url": {target}}, resp.StatusCode, false, err.Error(), "", time.Since(start), bodyBytes)
		return bodyBytes, resp.StatusCode, err
	}
	c.log("/api/functions/download", url.Values{"url": {target}}, resp.StatusCode, resp.StatusCode < 400, "", "", time.Since(start), bodyBytes)
	return bodyBytes, resp.StatusCode, nil
}

func (c *Client) accountExternalGet(ctx context.Context, target string) (json.RawMessage, int, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, 0, sms.NewProviderError(sms.ErrOrderNotFound, "62-us message URL is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log("/api/get_sms", url.Values{"url": {target}}, 0, false, err.Error(), "", time.Since(start), nil)
		return nil, 0, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log("/api/get_sms", url.Values{"url": {target}}, resp.StatusCode, false, err.Error(), "", time.Since(start), bodyBytes)
		return bodyBytes, resp.StatusCode, err
	}
	c.log("/api/get_sms", url.Values{"url": {target}}, resp.StatusCode, resp.StatusCode < 400, "", "", time.Since(start), bodyBytes)
	return bodyBytes, resp.StatusCode, nil
}

func (c *Client) goods(ctx context.Context, serviceID, countryID string) ([]goodsRow, error) {
	query := url.Values{}
	if serviceID != "" {
		query.Set("pingtai_id", serviceID)
	}
	if countryID != "" {
		query.Set("country", countryID)
	}
	var out apiEnvelope
	raw, status, err := c.request(ctx, http.MethodGet, "/api/v1/goods", query, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if !successCode(out.Code) {
		return nil, providerError(statusCode(out.Code), firstNonEmpty(out.Msg, out.Message, string(raw)))
	}
	return parseGoodsRows(out.Data)
}

func (c *Client) goodsDetail(ctx context.Context, countryID, serviceID string) ([]goodsRow, error) {
	if countryID == "" || serviceID == "" {
		return nil, sms.NewProviderError(sms.ErrPriceNotFound, "62-us country and service are required")
	}
	query := url.Values{"goods_id": {fmt.Sprintf("%s-%s", serviceID, countryID)}}
	var out apiEnvelope
	raw, status, err := c.request(ctx, http.MethodGet, "/api/v1/goods/detail", query, &out)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, httpError(status, raw)
	}
	if !successCode(out.Code) {
		return nil, providerError(statusCode(out.Code), firstNonEmpty(out.Msg, out.Message, string(raw)))
	}
	return parseGoodsRows(out.Data)
}

func (c *Client) firstOrderToken(ctx context.Context, orderID string) (string, string, json.RawMessage, error) {
	var out apiEnvelope
	raw, status, err := c.request(ctx, http.MethodGet, "/api/v1/order/tokens", url.Values{"order_id": {orderID}}, &out)
	if err != nil {
		return "", "", raw, err
	}
	if status >= 400 {
		return "", "", raw, httpError(status, raw)
	}
	if !successCode(out.Code) {
		return "", "", raw, providerError(statusCode(out.Code), firstNonEmpty(out.Msg, out.Message, string(raw)))
	}
	token, phone := extractFirstToken(out.Data)
	return token, phone, raw, nil
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, out interface{}) (json.RawMessage, int, error) {
	_, apiKey, baseURL, _ := c.config()
	if strings.TrimSpace(apiKey) == "" {
		return nil, 0, sms.NewProviderError("AUTH_ERROR", "62-us api key is not configured")
	}
	if query == nil {
		query = url.Values{}
	}
	target := baseURL + path
	var body io.Reader
	if method == http.MethodGet {
		if len(query) > 0 {
			target += "?" + query.Encode()
		}
	} else {
		body = bytes.NewBufferString(query.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-API-Key", apiKey)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log(path, query, 0, false, err.Error(), "", time.Since(start), nil)
		return nil, 0, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log(path, query, resp.StatusCode, false, err.Error(), "", time.Since(start), bodyBytes)
		return bodyBytes, resp.StatusCode, err
	}
	if out != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			c.log(path, query, resp.StatusCode, false, err.Error(), "", time.Since(start), bodyBytes)
			return bodyBytes, resp.StatusCode, err
		}
	}
	requestID := ""
	if envelope, ok := out.(*apiEnvelope); ok {
		requestID = envelope.RequestID
	}
	c.log(path, query, resp.StatusCode, resp.StatusCode < 400, "", requestID, time.Since(start), bodyBytes)
	return bodyBytes, resp.StatusCode, nil
}

func (c *Client) config() (string, string, string, accountCredential) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	mode := c.authMode
	if mode == "" {
		if c.apiKey != "" {
			mode = "openapi_token"
		} else {
			mode = "account_password"
		}
	}
	baseURL := c.baseURL
	if mode == "account_password" {
		baseURL = accountPasswordBaseURL
	} else if baseURL == "" {
		baseURL = "https://api.62-us.com"
	}
	return mode, c.apiKey, strings.TrimRight(baseURL, "/"), accountCredential{Account: c.account, Password: c.password}
}

func (c *Client) log(action string, query url.Values, status int, success bool, message string, requestID string, latency time.Duration, body []byte) {
	if c.logger == nil {
		return
	}
	safe := url.Values{}
	for key, values := range query {
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, "token") || strings.Contains(keyLower, "password") || strings.Contains(keyLower, "account") || strings.EqualFold(key, "api_key") || keyLower == "url" {
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
		RequestID:    requestID,
		HTTPStatus:   status,
		Success:      success,
		ErrorMessage: message,
		LatencyMS:    latency.Milliseconds(),
		RequestBody:  safe.Encode(),
		ResponseBody: sanitizeLogBody(action, body),
	})
}

type accountCredential struct {
	AuthMode string `json:"authMode"`
	Account  string `json:"account"`
	Password string `json:"password"`
}

func parseCredential(value string) accountCredential {
	value = strings.TrimSpace(value)
	if value == "" {
		return accountCredential{}
	}
	var credential accountCredential
	if err := json.Unmarshal([]byte(value), &credential); err == nil {
		credential.AuthMode = normalizeAuthMode(credential.AuthMode)
		credential.Account = strings.TrimSpace(credential.Account)
		credential.Password = strings.TrimSpace(credential.Password)
		return credential
	}
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		return accountCredential{AuthMode: "account_password", Account: strings.TrimSpace(parts[0]), Password: strings.TrimSpace(parts[1])}
	}
	return accountCredential{AuthMode: "account_password", Account: value}
}

func normalizeAuthMode(value string) string {
	value = strings.TrimSpace(value)
	if value == "openapi_token" {
		return value
	}
	return "account_password"
}
func parseGoodsRows(raw json.RawMessage) ([]goodsRow, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	items := flattenItems(raw)
	rows := make([]goodsRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, parseGoodsItem(item))
	}
	return rows, nil
}

func flattenItems(raw json.RawMessage) []json.RawMessage {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"list", "items", "rows", "data", "goods", "tokens"} {
		if value, ok := obj[key]; ok {
			if rows := flattenItems(value); len(rows) > 0 {
				return rows
			}
		}
	}
	return []json.RawMessage{raw}
}

func parseGoodsItem(raw json.RawMessage) goodsRow {
	var row map[string]interface{}
	_ = json.Unmarshal(raw, &row)
	goodsID := firstNonEmpty(toString(row["goods_id"]), toString(row["goodsId"]), toString(row["id"]), toString(row["goodsid"]))
	serviceID := firstNonEmpty(toString(row["pingtai_id"]), toString(row["pingtaiId"]), toString(row["pingtai"]), toString(row["platform_id"]), toString(row["platformId"]), toString(row["service_id"]), toString(row["serviceId"]))
	countryID := firstNonEmpty(toString(row["country"]), toString(row["country_id"]), toString(row["countryId"]))
	if goodsID != "" {
		parts := strings.Split(goodsID, "-")
		if serviceID == "" && len(parts) >= 1 {
			serviceID = parts[0]
		}
		if countryID == "" && len(parts) >= 2 {
			countryID = parts[1]
		}
	}
	return goodsRow{
		GoodsID:     goodsID,
		PingtaiID:   serviceID,
		PingtaiText: firstNonEmpty(toString(row["pingtai_text"]), toString(row["pingtaiText"]), toString(row["pingtai_name"]), toString(row["platform_name"]), toString(row["name"]), toString(row["title"]), serviceID),
		Country:     countryID,
		CountryText: firstNonEmpty(toString(row["country_text"]), toString(row["countryText"]), toString(row["country_name"]), toString(row["countryName"]), countryName(countryID), countryID),
		EndDay:      firstNonEmpty(toString(row["endday"]), toString(row["end_day"]), toString(row["days"]), toString(row["validity"]), endDayFromGoodsID(goodsID)),
		Price:       firstNonEmpty(toString(row["price"]), toString(row["money"]), toString(row["amount"]), toString(row["cost"])),
		Stock:       firstNonZeroInt(toInt(row["stock"]), toInt(row["stocks"]), toInt(row["surplus"]), toInt(row["num"]), toInt(row["count"]), toInt(row["quantity"])),
		Raw:         raw,
	}
}

func parseAccountPriceStoreRows(raw json.RawMessage) ([]accountPriceStoreRow, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	items := flattenAccountPriceStoreItems(raw)
	rows := make([]accountPriceStoreRow, 0, len(items))
	for _, item := range items {
		var row accountPriceStoreRow
		if err := json.Unmarshal(item, &row); err != nil {
			continue
		}
		if row.EndDay == "" {
			var values map[string]interface{}
			_ = json.Unmarshal(item, &values)
			row.EndDay = toString(values["endday"])
			row.Price = firstNonEmpty(row.Price, toString(values["price"]))
			row.Store = firstNonZeroInt(row.Store, toInt(values["store"]))
			row.EndDayName = firstNonEmpty(row.EndDayName, toString(values["enddayname"]))
		}
		row.Raw = item
		if row.EndDay != "" {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return parseInt(rows[i].EndDay) < parseInt(rows[j].EndDay) })
	return rows, nil
}

func flattenAccountPriceStoreItems(raw json.RawMessage) []json.RawMessage {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"list", "items", "rows", "data", "goods"} {
		if value, ok := obj[key]; ok {
			if rows := flattenAccountPriceStoreItems(value); len(rows) > 0 {
				return rows
			}
		}
	}
	rows := make([]json.RawMessage, 0, len(obj))
	for _, value := range obj {
		rows = append(rows, value)
	}
	return rows
}

func extractBalance(raw json.RawMessage) string {
	var row map[string]interface{}
	if err := json.Unmarshal(raw, &row); err != nil {
		return ""
	}
	return firstNonEmpty(toString(row["balance"]), toString(row["money"]), toString(row["amount"]), toString(row["available_balance"]), toString(row["availableBalance"]), toString(row["credit"]), toString(row["wallet"]))
}

func extractOrderID(raw json.RawMessage) string {
	var row map[string]interface{}
	if err := json.Unmarshal(raw, &row); err == nil {
		if value := firstNonEmpty(toString(row["order_id"]), toString(row["orderId"]), toString(row["id"])); value != "" {
			return value
		}
		if nestedRaw, ok := row["order"].(map[string]interface{}); ok {
			if value := firstNonEmpty(toString(nestedRaw["order_id"]), toString(nestedRaw["id"])); value != "" {
				return value
			}
		}
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return firstNonEmpty(toString(list[0]["order_id"]), toString(list[0]["orderId"]), toString(list[0]["id"]))
	}
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" || strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		return ""
	}
	return strings.Trim(value, `"`)
}

func extractFirstToken(raw json.RawMessage) (string, string) {
	items := flattenItems(raw)
	for _, item := range items {
		var row map[string]interface{}
		if err := json.Unmarshal(item, &row); err != nil {
			continue
		}
		token := firstNonEmpty(toString(row["token"]), toString(row["order_token"]), toString(row["orderToken"]), toString(row["key"]))
		phone := firstNonEmpty(toString(row["number"]), toString(row["phone"]), toString(row["phone_number"]), toString(row["phoneNumber"]))
		if token != "" || phone != "" {
			return token, phone
		}
	}
	return "", ""
}

func extractMessage(raw json.RawMessage) string {
	if message := extractMessageFromObject(raw); message != "" {
		return message
	}
	items := flattenItems(raw)
	for _, item := range items {
		if message := extractMessageFromObject(item); message != "" {
			return message
		}
		text := strings.Trim(strings.TrimSpace(string(item)), `"`)
		if text != "" && text != "null" && !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[") {
			return text
		}
	}
	return ""
}

func parseAccountNumberFile(raw json.RawMessage) (string, string, error) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.Trim(line, "\ufeff"))
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 2 {
			continue
		}
		phone := strings.TrimSpace(parts[0])
		messageURL := accountMessageURL(strings.TrimSpace(parts[1]))
		if phone != "" && messageURL != "" {
			return phone, messageURL, nil
		}
	}
	return "", "", sms.NewProviderError(sms.ErrProviderRejected, "62-us number file format is invalid")
}

func selectedEndDay(raw json.RawMessage) string {
	var parsed serviceConfigMetadata
	if len(raw) > 0 && json.Unmarshal(raw, &parsed) == nil {
		return firstNonEmpty(parsed.EndDay, parsed.ValidityType)
	}
	return ""
}

func isNoSMSContent(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" || content == "[]" || content == "{}" || content == "null" {
		return true
	}
	lower := strings.ToLower(content)
	for _, marker := range []string{
		"no sms",
		"no message",
		"not found",
		"empty",
		"暂无",
		"没有",
		"无短信",
		"未收到",
		"未获取",
	} {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func jsonRawHasEmptyData(raw json.RawMessage) bool {
	var row map[string]interface{}
	if err := json.Unmarshal(raw, &row); err != nil {
		return false
	}
	data, ok := row["data"]
	if !ok || data == nil {
		return true
	}
	switch typed := data.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []interface{}:
		return len(typed) == 0
	case map[string]interface{}:
		return len(typed) == 0
	default:
		return false
	}
}

func extractMessageFromObject(raw json.RawMessage) string {
	var row map[string]interface{}
	if err := json.Unmarshal(raw, &row); err != nil {
		return ""
	}
	message := firstNonEmpty(toString(row["message"]), toString(row["sms"]), toString(row["content"]), toString(row["text"]), toString(row["msg"]), toString(row["body"]))
	if message != "" {
		return message
	}
	if data, ok := row["data"]; ok {
		switch typed := data.(type) {
		case string:
			return strings.TrimSpace(typed)
		case []interface{}:
			for _, item := range typed {
				if nested, ok := item.(map[string]interface{}); ok {
					message = firstNonEmpty(toString(nested["message"]), toString(nested["sms"]), toString(nested["content"]), toString(nested["text"]), toString(nested["msg"]), toString(nested["body"]))
					if message != "" {
						return message
					}
				}
			}
		case map[string]interface{}:
			return firstNonEmpty(toString(typed["message"]), toString(typed["sms"]), toString(typed["content"]), toString(typed["text"]), toString(typed["msg"]), toString(typed["body"]))
		}
	}
	return ""
}

func accountMessageURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "http://a.62-us.com/api/get_sms?key=" + url.QueryEscape(value)
}

func normalizeAccountDownloadURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(value, "\\/", "/")
}

func looksLikeJSONError(raw json.RawMessage) bool {
	var payload struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	return payload.Code != 0 && payload.Code != 1
}

func jsonNumberOrString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if number, err := strconv.Atoi(value); err == nil {
		return number
	}
	return value
}

func jsonValueString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

func validityRange(value, label string) (int, int) {
	switch strings.TrimSpace(value) {
	case "1":
		return 6, 15
	case "2":
		return 16, 30
	case "3":
		return 31, 60
	case "4":
		return 61, 0
	}
	numbers := regexp.MustCompile(`\d+`).FindAllString(label, -1)
	if len(numbers) >= 2 {
		return parseInt(numbers[0]) + 1, parseInt(numbers[1])
	}
	if len(numbers) == 1 {
		return parseInt(numbers[0]) + 1, 0
	}
	return parseInt(value), parseInt(value)
}

func endDayFromGoodsID(goodsID string) string {
	parts := strings.Split(strings.TrimSpace(goodsID), "-")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

func successCode(raw json.RawMessage) bool {
	return statusCode(raw) == "1"
}

func statusCode(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" || strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		return ""
	}
	return strings.Trim(value, `"`)
}

func httpError(status int, raw json.RawMessage) error {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return sms.NewProviderError("AUTH_ERROR", fmt.Sprintf("62-us http %d: %s", status, string(raw)))
	}
	return fmt.Errorf("62-us http %d: %s", status, string(raw))
}

func providerError(code string, message string) error {
	message = firstNonEmpty(message, "62-us request failed")
	switch code {
	case "40101", "40102", "40301", "40302", "40304", "40401":
		return sms.NewProviderError("AUTH_ERROR", message)
	case "42204":
		return sms.NewProviderError(sms.ErrBalance, message)
	case "42205":
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	case "42201", "42202", "42203":
		return sms.NewProviderError(sms.ErrPriceNotFound, message)
	case "42206", "42207", "40411", "40412":
		return sms.NewProviderError(sms.ErrOrderNotFound, message)
	}
	if strings.Contains(strings.ToLower(message), "balance") || strings.Contains(message, "余额") {
		return sms.NewProviderError(sms.ErrBalance, message)
	}
	if strings.Contains(strings.ToLower(message), "stock") || strings.Contains(message, "库存") {
		return sms.NewProviderError(sms.ErrOutOfStock, message)
	}
	if strings.Contains(strings.ToLower(message), "api key") || strings.Contains(strings.ToLower(message), "authorization") || strings.Contains(message, "授权") {
		return sms.NewProviderError("AUTH_ERROR", message)
	}
	return sms.NewProviderError(sms.ErrProviderRejected, message)
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

func extractCode(text string) string {
	if match := regexp.MustCompile(`\d{3,8}`).FindString(text); match != "" {
		return match
	}
	return strings.TrimSpace(text)
}

func countryName(countryID string) string {
	switch countryID {
	case "1", "188", "US", "us":
		return "United States"
	case "33", "CA", "ca":
		return "Canada"
	default:
		return ""
	}
}

func dialCode(countryID string) string {
	switch countryID {
	case "1", "188", "33", "US", "us", "CA", "ca":
		return "1"
	default:
		return ""
	}
}

func priceLess(left, right string) bool {
	leftPrice := parseFloat(left)
	rightPrice := parseFloat(right)
	if leftPrice <= 0 {
		return false
	}
	if rightPrice <= 0 {
		return true
	}
	return leftPrice < rightPrice
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func parseInt(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func stockTextToAmount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if amount := parseInt(value); amount > 0 {
		return amount
	}
	if strings.Contains(value, "充足") || strings.Contains(strings.ToLower(value), "enough") {
		return 999999
	}
	return 0
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func toInt(value interface{}) int {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return int(parsed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func mustMarshal(value interface{}) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func valuesToJSON(values url.Values) accountJSONValues {
	result := accountJSONValues{}
	for key, items := range values {
		if len(items) > 0 {
			result[key] = items[0]
		}
	}
	return result
}

func jsonValuesToLogValues(values accountJSONValues) url.Values {
	result := url.Values{}
	for key, value := range values {
		switch typed := value.(type) {
		case []string:
			result.Set(key, strings.Join(typed, ","))
		default:
			result.Set(key, fmt.Sprint(typed))
		}
	}
	return result
}

func mergeLogValues(values ...url.Values) url.Values {
	merged := url.Values{}
	for _, group := range values {
		for key, items := range group {
			for _, item := range items {
				merged.Add(key, item)
			}
		}
	}
	return merged
}

func sanitizeLogBody(action string, body []byte) string {
	text := string(body)
	if action == "/api/user/login" {
		var value map[string]interface{}
		if err := json.Unmarshal(body, &value); err == nil {
			if data, ok := value["data"].(map[string]interface{}); ok {
				if _, ok := data["token"]; ok {
					data["token"] = "***"
				}
				if userInfo, ok := data["userinfo"].(map[string]interface{}); ok {
					if _, ok := userInfo["token"]; ok {
						userInfo["token"] = "***"
					}
				}
			}
			if raw, err := json.Marshal(value); err == nil {
				return string(raw)
			}
		}
	}
	return text
}

var _ sms.SMSProvider = (*Client)(nil)
var _ sms.MetadataProvider = (*Client)(nil)
var _ sms.CatalogProvider = (*Client)(nil)
var _ sms.ValidityOptionsProvider = (*Client)(nil)
var _ sms.LongLivedProvider = (*Client)(nil)
var _ sms.AdvancedConfigurableProvider = (*Client)(nil)
