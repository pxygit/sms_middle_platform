package service

import (
	"errors"
	"time"

	"sms-middle-platform/backend/internal/auth"
	"sms-middle-platform/backend/internal/config"
	"sms-middle-platform/backend/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminService struct {
	db  *gorm.DB
	cfg config.Config
}

type LoginResult struct {
	Token string      `json:"token"`
	Admin model.Admin `json:"admin"`
}

func NewAdminService(db *gorm.DB, cfg config.Config) *AdminService {
	return &AdminService{db: db, cfg: cfg}
}

func (s *AdminService) Login(username, password string) (*LoginResult, error) {
	var admin model.Admin
	if err := s.db.Where("username = ?", username).First(&admin).Error; err != nil {
		return nil, errors.New("invalid username or password")
	}
	if admin.Status != model.StatusEnabled {
		return nil, errors.New("admin account is disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}
	now := time.Now()
	if err := s.db.Model(&admin).Update("last_login_at", now).Error; err != nil {
		return nil, err
	}
	admin.LastLoginAt = &now
	token, err := auth.GenerateToken(s.cfg.JWTSecret, s.cfg.JWTExpireHours, admin.ID, admin.Username, admin.Role)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, Admin: admin}, nil
}
