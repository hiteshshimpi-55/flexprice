package settings

import (
	"context"
)

// Repository defines the interface for settings persistence operations
type Repository interface {
	// Core operations
	Create(ctx context.Context, setting *Setting) error
	Get(ctx context.Context, key string) (*Setting, error)
	GetByID(ctx context.Context, id string) (*Setting, error)
	Update(ctx context.Context, setting *Setting) error
	Delete(ctx context.Context, id string) error

	// Operations by key
	GetByKey(ctx context.Context, key string) (*Setting, error)
	DeleteByKey(ctx context.Context, key string) error
	UpsertByKey(ctx context.Context, setting *Setting) error
}
