package smspool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/model"
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     func(model.SupplierRequestLog)
}

func New(apiKey, baseURL string, timeout time.Duration, logger func(model.SupplierRequestLog)) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

func (c *Client) Name() string {
	return "smspool"
}

func (c *Client) GetBalance(ctx context.Context) (*sms.ProviderBalance, error) {
	var out struct {
		Balance string `json:"balance"`
	}
	raw, status, err := c.postForm(ctx, "/request/balance", url.Values{"key": {c.apiKey}})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, parseError(raw)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &sms.ProviderBalance{Balance: out.Balance}, nil
}

func (c *Client) RequestNumber(ctx context.Context, input sms.RequestNumberInput) (*sms.RequestNumberResult, error) {
	form := url.Values{
		"key":             {c.apiKey},
		"country":         {input.CountryID},
		"service":         {input.ServiceID},
		"quantity":        {"1"},
		"activation_type": {"SMS"},
		"create_token":    {"0"},
	}
	if input.PoolID != "" {
		form.Set("pool", input.PoolID)
	}
	if input.MaxPrice > 0 {
		form.Set("max_price", strconv.FormatFloat(input.MaxPrice, 'f', 4, 64))
	}

	raw, status, err := c.postForm(ctx, "/purchase/sms", form)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, parseError(raw)
	}
	var out struct {
		Success     int             `json:"success"`
		Number      json.RawMessage `json:"number"`
		CC          string          `json:"cc"`
		PhoneNumber string          `json:"phonenumber"`
		OrderID     string          `json:"order_id"`
		Country     string          `json:"country"`
		Service     string          `json:"service"`
		Cost        string          `json:"cost"`
		ExpiresIn   int             `json:"expires_in"`
		Expiration  int64           `json:"expiration"`
		Message     string          `json:"message"`
		Type        string          `json:"type"`
		Token       string          `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Success != 1 || out.OrderID == "" {
		if out.Type != "" {
			return nil, sms.NewProviderError(out.Type, out.Message)
		}
		return nil, sms.NewProviderError(sms.ErrProviderRejected, out.Message)
	}
	cost, _ := strconv.ParseFloat(out.Cost, 64)
	phone := out.PhoneNumber
	if out.CC != "" && out.PhoneNumber != "" {
		phone = "+" + out.CC + out.PhoneNumber
	}
	return &sms.RequestNumberResult{
		SupplierOrderID: out.OrderID,
		SupplierToken:   out.Token,
		PhoneNumber:     phone,
		Country:         out.Country,
		Service:         out.Service,
		Cost:            cost,
		ExpiresIn:       out.ExpiresIn,
		Expiration:      out.Expiration,
		Raw:             raw,
	}, nil
}

func (c *Client) CheckSMS(ctx context.Context, input sms.CheckSMSInput) (*sms.SMSResult, error) {
	raw, status, err := c.postForm(ctx, "/sms/check", url.Values{
		"key":     {c.apiKey},
		"orderid": {input.SupplierOrderID},
	})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, parseError(raw)
	}
	var out struct {
		Status     int    `json:"status"`
		SMS        string `json:"sms"`
		FullSMS    string `json:"full_sms"`
		Message    string `json:"message"`
		Expiration int64  `json:"expiration"`
		TimeLeft   int    `json:"time_left"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	result := &sms.SMSResult{
		SupplierStatus: strconv.Itoa(out.Status),
		Expiration:     out.Expiration,
		TimeLeft:       out.TimeLeft,
		Raw:            raw,
	}
	switch out.Status {
	case 3:
		result.Status = "sms_received"
		result.VerificationCode = out.SMS
		result.SMSContent = out.FullSMS
	case 6:
		result.Status = "cancelled"
		result.SMSContent = out.Message
	default:
		result.Status = "active"
	}
	return result, nil
}

func (c *Client) CancelNumber(ctx context.Context, input sms.CancelNumberInput) (*sms.CancelResult, error) {
	raw, status, err := c.postForm(ctx, "/sms/cancel", url.Values{
		"key":     {c.apiKey},
		"orderid": {input.SupplierOrderID},
	})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, parseError(raw)
	}
	var out struct {
		Success int    `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Success != 1 {
		return nil, sms.NewProviderError(sms.ErrProviderRejected, out.Message)
	}
	return &sms.CancelResult{Success: true, Message: out.Message, Raw: raw}, nil
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values) ([]byte, int, error) {
	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)
	for key, values := range form {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, 0, err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, requestBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log(path, form, 0, false, "", err.Error(), time.Since(start), nil)
		return nil, 0, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log(path, form, resp.StatusCode, false, "", err.Error(), time.Since(start), responseBody)
		return nil, resp.StatusCode, err
	}
	c.log(path, form, resp.StatusCode, resp.StatusCode < 400, "", "", time.Since(start), responseBody)
	return responseBody, resp.StatusCode, nil
}

func (c *Client) log(action string, form url.Values, status int, success bool, code, message string, latency time.Duration, body []byte) {
	if c.logger == nil {
		return
	}
	safeForm := url.Values{}
	for key, values := range form {
		if key == "key" {
			safeForm.Set(key, "***")
			continue
		}
		for _, value := range values {
			safeForm.Add(key, value)
		}
	}
	c.logger(model.SupplierRequestLog{
		ProviderCode: "smspool",
		Action:       action,
		HTTPStatus:   status,
		Success:      success,
		ErrorCode:    code,
		ErrorMessage: message,
		LatencyMS:    latency.Milliseconds(),
		RequestBody:  safeForm.Encode(),
		ResponseBody: string(body),
	})
}

func parseError(raw []byte) error {
	var out struct {
		Success int    `json:"success"`
		Message string `json:"message"`
		Type    string `json:"type"`
		Errors  []struct {
			Message string `json:"message"`
			Param   string `json:"param"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("smspool error: %s", string(raw))
	}
	message := out.Message
	if message == "" && len(out.Errors) > 0 {
		message = out.Errors[0].Message
	}
	code := out.Type
	if code == "" {
		code = sms.ErrProviderRejected
	}
	if strings.Contains(strings.ToLower(message), "could not find this order") {
		code = sms.ErrOrderNotFound
	}
	if strings.Contains(strings.ToLower(message), "cannot be cancelled") {
		code = sms.ErrCannotCancel
	}
	return sms.NewProviderError(code, message)
}
