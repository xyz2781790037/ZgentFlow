package interfaces

import (
	"context"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// SystemSettingRepository is the storage layer for the platform-wide
// system_settings table. All methods are system-scoped — there is no
// tenant_id; rows are global to the deployment.
type SystemSettingRepository interface {
	// Get fetches a row by key. Returns (nil, nil) when the key is not
	// present — callers fall back to ENV / default at the service layer.
	Get(ctx context.Context, key string) (*types.SystemSetting, error)
	// List returns every row. Used by the management UI to render the
	// settings page; no pagination yet (the registry is small — single
	// digits in P1, expected to stay double-digits long-term).
	List(ctx context.Context) ([]*types.SystemSetting, error)
}

// SystemSettingService exposes the read-only three-tier resolver used by
// production code paths that consume settings.
//
// 3-tier resolver priority: DB > ENV > built-in default. The service
// owns the registry of legal keys; reading or writing an unknown key
// returns an error from Update / falls through to default for Get.
type SystemSettingService interface {
	// GetInt returns the resolved int64 value for `key`.
	//
	// envName is the legacy environment-variable name to consult when
	// the DB row is absent ("" means the key has no ENV fallback).
	// def is the built-in default used when both DB and ENV miss.
	//
	// Errors at the DB layer degrade gracefully: the function logs a
	// warning and falls through to ENV / default rather than returning
	// an error to upstream business code (which would have to bubble
	// it through every caller — we'd rather mis-serve a request with
	// the default than 500 a file upload).
	GetInt(ctx context.Context, key string, envName string, def int64) int64
	GetString(ctx context.Context, key string, envName string, def string) string
}
