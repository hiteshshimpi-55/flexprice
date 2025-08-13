package service

import (
	"context"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/domain/settings"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/types"
)

// SettingsService defines the interface for managing settings operations
type SettingsService interface {
	// CRUD Operations
	CreateSetting(ctx context.Context, req *dto.CreateSettingRequest) (*dto.SettingResponse, error)
	GetSetting(ctx context.Context, key string) (*dto.SettingResponse, error)
	GetSettingByID(ctx context.Context, id string) (*dto.SettingResponse, error)
	UpdateSetting(ctx context.Context, req *dto.UpdateSettingRequest) (*dto.SettingResponse, error)
	DeleteSetting(ctx context.Context, id string) error

	// Key-based operations
	GetSettingByKey(ctx context.Context, key string) (*dto.SettingResponse, error)
	UpdateSettingByKey(ctx context.Context, key string, req *dto.UpdateSettingRequest) (*dto.SettingResponse, error)
}

type settingsService struct {
	repo settings.Repository
}

func NewSettingsService(repo settings.Repository) SettingsService {
	return &settingsService{
		repo: repo,
	}
}

func (s *settingsService) CreateSetting(ctx context.Context, req *dto.CreateSettingRequest) (*dto.SettingResponse, error) {
	// Validate the request
	if req.Key == "" {
		return nil, ierr.NewError("setting key is required").
			Mark(ierr.ErrValidation)
	}

	// Create domain model
	setting := &settings.Setting{
		ID:            types.GenerateUUIDWithPrefix(types.UUID_PREFIX_SETTINGS),
		Key:           req.Key,
		Value:         req.Value,
		EnvironmentID: types.GetEnvironmentID(ctx),
		BaseModel: types.BaseModel{
			TenantID:  types.GetTenantID(ctx),
			Status:    types.StatusPublished,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			CreatedBy: types.GetUserID(ctx),
			UpdatedBy: types.GetUserID(ctx),
		},
	}

	// Validate setting
	if err := setting.Validate(); err != nil {
		return nil, err
	}

	// Create in repository
	err := s.repo.Create(ctx, setting)
	if err != nil {
		return nil, err
	}

	return dto.SettingFromDomain(setting), nil
}

func (s *settingsService) GetSetting(ctx context.Context, key string) (*dto.SettingResponse, error) {
	setting, err := s.repo.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	return dto.SettingFromDomain(setting), nil
}

func (s *settingsService) GetSettingByID(ctx context.Context, id string) (*dto.SettingResponse, error) {
	setting, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return dto.SettingFromDomain(setting), nil
}

func (s *settingsService) UpdateSetting(ctx context.Context, req *dto.UpdateSettingRequest) (*dto.SettingResponse, error) {
	// Get existing setting
	setting, err := s.repo.GetByID(ctx, req.ID)
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

func (s *settingsService) DeleteSetting(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *settingsService) GetSettingByKey(ctx context.Context, key string) (*dto.SettingResponse, error) {
	setting, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return dto.SettingFromDomain(setting), nil
}

func (s *settingsService) UpdateSettingByKey(ctx context.Context, key string, req *dto.UpdateSettingRequest) (*dto.SettingResponse, error) {
	setting, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return dto.SettingFromDomain(setting), nil
}
