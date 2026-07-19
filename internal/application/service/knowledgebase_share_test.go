package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newShareTestService(t *testing.T) (*KnowledgeBaseShareService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	ddl := []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, username TEXT, tenant_id INTEGER, deleted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE tenants (id INTEGER PRIMARY KEY, name TEXT, status TEXT, storage_quota INTEGER, storage_used INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE knowledge_bases (id TEXT PRIMARY KEY, name TEXT, type TEXT, is_temporary BOOLEAN, tenant_id INTEGER, owner_user_id TEXT, sharing_enabled BOOLEAN, invite_code TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE knowledge_base_members (id INTEGER PRIMARY KEY AUTOINCREMENT, knowledge_base_id TEXT, user_id TEXT, role TEXT, joined_at DATETIME, created_at DATETIME, updated_at DATETIME, UNIQUE(knowledge_base_id, user_id))`,
		`CREATE TABLE knowledge_base_join_requests (id TEXT PRIMARY KEY, knowledge_base_id TEXT, user_id TEXT, status TEXT, reviewed_by TEXT, reviewed_at DATETIME, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE knowledge_base_audit_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, knowledge_base_id TEXT, actor_user_id TEXT, action TEXT, target_user_id TEXT, target_resource_id TEXT, details BLOB, created_at DATETIME)`,
		`CREATE TABLE user_kb_pins (tenant_id INTEGER, user_id TEXT, kb_id TEXT, pinned_at DATETIME)`,
	}
	for _, stmt := range ddl {
		require.NoError(t, db.Exec(stmt).Error)
	}
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO tenants(id,name,status,created_at,updated_at) VALUES (1,'owner','active',?,?),(2,'reader','active',?,?),(3,'admin','active',?,?)`, now, now, now, now, now, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO users(id,username,tenant_id,created_at) VALUES ('owner','owner',1,?),('reader','reader',2,?),('admin','admin',3,?)`, now, now, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO knowledge_bases(id,name,type,is_temporary,tenant_id,owner_user_id,sharing_enabled,invite_code,created_at,updated_at) VALUES ('kb','Shared KB','document',0,1,'owner',1,'INVITE123',?,?)`, now, now).Error)
	return NewKnowledgeBaseShareService(db), db
}

func shareTestContext(userID string, tenantID uint64) context.Context {
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, userID)
	return context.WithValue(ctx, types.TenantIDContextKey, tenantID)
}

func TestKnowledgeBaseShareAccessIsScopedByMembershipRole(t *testing.T) {
	svc, db := newShareTestService(t)
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO knowledge_base_members(knowledge_base_id,user_id,role,joined_at,created_at,updated_at) VALUES ('kb','reader','reader',?,?,?)`, now, now, now).Error)

	kb, access, err := svc.ResolveAccess(shareTestContext("reader", 2), "kb", types.KBRoleReader)
	require.NoError(t, err)
	require.Equal(t, uint64(1), kb.TenantID)
	require.Equal(t, types.KBRoleReader, access.Role)

	_, _, err = svc.ResolveAccess(shareTestContext("reader", 2), "kb", types.KBRoleWriter)
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)

	effective := svc.WithStorageTenant(shareTestContext("reader", 2), kb, access.Role)
	storageTenant, _ := types.TenantIDFromContext(effective)
	callerTenant, _ := types.CallerTenantIDFromContext(effective)
	require.Equal(t, uint64(1), storageTenant)
	require.Equal(t, uint64(2), callerTenant)
}

func TestKnowledgeBaseJoinApprovalDefaultsToReader(t *testing.T) {
	svc, db := newShareTestService(t)
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO knowledge_base_members(knowledge_base_id,user_id,role,joined_at,created_at,updated_at) VALUES ('kb','admin','admin',?,?,?)`, now, now, now).Error)

	request, err := svc.SubmitJoinRequest(shareTestContext("reader", 2), "INVITE123")
	require.NoError(t, err)
	require.Equal(t, types.KBJoinPending, request.Status)

	require.NoError(t, svc.ReviewJoinRequest(shareTestContext("admin", 3), "kb", request.ID, types.KBJoinApproved))
	var member types.KnowledgeBaseMember
	require.NoError(t, db.Where("knowledge_base_id = ? AND user_id = ?", "kb", "reader").First(&member).Error)
	require.Equal(t, types.KBRoleReader, member.Role)
}

func TestAdminCanToggleWriteButCannotGrantAdmin(t *testing.T) {
	svc, db := newShareTestService(t)
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO knowledge_base_members(knowledge_base_id,user_id,role,joined_at,created_at,updated_at) VALUES ('kb','admin','admin',?,?,?),('kb','reader','reader',?,?,?)`, now, now, now, now, now, now).Error)

	require.NoError(t, svc.UpdateMemberRole(shareTestContext("admin", 3), "kb", "reader", types.KBRoleWriter))
	var member types.KnowledgeBaseMember
	require.NoError(t, db.Where("knowledge_base_id = ? AND user_id = ?", "kb", "reader").First(&member).Error)
	require.Equal(t, types.KBRoleWriter, member.Role)

	err := svc.UpdateMemberRole(shareTestContext("admin", 3), "kb", "reader", types.KBRoleAdmin)
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)
}

func TestDisablingInvitationKeepsExistingMember(t *testing.T) {
	svc, db := newShareTestService(t)
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO knowledge_base_members(knowledge_base_id,user_id,role,joined_at,created_at,updated_at) VALUES ('kb','reader','reader',?,?,?)`, now, now, now).Error)

	kb, err := svc.UpdateInvitation(shareTestContext("owner", 1), "kb", false, false)
	require.NoError(t, err)
	require.False(t, kb.SharingEnabled)
	_, _, err = svc.ResolveAccess(shareTestContext("reader", 2), "kb", types.KBRoleReader)
	require.NoError(t, err)
}
