package service

import (
	"errors"
	"strings"
	"time"

	"sms-middle-platform/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AnnouncementService struct {
	db *gorm.DB
}

type AnnouncementInput struct {
	Title      string     `json:"title" binding:"required"`
	Content    string     `json:"content" binding:"required"`
	Status     string     `json:"status"`
	NotifyMode string     `json:"notifyMode"`
	StartAt    *time.Time `json:"startAt"`
	EndAt      *time.Time `json:"endAt"`
}

type AnnouncementListFilter struct {
	Keyword    string
	Status     string
	NotifyMode string
	Limit      int
	Offset     int
}

type PublicAnnouncement struct {
	model.Announcement
	Unread bool `json:"unread"`
}

func NewAnnouncementService(db *gorm.DB) *AnnouncementService {
	return &AnnouncementService{db: db}
}

func (s *AnnouncementService) List(filter AnnouncementListFilter) ([]model.Announcement, error) {
	query := s.db.Model(&model.Announcement{}).Order("id desc")
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		query = query.Where("title ILIKE ?", "%"+keyword+"%")
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.NotifyMode != "" {
		query = query.Where("notify_mode = ?", filter.NotifyMode)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	var items []model.Announcement
	return items, query.Find(&items).Error
}

func (s *AnnouncementService) Create(input AnnouncementInput, adminID uint) (*model.Announcement, error) {
	if err := normalizeAnnouncementInput(&input); err != nil {
		return nil, err
	}
	item := model.Announcement{
		Title:      strings.TrimSpace(input.Title),
		Content:    strings.TrimSpace(input.Content),
		Status:     input.Status,
		NotifyMode: input.NotifyMode,
		StartAt:    input.StartAt,
		EndAt:      input.EndAt,
		CreatedBy:  adminID,
	}
	if item.Status == model.AnnouncementActive {
		now := time.Now()
		item.PublishedAt = &now
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AnnouncementService) Update(id uint, input AnnouncementInput) (*model.Announcement, error) {
	if err := normalizeAnnouncementInput(&input); err != nil {
		return nil, err
	}
	var item model.Announcement
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"title":       strings.TrimSpace(input.Title),
		"content":     strings.TrimSpace(input.Content),
		"status":      input.Status,
		"notify_mode": input.NotifyMode,
		"start_at":    input.StartAt,
		"end_at":      input.EndAt,
	}
	if item.Status != model.AnnouncementActive && input.Status == model.AnnouncementActive && item.PublishedAt == nil {
		now := time.Now()
		updates["published_at"] = &now
	}
	if err := s.db.Model(&item).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &item, s.db.First(&item, id).Error
}

func (s *AnnouncementService) Delete(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("announcement_id = ?", id).Delete(&model.AnnouncementRead{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Announcement{}, id).Error
	})
}

func (s *AnnouncementService) PublicList(readerID string) ([]PublicAnnouncement, error) {
	now := time.Now()
	var items []model.Announcement
	if err := visibleAnnouncementQuery(s.db, now).
		Order("COALESCE(published_at, created_at) desc, id desc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	readMap := map[uint]bool{}
	if strings.TrimSpace(readerID) != "" && len(items) > 0 {
		ids := make([]uint, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		var reads []model.AnnouncementRead
		if err := s.db.Where("reader_id = ? AND announcement_id IN ?", readerID, ids).Find(&reads).Error; err != nil {
			return nil, err
		}
		for _, read := range reads {
			readMap[read.AnnouncementID] = true
		}
	}
	out := make([]PublicAnnouncement, 0, len(items))
	for _, item := range items {
		out = append(out, PublicAnnouncement{Announcement: item, Unread: !readMap[item.ID]})
	}
	return out, nil
}

func (s *AnnouncementService) MarkRead(id uint, readerID, ip, userAgent string) error {
	readerID = strings.TrimSpace(readerID)
	if readerID == "" {
		return errors.New("reader id is required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var item model.Announcement
		if err := visibleAnnouncementQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"}), now).First(&item, id).Error; err != nil {
			return err
		}
		read := model.AnnouncementRead{
			AnnouncementID: id,
			ReaderID:       readerID,
			IP:             ip,
			UserAgent:      userAgent,
			ReadAt:         now,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&read)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&model.Announcement{}).Where("id = ?", id).Update("read_count", gorm.Expr("read_count + 1")).Error
	})
}

func visibleAnnouncementQuery(db *gorm.DB, now time.Time) *gorm.DB {
	return db.Where("status = ?", model.AnnouncementActive).
		Where("(start_at IS NULL OR start_at <= ?)", now).
		Where("(end_at IS NULL OR end_at >= ?)", now)
}

func normalizeAnnouncementInput(input *AnnouncementInput) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" {
		return errors.New("announcement title is required")
	}
	if input.Content == "" {
		return errors.New("announcement content is required")
	}
	if input.Status == "" {
		input.Status = model.AnnouncementDraft
	}
	if input.NotifyMode == "" {
		input.NotifyMode = model.AnnouncementNotifySilent
	}
	if !validAnnouncementStatus(input.Status) {
		return errors.New("invalid announcement status")
	}
	if input.NotifyMode != model.AnnouncementNotifyModal && input.NotifyMode != model.AnnouncementNotifySilent {
		return errors.New("invalid announcement notify mode")
	}
	if input.StartAt != nil && input.EndAt != nil && input.EndAt.Before(*input.StartAt) {
		return errors.New("announcement end time must be after start time")
	}
	return nil
}

func validAnnouncementStatus(status string) bool {
	switch status {
	case model.AnnouncementDraft, model.AnnouncementActive, model.AnnouncementArchived:
		return true
	default:
		return false
	}
}
