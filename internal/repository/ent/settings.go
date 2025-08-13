package ent

import (
	"context"
	"strings"
	"time"

	"github.com/flexprice/flexprice/ent"
	"github.com/flexprice/flexprice/ent/settings"
	"github.com/flexprice/flexprice/internal/cache"
	domainSettings "github.com/flexprice/flexprice/internal/domain/settings"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/lib/pq"
)

type settingsRepository struct {
	client postgres.IClient
	log    *logger.Logger
	cache  cache.Cache
}

func NewSettingsRepository(client postgres.IClient, log *logger.Logger, cache cache.Cache) domainSettings.Repository {
	return &settingsRepository{
		client: client,
		log:    log,
		cache:  cache,
	}
}

func (r *settingsRepository) Create(ctx context.Context, s *domainSettings.Setting) error {
	client := r.client.Querier(ctx)

	r.log.Debugw("creating setting",
		"setting_id", s.ID,
		"tenant_id", s.TenantID,
		"key", s.Key,
	)

	// Set environment ID from context if not already set
	if s.EnvironmentID == "" {
		s.EnvironmentID = types.GetEnvironmentID(ctx)
	}

	setting, err := client.Settings.Create().
		SetID(s.ID).
		SetTenantID(s.TenantID).
		SetKey(s.Key).
		SetValue(s.Value).
		SetStatus(string(s.Status)).
		SetCreatedAt(s.CreatedAt).
		SetUpdatedAt(s.UpdatedAt).
		SetCreatedBy(s.CreatedBy).
		SetUpdatedBy(s.UpdatedBy).
		SetEnvironmentID(s.EnvironmentID).
		Save(ctx)

	if err != nil {
		if ent.IsConstraintError(err) {
			if pqErr, ok := err.(*pq.Error); ok {
				if strings.Contains(pqErr.Message, "tenant_id_environment_id_key") {
					return ierr.WithError(err).
						WithHint("A setting with this key already exists for this tenant and environment").
						WithReportableDetails(map[string]any{
							"key": s.Key,
						}).
						Mark(ierr.ErrAlreadyExists)
				}
			}
			return ierr.WithError(err).
				WithHint("Failed to create setting").
				WithReportableDetails(map[string]any{
					"key": s.Key,
				}).
				Mark(ierr.ErrAlreadyExists)
		}
		return ierr.WithError(err).
			WithHint("Failed to create setting").
			Mark(ierr.ErrDatabase)
	}

	*s = *domainSettings.FromEnt(setting)
	return nil
}

func (r *settingsRepository) Update(ctx context.Context, s *domainSettings.Setting) error {
	client := r.client.Querier(ctx)

	r.log.Debugw("updating setting",
		"setting_id", s.ID,
		"tenant_id", s.TenantID,
		"key", s.Key,
	)

	_, err := client.Settings.Update().
		Where(
			settings.ID(s.ID),
			settings.TenantID(s.TenantID),
			settings.EnvironmentID(types.GetEnvironmentID(ctx)),
		).
		SetValue(s.Value).
		SetUpdatedAt(time.Now().UTC()).
		SetUpdatedBy(types.GetUserID(ctx)).
		Save(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return ierr.WithError(err).
				WithHintf("Setting with ID %s was not found", s.ID).
				WithReportableDetails(map[string]any{
					"setting_id": s.ID,
				}).
				Mark(ierr.ErrNotFound)
		}
		return ierr.WithError(err).
			WithHint("Failed to update setting").
			Mark(ierr.ErrDatabase)
	}

	r.DeleteCache(ctx, s)
	return nil
}

func (r *settingsRepository) Delete(ctx context.Context, id string) error {
	client := r.client.Querier(ctx)

	r.log.Debugw("deleting setting",
		"setting_id", id,
		"tenant_id", types.GetTenantID(ctx),
		"environment_id", types.GetEnvironmentID(ctx),
	)

	_, err := client.Settings.Update().
		Where(
			settings.ID(id),
			settings.TenantID(types.GetTenantID(ctx)),
			settings.EnvironmentID(types.GetEnvironmentID(ctx)),
		).
		SetStatus(string(types.StatusArchived)).
		SetUpdatedAt(time.Now().UTC()).
		SetUpdatedBy(types.GetUserID(ctx)).
		Save(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return ierr.WithError(err).
				WithHintf("Setting with ID %s was not found", id).
				WithReportableDetails(map[string]any{
					"setting_id": id,
				}).
				Mark(ierr.ErrNotFound)
		}
		return ierr.WithError(err).
			WithHint("Failed to delete setting").
			Mark(ierr.ErrDatabase)
	}

	return nil
}

func (r *settingsRepository) Get(ctx context.Context, key string) (*domainSettings.Setting, error) {
	// Try to get from cache first
	if cachedSetting := r.GetCache(ctx, key); cachedSetting != nil {
		return cachedSetting, nil
	}

	client := r.client.Querier(ctx)
	r.log.Debugw("getting setting", "key", key)

	s, err := client.Settings.Query().
		Where(
			settings.Key(key),
			settings.TenantID(types.GetTenantID(ctx)),
			settings.EnvironmentID(types.GetEnvironmentID(ctx)),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ierr.WithError(err).
				WithHintf("Setting with key %s was not found", key).
				WithReportableDetails(map[string]any{
					"key": key,
				}).
				Mark(ierr.ErrNotFound)
		}
		return nil, ierr.WithError(err).
			WithHint("Failed to get setting").
			Mark(ierr.ErrDatabase)
	}

	setting := domainSettings.FromEnt(s)

	// Set cache
	r.SetCache(ctx, setting)
	return setting, nil
}

func (r *settingsRepository) GetByID(ctx context.Context, id string) (*domainSettings.Setting, error) {
	client := r.client.Querier(ctx)
	r.log.Debugw("getting setting by ID", "setting_id", id)

	s, err := client.Settings.Query().
		Where(
			settings.ID(id),
			settings.TenantID(types.GetTenantID(ctx)),
			settings.EnvironmentID(types.GetEnvironmentID(ctx)),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ierr.WithError(err).
				WithHintf("Setting with ID %s was not found", id).
				WithReportableDetails(map[string]any{
					"setting_id": id,
				}).
				Mark(ierr.ErrNotFound)
		}
		return nil, ierr.WithError(err).
			WithHint("Failed to get setting").
			Mark(ierr.ErrDatabase)
	}

	return domainSettings.FromEnt(s), nil
}

func (r *settingsRepository) GetByKey(ctx context.Context, key string) (*domainSettings.Setting, error) {
	// Try to get from cache first
	if cachedSetting := r.GetCache(ctx, key); cachedSetting != nil {
		return cachedSetting, nil
	}

	client := r.client.Querier(ctx)
	r.log.Debugw("getting setting by key", "key", key)

	s, err := client.Settings.Query().
		Where(
			settings.Key(key),
			settings.TenantID(types.GetTenantID(ctx)),
			settings.EnvironmentID(types.GetEnvironmentID(ctx)),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ierr.WithError(err).
				WithHintf("Setting with key %s was not found", key).
				WithReportableDetails(map[string]any{
					"key": key,
				}).
				Mark(ierr.ErrNotFound)
		}
		return nil, ierr.WithError(err).
			WithHint("Failed to get setting by key").
			Mark(ierr.ErrDatabase)
	}

	setting := domainSettings.FromEnt(s)

	// Set cache
	r.SetCache(ctx, setting)
	return setting, nil
}

func (r *settingsRepository) UpsertByKey(ctx context.Context, s *domainSettings.Setting) error {
	client := r.client.Querier(ctx)

	r.log.Debugw("upserting setting",
		"tenant_id", s.TenantID,
		"key", s.Key,
	)

	// Set environment ID from context if not already set
	if s.EnvironmentID == "" {
		s.EnvironmentID = types.GetEnvironmentID(ctx)
	}

	// Try to find existing setting
	existing, err := client.Settings.Query().
		Where(
			settings.Key(s.Key),
			settings.TenantID(s.TenantID),
			settings.EnvironmentID(s.EnvironmentID),
		).
		Only(ctx)

	if err != nil && !ent.IsNotFound(err) {
		return ierr.WithError(err).
			WithHint("Failed to check existing setting").
			Mark(ierr.ErrDatabase)
	}

	if existing != nil {
		// Update existing
		s.ID = existing.ID
		return r.Update(ctx, s)
	} else {
		// Create new
		return r.Create(ctx, s)
	}
}

func (r *settingsRepository) DeleteByKey(ctx context.Context, key string) error {
	// Get the setting first for cache invalidation
	setting, err := r.GetByKey(ctx, key)
	if err != nil {
		return err
	}

	client := r.client.Querier(ctx)

	r.log.Debugw("deleting setting by key", "key", key)

	_, err = client.Settings.Update().
		Where(
			settings.Key(key),
			settings.TenantID(types.GetTenantID(ctx)),
			settings.EnvironmentID(types.GetEnvironmentID(ctx)),
		).
		SetStatus(string(types.StatusArchived)).
		SetUpdatedAt(time.Now().UTC()).
		SetUpdatedBy(types.GetUserID(ctx)).
		Save(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return ierr.WithError(err).
				WithHintf("Setting with key %s was not found", key).
				WithReportableDetails(map[string]any{
					"key": key,
				}).
				Mark(ierr.ErrNotFound)
		}
		return ierr.WithError(err).
			WithHint("Failed to delete setting by key").
			Mark(ierr.ErrDatabase)
	}

	// Delete from cache
	r.DeleteCache(ctx, setting)
	return nil
}

func (r *settingsRepository) SetCache(ctx context.Context, setting *domainSettings.Setting) {
	span := cache.StartCacheSpan(ctx, "settings", "set", map[string]interface{}{
		"setting_id": setting.ID,
		"key":        setting.Key,
	})
	defer cache.FinishSpan(span)

	tenantID := types.GetTenantID(ctx)
	environmentID := types.GetEnvironmentID(ctx)

	// Set both ID and key based cache entries
	idKey := cache.GenerateKey(cache.PrefixSettings, tenantID, environmentID, setting.ID)
	keyKey := cache.GenerateKey(cache.PrefixSettings, tenantID, environmentID, setting.Key)

	r.cache.Set(ctx, idKey, setting, cache.ExpiryDefaultInMemory)
	r.cache.Set(ctx, keyKey, setting, cache.ExpiryDefaultInMemory)

	r.log.Debugw("cache set", "id_key", idKey, "key_key", keyKey)
}

func (r *settingsRepository) GetCache(ctx context.Context, key string) *domainSettings.Setting {
	span := cache.StartCacheSpan(ctx, "settings", "get", map[string]interface{}{
		"key": key,
	})
	defer cache.FinishSpan(span)

	cacheKey := cache.GenerateKey(cache.PrefixSettings, types.GetTenantID(ctx), types.GetEnvironmentID(ctx), key)
	if value, found := r.cache.Get(ctx, cacheKey); found {
		if setting, ok := value.(*domainSettings.Setting); ok {
			r.log.Debugw("cache hit", "key", cacheKey)
			return setting
		}
	}
	return nil
}

func (r *settingsRepository) DeleteCache(ctx context.Context, setting *domainSettings.Setting) {
	span := cache.StartCacheSpan(ctx, "settings", "delete", map[string]interface{}{
		"setting_id": setting.ID,
		"key":        setting.Key,
	})
	defer cache.FinishSpan(span)

	tenantID := types.GetTenantID(ctx)
	environmentID := types.GetEnvironmentID(ctx)

	// Delete both ID and key based cache entries
	idKey := cache.GenerateKey(cache.PrefixSettings, tenantID, environmentID, setting.ID)
	keyKey := cache.GenerateKey(cache.PrefixSettings, tenantID, environmentID, setting.Key)
	r.cache.Delete(ctx, idKey)
	r.cache.Delete(ctx, keyKey)
	r.log.Debugw("cache deleted", "id_key", idKey, "key_key", keyKey)
}
