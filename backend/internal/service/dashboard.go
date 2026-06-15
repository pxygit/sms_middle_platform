package service

import (
	"time"

	"sms-middle-platform/backend/internal/model"

	"gorm.io/gorm"
)

type DashboardService struct {
	db *gorm.DB
}

type DashboardStats struct {
	TotalCompletedOrders int64           `json:"totalCompletedOrders"`
	TodayCompletedOrders int64           `json:"todayCompletedOrders"`
	ActiveOrders         int64           `json:"activeOrders"`
	TodayOrders          int64           `json:"todayOrders"`
	TodayVisits          int64           `json:"todayVisits"`
	TotalVisits          int64           `json:"totalVisits"`
	AvailableCards       int64           `json:"availableCards"`
	ProviderRanking      []DashboardRank `json:"providerRanking"`
	ServiceRanking       []DashboardRank `json:"serviceRanking"`
	StatusSummary        []DashboardRank `json:"statusSummary"`
}

type DashboardRank struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

func (s *DashboardService) RecordVisit(path, ip, userAgent string) error {
	if path == "" {
		path = "/"
	}
	return s.db.Create(&model.SiteVisit{
		Path:      path,
		IP:        ip,
		UserAgent: userAgent,
	}).Error
}

func (s *DashboardService) Stats() (*DashboardStats, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	stats := &DashboardStats{}

	if err := s.db.Model(&model.ReceiveOrder{}).Where("status = ?", model.OrderSMSReceived).Count(&stats.TotalCompletedOrders).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.ReceiveOrder{}).Where("status = ? AND received_at >= ?", model.OrderSMSReceived, todayStart).Count(&stats.TodayCompletedOrders).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.ReceiveOrder{}).Where("status IN ?", []string{model.OrderCreated, model.OrderActive, model.OrderCancelRequested}).Count(&stats.ActiveOrders).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.ReceiveOrder{}).Where("created_at >= ?", todayStart).Count(&stats.TodayOrders).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.SiteVisit{}).Count(&stats.TotalVisits).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.SiteVisit{}).Where("created_at >= ?", todayStart).Count(&stats.TodayVisits).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CardCode{}).Where("status = ? AND remaining_uses > 0", model.StatusEnabled).Count(&stats.AvailableCards).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.ReceiveOrder{}).
		Select("provider_code AS key, provider_code AS name, count(*) AS count").
		Where("status = ?", model.OrderSMSReceived).
		Group("provider_code").
		Order("count desc").
		Limit(6).
		Scan(&stats.ProviderRanking).Error; err != nil {
		return nil, err
	}
	if err := s.db.Table("sys_receive_orders").
		Select("sys_service_configs.target_platform AS key, sys_service_configs.target_platform AS name, count(*) AS count").
		Joins("JOIN sys_service_configs ON sys_service_configs.id = sys_receive_orders.service_config_id").
		Where("sys_receive_orders.status = ?", model.OrderSMSReceived).
		Group("sys_service_configs.target_platform").
		Order("count desc").
		Limit(8).
		Scan(&stats.ServiceRanking).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.ReceiveOrder{}).
		Select("status AS key, status AS name, count(*) AS count").
		Group("status").
		Order("count desc").
		Scan(&stats.StatusSummary).Error; err != nil {
		return nil, err
	}

	return stats, nil
}
