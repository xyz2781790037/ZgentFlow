package interfaces

import (
	"context"
	"errors"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

var ErrUserNotFound = errors.New("user not found")

// UserService defines the user service interface
type UserService interface {
	ResolveLocalWorkspace(ctx context.Context) (*types.User, *types.Tenant, error)
}

// UserRepository defines the user repository interface
type UserRepository interface {
	// CreateUser creates a user
	CreateUser(ctx context.Context, user *types.User) error
	// CreateUserWithTenant atomically creates a personal tenant and its owner.
	CreateUserWithTenant(ctx context.Context, user *types.User, tenant *types.Tenant) error
	// GetUserByEmail gets a user by email
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// GetUserByUsername gets a user by username, case-insensitively.
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
	// GetUserByID gets a user by its stable identifier.
	GetUserByID(ctx context.Context, id string) (*types.User, error)
}
