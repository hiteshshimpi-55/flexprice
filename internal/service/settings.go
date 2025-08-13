package service

import (
	"context"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/domain/settings"
	"github.com/flexprice/flexprice/internal/types"
)

// SettingsService defines the interface for managing settings operations
type SettingsService interface {

	// Key-based operations
	GetSettingByKey(ctx context.Context, key string) (*dto.SettingResponse, error)
	UpdateSettingByKey(ctx context.Context, key string, req *dto.UpdateSettingRequest) (*dto.SettingResponse, error)
	DeleteSettingByKey(ctx context.Context, key string) error
}

type settingsService struct {
	repo settings.Repository
}

func NewSettingsService(repo settings.Repository) SettingsService {
	return &settingsService{
		repo: repo,
	}
}

func (s *settingsService) GetSettingByKey(ctx context.Context, key string) (*dto.SettingResponse, error) {
	setting, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return dto.SettingFromDomain(setting), nil
}

func (s *settingsService) UpdateSettingByKey(ctx context.Context, key string, req *dto.UpdateSettingRequest) (*dto.SettingResponse, error) {
	// Get existing setting by key
	setting, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Value != nil {
		setting.Value = *req.Value
	}

	setting.UpdatedAt = time.Now().UTC()
	setting.UpdatedBy = types.GetUserID(ctx)

	// Update in repository
	err = s.repo.Update(ctx, setting)
	if err != nil {
		return nil, err
	}

	return dto.SettingFromDomain(setting), nil
}

func (s *settingsService) DeleteSettingByKey(ctx context.Context, key string) error {
	err := s.repo.Delete(ctx, key)
	if err != nil {
		return err
	}
	return nil
}
