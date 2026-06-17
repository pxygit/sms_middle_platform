package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/model"
	"sms-middle-platform/backend/internal/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderService struct {
	db       *gorm.DB
	registry *sms.Registry
}

func NewOrderService(db *gorm.DB, registry *sms.Registry) *OrderService {
	return &OrderService{db: db, registry: registry}
}

func (s *OrderService) Create(ctx context.Context, cardCode string) (*model.ReceiveOrder, error) {
	var order model.ReceiveOrder
	var card model.CardCode

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("ServiceConfig").
			Where("code_hash = ?", util.HashCardCode(cardCode)).
			First(&card).Error; err != nil {
			return errors.New("card code not found")
		}
		if err := validateCard(card); err != nil {
			return err
		}

		var activeCount int64
		if err := tx.Model(&model.ReceiveOrder{}).
			Where("card_code_id = ? AND status IN ?", card.ID, []string{model.OrderCreated, model.OrderActive, model.OrderCancelRequested}).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount >= int64(card.RemainingUses) {
			return errors.New("card code has no remaining uses")
		}

		order = model.ReceiveOrder{
			OrderNo:         util.GenerateOrderNo(),
			CardCodeID:      card.ID,
			ProviderCode:    card.ProviderCode,
			ServiceConfigID: card.ServiceConfigID,
			MaxPrice:        card.ServiceConfig.MaxPrice,
			Status:          model.OrderCreated,
		}
		return tx.Create(&order).Error
	})
	if err != nil {
		return nil, err
	}

	provider, err := s.registry.Get(card.ProviderCode)
	if err != nil {
		s.markFailed(order.ID, err.Error())
		return nil, err
	}
	result, err := provider.RequestNumber(ctx, sms.RequestNumberInput{
		CountryID: card.ServiceConfig.ProviderCountryID,
		ServiceID: card.ServiceConfig.ProviderServiceID,
		PoolID:    card.ServiceConfig.ProviderPoolID,
		MaxPrice:  card.ServiceConfig.MaxPrice,
	})
	if err != nil {
		s.markFailed(order.ID, err.Error())
		return nil, err
	}

	now := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&card, card.ID).Error; err != nil {
			return err
		}
		if card.RemainingUses <= 0 {
			return errors.New("card code has no remaining uses")
		}
		if err := tx.Model(&card).Update("remaining_uses", gorm.Expr("remaining_uses - 1")).Error; err != nil {
			return err
		}
		return tx.Model(&order).Updates(map[string]interface{}{
			"supplier_order_id":     result.SupplierOrderID,
			"supplier_token":        result.SupplierToken,
			"phone_number":          result.PhoneNumber,
			"phone_country_code":    result.PhoneCountryCode,
			"phone_national_number": result.PhoneNationalNumber,
			"cost":                  result.Cost,
			"status":                model.OrderActive,
			"supplier_status":       "active",
			"raw_response":          []byte(result.Raw),
			"started_at":            now,
		}).Error
	})
	if err != nil {
		s.markFailed(order.ID, err.Error())
		return nil, err
	}
	return s.GetByCard(ctx, order.OrderNo, cardCode)
}

func (s *OrderService) GetByCard(ctx context.Context, orderNo, cardCode string) (*model.ReceiveOrder, error) {
	var order model.ReceiveOrder
	err := s.db.Joins("JOIN sys_card_codes ON sys_card_codes.id = sys_receive_orders.card_code_id").
		Preload("ServiceConfig").
		Where("sys_receive_orders.order_no = ? AND sys_card_codes.code_hash = ?", orderNo, util.HashCardCode(cardCode)).
		First(&order).Error
	if err != nil {
		return nil, errors.New("order not found")
	}
	normalizeDisplayCost(&order)
	return &order, nil
}

func (s *OrderService) History(cardCode string, limit, offset int) ([]model.ReceiveOrder, error) {
	var card model.CardCode
	if err := s.db.Where("code_hash = ?", util.HashCardCode(cardCode)).First(&card).Error; err != nil {
		return nil, errors.New("card code not found")
	}
	var orders []model.ReceiveOrder
	err := s.db.Preload("ServiceConfig").
		Where("card_code_id = ?", card.ID).
		Where("COALESCE(phone_number, '') <> ''").
		Order("id desc").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error
	for index := range orders {
		normalizeDisplayCost(&orders[index])
	}
	return orders, err
}

func (s *OrderService) CancelByCard(ctx context.Context, orderNo, cardCode string) (*model.ReceiveOrder, error) {
	order, err := s.GetByCard(ctx, orderNo, cardCode)
	if err != nil {
		return nil, err
	}
	return s.Cancel(ctx, order.ID)
}

func (s *OrderService) Cancel(ctx context.Context, orderID uint) (*model.ReceiveOrder, error) {
	var order model.ReceiveOrder
	if err := s.db.First(&order, orderID).Error; err != nil {
		return nil, errors.New("order not found")
	}
	if order.Status == model.OrderSMSReceived {
		return nil, errors.New("order already received sms")
	}
	if order.Status == model.OrderCancelled {
		return &order, nil
	}
	if order.Status != model.OrderActive {
		return nil, errors.New("order cannot be cancelled in current status")
	}
	if order.StartedAt == nil || time.Since(*order.StartedAt) < 2*time.Minute {
		return nil, errors.New("cancel is allowed after two minutes if no sms has been received")
	}

	if err := s.db.Model(&order).Update("status", model.OrderCancelRequested).Error; err != nil {
		return nil, err
	}
	provider, err := s.registry.Get(order.ProviderCode)
	if err != nil {
		_ = s.db.Model(&order).Updates(map[string]interface{}{"status": model.OrderActive, "failure_reason": err.Error()}).Error
		return nil, err
	}
	_, err = provider.CancelNumber(ctx, sms.CancelNumberInput{SupplierOrderID: order.SupplierOrderID})
	if err != nil {
		_ = s.db.Model(&order).Updates(map[string]interface{}{"status": model.OrderActive, "failure_reason": err.Error()}).Error
		return nil, err
	}

	now := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":       model.OrderCancelled,
			"cancelled_at": now,
		}
		if order.ProviderCode == "firefox" {
			updates["cost"] = 0
		}
		if err := tx.Model(&model.ReceiveOrder{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Model(&model.CardCode{}).
			Where("id = ? AND remaining_uses < total_uses", order.CardCodeID).
			Update("remaining_uses", gorm.Expr("remaining_uses + 1")).Error
	})
	if err != nil {
		return nil, err
	}
	return s.getByID(order.ID)
}

func (s *OrderService) List(limit, offset int) ([]model.ReceiveOrder, error) {
	var orders []model.ReceiveOrder
	err := s.db.Preload("ServiceConfig").Order("id desc").Limit(limit).Offset(offset).Find(&orders).Error
	for index := range orders {
		normalizeDisplayCost(&orders[index])
	}
	return orders, err
}

func (s *OrderService) PollActive(ctx context.Context, staleBefore time.Time) error {
	var orders []model.ReceiveOrder
	if err := s.db.Preload("ServiceConfig").
		Where("status = ? AND (last_polled_at IS NULL OR last_polled_at < ?)", model.OrderActive, staleBefore).
		Limit(20).
		Find(&orders).Error; err != nil {
		return err
	}
	for _, order := range orders {
		if err := s.pollOne(ctx, order); err != nil {
			fmt.Printf("poll order %s failed: %v\n", order.OrderNo, err)
		}
	}
	return nil
}

func (s *OrderService) pollOne(ctx context.Context, order model.ReceiveOrder) error {
	now := time.Now()
	timeout := time.Duration(order.ServiceConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	if order.StartedAt != nil && now.After(order.StartedAt.Add(timeout)) {
		return s.db.Model(&order).Updates(map[string]interface{}{
			"status":         model.OrderExpired,
			"expired_at":     now,
			"last_polled_at": now,
		}).Error
	}
	provider, err := s.registry.Get(order.ProviderCode)
	if err != nil {
		return err
	}
	result, err := provider.CheckSMS(ctx, sms.CheckSMSInput{SupplierOrderID: order.SupplierOrderID})
	updates := map[string]interface{}{
		"last_polled_at":  now,
		"supplier_status": resultStatus(result),
	}
	if result != nil {
		updates["raw_response"] = []byte(result.Raw)
	}
	if err != nil {
		updates["failure_reason"] = err.Error()
		return s.db.Model(&order).Updates(updates).Error
	}
	switch result.Status {
	case model.OrderSMSReceived:
		updates["status"] = model.OrderSMSReceived
		updates["verification_code"] = result.VerificationCode
		updates["sms_content"] = result.SMSContent
		updates["received_at"] = now
		if order.ProviderCode == "firefox" {
			updates["cost"] = s.firefoxReceivedCost(ctx, order)
		}
	case model.OrderCancelled:
		updates["status"] = model.OrderCancelled
		updates["cancelled_at"] = now
	}
	return s.db.Model(&order).Updates(updates).Error
}

func (s *OrderService) markFailed(orderID uint, reason string) {
	_ = s.db.Model(&model.ReceiveOrder{}).Where("id = ?", orderID).Updates(map[string]interface{}{
		"status":         model.OrderFailed,
		"failure_reason": reason,
	}).Error
}

func (s *OrderService) getByID(id uint) (*model.ReceiveOrder, error) {
	var order model.ReceiveOrder
	if err := s.db.Preload("ServiceConfig").First(&order, id).Error; err != nil {
		return nil, err
	}
	normalizeDisplayCost(&order)
	return &order, nil
}

func (s *OrderService) firefoxReceivedCost(ctx context.Context, order model.ReceiveOrder) float64 {
	if order.Cost > 0 {
		return order.Cost
	}
	provider, err := s.registry.Get(order.ProviderCode)
	if err == nil {
		if metadataProvider, ok := provider.(sms.MetadataProvider); ok {
			price, err := metadataProvider.GetPrice(ctx, sms.ProviderPriceInput{
				CountryID: order.ServiceConfig.ProviderCountryID,
				ServiceID: order.ServiceConfig.ProviderServiceID,
				PoolID:    order.ServiceConfig.ProviderPoolID,
			})
			if err == nil {
				if parsed := parseCost(firstNonEmpty(price.Price, price.LowPrice, price.HighPrice)); parsed > 0 {
					return parsed
				}
			}
		}
	}
	if order.MaxPrice > 0 {
		return order.MaxPrice
	}
	return 0
}

func normalizeDisplayCost(order *model.ReceiveOrder) {
	if order.ProviderCode != "firefox" {
		return
	}
	if order.Status != model.OrderSMSReceived {
		order.Cost = 0
		return
	}
	if order.Cost <= 0 && order.MaxPrice > 0 {
		order.Cost = order.MaxPrice
	}
}

func parseCost(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func validateCard(card model.CardCode) error {
	if card.Status != model.StatusEnabled {
		return errors.New("card code is not enabled")
	}
	if card.ExpiresAt != nil && time.Now().After(*card.ExpiresAt) {
		return errors.New("card code has expired")
	}
	if card.RemainingUses <= 0 {
		return errors.New("card code has no remaining uses")
	}
	if card.ServiceConfig.Status != model.StatusEnabled {
		return errors.New("service is disabled")
	}
	return nil
}

func resultStatus(result *sms.SMSResult) string {
	if result == nil {
		return ""
	}
	return result.SupplierStatus
}
