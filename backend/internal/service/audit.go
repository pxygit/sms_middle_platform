package service

import (
	"encoding/json"

	"sms-middle-platform/backend/internal/model"

	"gorm.io/gorm"
)

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) Record(actorType string, actorID uint, action, resourceType, resourceID, ip, userAgent string, metadata interface{}) {
	var raw []byte
	if metadata != nil {
		raw, _ = json.Marshal(metadata)
	}
	_ = s.db.Create(&model.AuditLog{
		ActorType:    actorType,
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IP:           ip,
		UserAgent:    userAgent,
		Metadata:     raw,
	}).Error
}

func (s *AuditService) List(limit, offset int) ([]model.AuditLog, error) {
	var logs []model.AuditLog
	err := s.db.Order("id desc").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, err
}
