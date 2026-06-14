package service

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sms-middle-platform/backend/internal/model"
	"sms-middle-platform/backend/internal/util"

	"gorm.io/gorm"
)

type CardService struct {
	db            *gorm.DB
	exportDir     string
	encryptionKey string
}

type CreateBatchInput struct {
	Name            string     `json:"name" binding:"required"`
	ServiceConfigID uint       `json:"serviceConfigId" binding:"required"`
	Quantity        int        `json:"quantity" binding:"required"`
	UsesPerCode     int        `json:"usesPerCode" binding:"required"`
	ExpiresAt       *time.Time `json:"expiresAt"`
}

type VerifyCardResult struct {
	CodeMask      string              `json:"codeMask"`
	RemainingUses int                 `json:"remainingUses"`
	ExpiresAt     *time.Time          `json:"expiresAt"`
	ServiceConfig model.ServiceConfig `json:"serviceConfig"`
}

func NewCardService(db *gorm.DB, exportDir, encryptionKey string) *CardService {
	return &CardService{db: db, exportDir: exportDir, encryptionKey: encryptionKey}
}

func (s *CardService) CreateBatch(input CreateBatchInput, adminID uint) (*model.CardBatch, error) {
	if input.Quantity <= 0 || input.Quantity > 10000 {
		return nil, errors.New("quantity must be between 1 and 10000")
	}
	if input.UsesPerCode <= 0 {
		return nil, errors.New("uses per code must be greater than 0")
	}

	var config model.ServiceConfig
	if err := s.db.First(&config, input.ServiceConfigID).Error; err != nil {
		return nil, err
	}
	var batch *model.CardBatch
	err := s.db.Transaction(func(tx *gorm.DB) error {
		b := model.CardBatch{
			Name:            input.Name,
			ProviderCode:    config.ProviderCode,
			ServiceConfigID: input.ServiceConfigID,
			Quantity:        input.Quantity,
			UsesPerCode:     input.UsesPerCode,
			ExpiresAt:       input.ExpiresAt,
			CreatedBy:       adminID,
		}
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
		codes := make([]string, 0, input.Quantity)
		for i := 0; i < input.Quantity; i++ {
			code, err := util.GenerateCardCode()
			if err != nil {
				return err
			}
			cipherText, err := util.EncryptString(s.encryptionKey, code)
			if err != nil {
				return err
			}
			codes = append(codes, code)
			card := model.CardCode{
				CodeHash:        util.HashCardCode(code),
				CodeMask:        util.MaskCardCode(code),
				CodeCipher:      cipherText,
				ProviderCode:    config.ProviderCode,
				ServiceConfigID: input.ServiceConfigID,
				BatchID:         b.ID,
				TotalUses:       input.UsesPerCode,
				RemainingUses:   input.UsesPerCode,
				ExpiresAt:       input.ExpiresAt,
				Status:          model.StatusEnabled,
			}
			if err := tx.Create(&card).Error; err != nil {
				return err
			}
		}
		export := strings.Join(codes, "\n")
		if err := os.MkdirAll(s.exportDir, 0700); err != nil {
			return err
		}
		exportPath := filepath.Join(s.exportDir, "card-batch-"+strconv.FormatUint(uint64(b.ID), 10)+".txt")
		if err := os.WriteFile(exportPath, []byte(export), 0600); err != nil {
			return err
		}
		if err := tx.Model(&b).Update("export_path", exportPath).Error; err != nil {
			return err
		}
		b.ExportPath = exportPath
		batch = &b
		return nil
	})
	return batch, err
}

func (s *CardService) Verify(code string) (*VerifyCardResult, error) {
	card, err := s.findUsableCard(code)
	if err != nil {
		return nil, err
	}
	return &VerifyCardResult{
		CodeMask:      card.CodeMask,
		RemainingUses: card.RemainingUses,
		ExpiresAt:     card.ExpiresAt,
		ServiceConfig: card.ServiceConfig,
	}, nil
}

func (s *CardService) ListCards(limit, offset int) ([]model.CardCode, error) {
	var cards []model.CardCode
	err := s.db.Preload("ServiceConfig").Order("id desc").Limit(limit).Offset(offset).Find(&cards).Error
	return cards, err
}

func (s *CardService) UpdateStatus(id uint, status string) error {
	if status != model.StatusEnabled && status != model.StatusDisabled && status != model.StatusVoided {
		return errors.New("invalid card status")
	}
	return s.db.Model(&model.CardCode{}).Where("id = ?", id).Update("status", status).Error
}

func (s *CardService) DeleteCard(id uint) error {
	var activeCount int64
	if err := s.db.Model(&model.ReceiveOrder{}).
		Where("card_code_id = ? AND status IN ?", id, []string{model.OrderCreated, model.OrderActive, model.OrderCancelRequested}).
		Count(&activeCount).Error; err != nil {
		return err
	}
	if activeCount > 0 {
		return errors.New("card has unfinished orders")
	}
	return s.db.Delete(&model.CardCode{}, id).Error
}

func (s *CardService) RevealCode(id uint) (string, error) {
	var card model.CardCode
	if err := s.db.First(&card, id).Error; err != nil {
		return "", err
	}
	if card.CodeCipher == "" {
		return "", errors.New("plain card code is unavailable for this record")
	}
	return util.DecryptString(s.encryptionKey, card.CodeCipher)
}

func (s *CardService) ExportBatch(id uint) (string, error) {
	var batch model.CardBatch
	if err := s.db.First(&batch, id).Error; err != nil {
		return "", err
	}
	content, err := os.ReadFile(batch.ExportPath)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if batch.ExportedAt == nil {
		_ = s.db.Model(&batch).Update("exported_at", now).Error
	}
	return string(content), nil
}

func (s *CardService) ListBatches(limit, offset int) ([]model.CardBatch, error) {
	var batches []model.CardBatch
	err := s.db.Order("id desc").Limit(limit).Offset(offset).Find(&batches).Error
	return batches, err
}

func (s *CardService) DeleteBatch(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var activeCount int64
		if err := tx.Model(&model.ReceiveOrder{}).
			Joins("JOIN sys_card_codes ON sys_card_codes.id = sys_receive_orders.card_code_id").
			Where("sys_card_codes.batch_id = ? AND sys_receive_orders.status IN ?", id, []string{model.OrderCreated, model.OrderActive, model.OrderCancelRequested}).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return errors.New("batch has unfinished orders")
		}
		if err := tx.Where("batch_id = ?", id).Delete(&model.CardCode{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.CardBatch{}, id).Error
	})
}

func (s *CardService) findUsableCard(code string) (*model.CardCode, error) {
	var card model.CardCode
	if err := s.db.Preload("ServiceConfig").Where("code_hash = ?", util.HashCardCode(code)).First(&card).Error; err != nil {
		return nil, errors.New("card code not found")
	}
	if card.Status != model.StatusEnabled {
		return nil, errors.New("card code is not enabled")
	}
	if card.ExpiresAt != nil && time.Now().After(*card.ExpiresAt) {
		return nil, errors.New("card code has expired")
	}
	if card.RemainingUses <= 0 {
		return nil, errors.New("card code has no remaining uses")
	}
	if card.ServiceConfig.Status != model.StatusEnabled {
		return nil, errors.New("service is disabled")
	}
	return &card, nil
}
