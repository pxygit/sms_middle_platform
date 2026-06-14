package service

import (
	"sms-middle-platform/backend/internal/model"

	"gorm.io/gorm"
)

type SupplierLogService struct {
	db *gorm.DB
}

func NewSupplierLogService(db *gorm.DB) *SupplierLogService {
	return &SupplierLogService{db: db}
}

func (s *SupplierLogService) Record(entry model.SupplierRequestLog) {
	_ = s.db.Create(&entry).Error
}
