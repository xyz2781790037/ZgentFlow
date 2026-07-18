package service

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"github.com/xyz2781790037/ZealRAG/internal/utils"
)

// Earlier migrations seeded these values without marking them as operator
// overrides. Treat unchanged seed rows as placeholders so environment values
// keep their original precedence.
var bootstrapSettingDefaults = map[string]types.JSON{
	"ssrf.whitelist":                  types.JSON(`[]`),
	"auth.registration_mode":          types.JSON(`"self_serve"`),
	"tenant.max_owned_per_user":       types.JSON(`10`),
	"tenant.default_storage_quota_gb": types.JSON(`10`),
	"asynq.concurrency":               types.JSON(`32`),
}

type systemSettingService struct {
	repo interfaces.SystemSettingRepository

	mu     sync.RWMutex
	cache  map[string]*types.SystemSetting
	loaded atomic.Bool
}

func NewSystemSettingService(repo interfaces.SystemSettingRepository) interfaces.SystemSettingService {
	service := &systemSettingService{
		repo:  repo,
		cache: make(map[string]*types.SystemSetting),
	}
	go service.preload(context.Background())
	return service
}

func (s *systemSettingService) preload(ctx context.Context) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		logger.Warnf(ctx, "[system_settings] preload failed, using database fallback: %v", err)
		return
	}
	s.mu.Lock()
	for _, row := range rows {
		if row != nil {
			s.cache[row.Key] = row
		}
	}
	s.mu.Unlock()
	s.loaded.Store(true)
	s.applySSRFWhitelist(ctx)
}

func (s *systemSettingService) resolveRaw(ctx context.Context, key string) (types.JSON, bool) {
	if s.loaded.Load() {
		s.mu.RLock()
		row := s.cache[key]
		s.mu.RUnlock()
		if row == nil || isBootstrapSetting(row) {
			return nil, false
		}
		return row.Value, true
	}

	row, err := s.repo.Get(ctx, key)
	if err != nil {
		logger.Warnf(ctx, "[system_settings] resolve %q failed, using fallback: %v", key, err)
		return nil, false
	}
	if row == nil || isBootstrapSetting(row) {
		return nil, false
	}
	return row.Value, true
}

func isBootstrapSetting(row *types.SystemSetting) bool {
	if row == nil || strings.TrimSpace(row.LastModifiedBy) != "" {
		return false
	}
	defaultValue, ok := bootstrapSettingDefaults[row.Key]
	if !ok {
		return false
	}
	var actual, expected bytes.Buffer
	if json.Compact(&actual, row.Value) != nil || json.Compact(&expected, defaultValue) != nil {
		return false
	}
	return bytes.Equal(actual.Bytes(), expected.Bytes())
}

func (s *systemSettingService) GetInt(ctx context.Context, key, envName string, fallback int64) int64 {
	if raw, ok := s.resolveRaw(ctx, key); ok {
		var value int64
		if json.Unmarshal(raw, &value) == nil {
			return value
		}
		var quoted string
		if json.Unmarshal(raw, &quoted) == nil {
			if value, err := strconv.ParseInt(quoted, 10, 64); err == nil {
				return value
			}
		}
		logger.Warnf(ctx, "[system_settings] %q is not an integer, using fallback", key)
	}
	if envName != "" {
		if value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(envName)), 10, 64); err == nil {
			return value
		}
	}
	return fallback
}

func (s *systemSettingService) GetString(ctx context.Context, key, envName, fallback string) string {
	if raw, ok := s.resolveRaw(ctx, key); ok {
		var value string
		if json.Unmarshal(raw, &value) == nil {
			return value
		}
		logger.Warnf(ctx, "[system_settings] %q is not a string, using fallback", key)
	}
	if envName != "" {
		if value := os.Getenv(envName); value != "" {
			return value
		}
	}
	return fallback
}

func (s *systemSettingService) getStringList(ctx context.Context, key, envName string, fallback []string) []string {
	if raw, ok := s.resolveRaw(ctx, key); ok {
		var value []string
		if json.Unmarshal(raw, &value) == nil {
			if value == nil {
				return []string{}
			}
			return value
		}
		logger.Warnf(ctx, "[system_settings] %q is not a string list, using fallback", key)
	}
	if envName != "" {
		if raw := os.Getenv(envName); raw != "" {
			values := make([]string, 0, 4)
			for _, entry := range strings.Split(raw, ",") {
				if entry = strings.TrimSpace(entry); entry != "" {
					values = append(values, entry)
				}
			}
			return values
		}
	}
	if fallback == nil {
		return []string{}
	}
	return fallback
}

func (s *systemSettingService) applySSRFWhitelist(ctx context.Context) {
	primary := strings.Join(s.getStringList(ctx, "ssrf.whitelist", "SSRF_WHITELIST", nil), ",")
	extra := strings.TrimSpace(os.Getenv("SSRF_WHITELIST_EXTRA"))
	if primary == "" {
		primary = extra
	} else if extra != "" {
		primary += "," + extra
	}
	utils.SetSSRFWhitelistFromRaw(primary)
}
