package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KnowledgeBaseShareService owns the small, security-sensitive ACL boundary
// around cross-tenant knowledge-base access.
type KnowledgeBaseShareService struct {
	db *gorm.DB
}

func NewKnowledgeBaseShareService(db *gorm.DB) *KnowledgeBaseShareService {
	return &KnowledgeBaseShareService{db: db}
}

type KBMemberView struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type KBJoinRequestView struct {
	ID                string     `json:"id"`
	KnowledgeBaseID   string     `json:"knowledge_base_id"`
	KnowledgeBaseName string     `json:"knowledge_base_name"`
	UserID            string     `json:"user_id"`
	Username          string     `json:"username"`
	Status            string     `json:"status"`
	ReviewedBy        *string    `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type KBAuditLogView struct {
	ID               uint64     `json:"id"`
	ActorUserID      string     `json:"actor_user_id"`
	ActorUsername    string     `json:"actor_username"`
	Action           string     `json:"action"`
	TargetUserID     string     `json:"target_user_id,omitempty"`
	TargetUsername   string     `json:"target_username,omitempty"`
	TargetResourceID string     `json:"target_resource_id,omitempty"`
	Details          types.JSON `json:"details,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (s *KnowledgeBaseShareService) ResolveAccess(
	ctx context.Context, kbID string, requiredRole string,
) (*types.KnowledgeBase, *types.KnowledgeBaseAccess, error) {
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || userID == "" {
		return nil, nil, apperrors.NewUnauthorizedError("请先登录")
	}
	var kb types.KnowledgeBase
	if err := s.db.WithContext(ctx).Where("id = ?", kbID).First(&kb).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apperrors.NewNotFoundError("知识库不存在")
		}
		return nil, nil, err
	}

	role := ""
	callerTenantID, _ := types.CallerTenantIDFromContext(ctx)
	if kb.OwnerUserID == userID || (kb.OwnerUserID == "" && kb.TenantID == callerTenantID) {
		role = types.KBRoleOwner
	} else {
		var member types.KnowledgeBaseMember
		err := s.db.WithContext(ctx).
			Where("knowledge_base_id = ? AND user_id = ?", kb.ID, userID).
			First(&member).Error
		if err == nil {
			role = member.Role
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
	}
	if types.KBRoleRank(role) < types.KBRoleRank(requiredRole) {
		// Do not disclose whether an arbitrary cross-tenant KB exists.
		if role == "" {
			return nil, nil, apperrors.NewNotFoundError("知识库不存在")
		}
		return nil, nil, apperrors.NewForbiddenError("没有执行此操作的权限")
	}
	ownerUsername := ""
	if kb.OwnerUserID != "" {
		_ = s.db.WithContext(ctx).Model(&types.User{}).
			Select("username").Where("id = ?", kb.OwnerUserID).Scan(&ownerUsername).Error
	}
	access := &types.KnowledgeBaseAccess{
		Role: role, Shared: role != types.KBRoleOwner, OwnerUsername: ownerUsername,
	}
	kb.AccessRole = role
	kb.IsShared = access.Shared
	kb.OwnerUsername = ownerUsername
	return &kb, access, nil
}

func (s *KnowledgeBaseShareService) ResolveKnowledgeAccess(
	ctx context.Context, knowledgeID string, requiredRole string,
) (*types.KnowledgeBase, *types.KnowledgeBaseAccess, *types.Knowledge, error) {
	var knowledge types.Knowledge
	if err := s.db.WithContext(ctx).Where("id = ?", knowledgeID).First(&knowledge).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, apperrors.NewNotFoundError("文档不存在")
		}
		return nil, nil, nil, err
	}
	kb, access, err := s.ResolveAccess(ctx, knowledge.KnowledgeBaseID, requiredRole)
	return kb, access, &knowledge, err
}

func (s *KnowledgeBaseShareService) ResolveChunkAccess(
	ctx context.Context, chunkID string, requiredRole string,
) (*types.KnowledgeBase, *types.KnowledgeBaseAccess, error) {
	var row struct {
		KnowledgeID string `gorm:"column:knowledge_id"`
	}
	if err := s.db.WithContext(ctx).Table("chunks").Select("knowledge_id").Where("id = ?", chunkID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apperrors.NewNotFoundError("分块不存在")
		}
		return nil, nil, err
	}
	kb, access, _, err := s.ResolveKnowledgeAccess(ctx, row.KnowledgeID, requiredRole)
	return kb, access, err
}

// WithStorageTenant switches only the resource partition. User identity stays
// untouched, so ACL checks and private session ownership still use the caller.
func (s *KnowledgeBaseShareService) WithStorageTenant(
	ctx context.Context, kb *types.KnowledgeBase, role string,
) context.Context {
	if kb == nil {
		return ctx
	}
	callerTenantID, _ := types.CallerTenantIDFromContext(ctx)
	ctx = context.WithValue(ctx, types.CallerTenantIDContextKey, callerTenantID)
	ctx = context.WithValue(ctx, types.TenantIDContextKey, kb.TenantID)
	ctx = context.WithValue(ctx, types.KnowledgeBaseAccessRoleContextKey, role)
	var tenant types.Tenant
	if err := s.db.WithContext(ctx).Where("id = ?", kb.TenantID).First(&tenant).Error; err == nil {
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, &tenant)
	}
	return ctx
}

func (s *KnowledgeBaseShareService) ListAccessibleKnowledgeBases(ctx context.Context) ([]*types.KnowledgeBase, error) {
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || userID == "" {
		return nil, apperrors.NewUnauthorizedError("请先登录")
	}
	callerTenantID, _ := types.CallerTenantIDFromContext(ctx)
	var rows []*types.KnowledgeBase
	err := s.db.WithContext(ctx).
		Where("is_temporary = ? AND (owner_user_id = ? OR (owner_user_id = '' AND tenant_id = ?) OR id IN (?))",
			false, userID, callerTenantID,
			s.db.Model(&types.KnowledgeBaseMember{}).Select("knowledge_base_id").Where("user_id = ?", userID)).
		Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}

	var members []types.KnowledgeBaseMember
	_ = s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&members).Error
	roles := make(map[string]string, len(members))
	for _, member := range members {
		roles[member.KnowledgeBaseID] = member.Role
	}
	ownerIDs := make([]string, 0, len(rows))
	for _, kb := range rows {
		if kb.OwnerUserID != "" {
			ownerIDs = append(ownerIDs, kb.OwnerUserID)
		}
	}
	type ownerRow struct{ ID, Username string }
	var owners []ownerRow
	if len(ownerIDs) > 0 {
		_ = s.db.WithContext(ctx).Model(&types.User{}).Select("id, username").Where("id IN ?", ownerIDs).Scan(&owners).Error
	}
	ownerNames := make(map[string]string, len(owners))
	for _, owner := range owners {
		ownerNames[owner.ID] = owner.Username
	}
	for _, kb := range rows {
		if kb.OwnerUserID == userID || (kb.OwnerUserID == "" && kb.TenantID == callerTenantID) {
			kb.AccessRole = types.KBRoleOwner
		} else {
			kb.AccessRole = roles[kb.ID]
			kb.IsShared = true
		}
		kb.OwnerUsername = ownerNames[kb.OwnerUserID]
		kb.EnsureDefaults()
	}

	var pins []struct {
		KBID     string    `gorm:"column:kb_id"`
		PinnedAt time.Time `gorm:"column:pinned_at"`
	}
	_ = s.db.WithContext(ctx).Table("user_kb_pins").
		Where("tenant_id = ? AND user_id = ?", callerTenantID, userID).Find(&pins).Error
	pinMap := make(map[string]time.Time, len(pins))
	for _, pin := range pins {
		pinMap[pin.KBID] = pin.PinnedAt
	}
	for _, kb := range rows {
		if t, ok := pinMap[kb.ID]; ok {
			kb.IsPinned = true
			tt := t
			kb.PinnedAt = &tt
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].IsPinned != rows[j].IsPinned {
			return rows[i].IsPinned
		}
		if rows[i].IsPinned && rows[i].PinnedAt != nil && rows[j].PinnedAt != nil {
			return rows[i].PinnedAt.After(*rows[j].PinnedAt)
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows, nil
}

func (s *KnowledgeBaseShareService) UpdateInvitation(
	ctx context.Context, kbID string, enabled bool, regenerate bool,
) (*types.KnowledgeBase, error) {
	kb, _, err := s.ResolveAccess(ctx, kbID, types.KBRoleOwner)
	if err != nil {
		return nil, err
	}
	if (enabled && kb.InviteCode == "") || regenerate {
		for attempts := 0; attempts < 5; attempts++ {
			kb.InviteCode, err = randomInviteCode()
			if err != nil {
				return nil, err
			}
			var count int64
			if dbErr := s.db.WithContext(ctx).Model(&types.KnowledgeBase{}).
				Where("invite_code = ? AND id <> ?", kb.InviteCode, kb.ID).Count(&count).Error; dbErr != nil {
				return nil, dbErr
			}
			if count == 0 {
				break
			}
		}
	}
	kb.SharingEnabled = enabled
	if err := s.db.WithContext(ctx).Model(&types.KnowledgeBase{}).Where("id = ?", kb.ID).
		Updates(map[string]interface{}{"sharing_enabled": enabled, "invite_code": kb.InviteCode, "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}
	action := "sharing_disabled"
	if enabled {
		action = "sharing_enabled"
	}
	if regenerate {
		action = "invite_code_regenerated"
	}
	_ = s.Audit(ctx, kb.ID, action, "", "", nil)
	return kb, nil
}

func randomInviteCode() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "="), nil
}

func (s *KnowledgeBaseShareService) LookupInvitation(ctx context.Context, code string) (map[string]interface{}, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, apperrors.NewBadRequestError("请输入邀请码")
	}
	var row struct{ ID, Name, OwnerUserID string }
	err := s.db.WithContext(ctx).Model(&types.KnowledgeBase{}).
		Select("id, name, owner_user_id").Where("invite_code = ? AND sharing_enabled = ?", code, true).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.NewNotFoundError("邀请码无效或已关闭")
	}
	if err != nil {
		return nil, err
	}
	var username string
	_ = s.db.WithContext(ctx).Model(&types.User{}).Select("username").Where("id = ?", row.OwnerUserID).Scan(&username).Error
	return map[string]interface{}{"knowledge_base_id": row.ID, "knowledge_base_name": row.Name, "owner_username": username}, nil
}

func (s *KnowledgeBaseShareService) SubmitJoinRequest(ctx context.Context, code string) (*types.KnowledgeBaseJoinRequest, error) {
	info, err := s.LookupInvitation(ctx, code)
	if err != nil {
		return nil, err
	}
	kbID, _ := info["knowledge_base_id"].(string)
	userID, _ := types.UserIDFromContext(ctx)
	kb, _, err := s.ResolveAccess(ctx, kbID, types.KBRoleReader)
	if err == nil && kb != nil {
		return nil, apperrors.NewConflictError("你已经是该知识库成员")
	}
	if err != nil {
		if appErr, ok := apperrors.IsAppError(err); !ok || appErr.Code != apperrors.ErrNotFound {
			return nil, err
		}
	}
	// ResolveAccess intentionally returns NotFound for non-members; continue.
	var pending types.KnowledgeBaseJoinRequest
	if dbErr := s.db.WithContext(ctx).Where("knowledge_base_id = ? AND user_id = ? AND status = ?", kbID, userID, types.KBJoinPending).
		First(&pending).Error; dbErr == nil {
		return &pending, nil
	} else if !errors.Is(dbErr, gorm.ErrRecordNotFound) {
		return nil, dbErr
	}
	now := time.Now()
	request := &types.KnowledgeBaseJoinRequest{ID: uuid.NewString(), KnowledgeBaseID: kbID, UserID: userID, Status: types.KBJoinPending, CreatedAt: now, UpdatedAt: now}
	if err := s.db.WithContext(ctx).Create(request).Error; err != nil {
		return nil, err
	}
	_ = s.Audit(ctx, kbID, "join_requested", userID, "", nil)
	return request, nil
}

func (s *KnowledgeBaseShareService) ListMyJoinRequests(ctx context.Context) ([]KBJoinRequestView, error) {
	userID, _ := types.UserIDFromContext(ctx)
	var rows []KBJoinRequestView
	err := s.db.WithContext(ctx).Table("knowledge_base_join_requests r").
		Select("r.id, r.knowledge_base_id, kb.name AS knowledge_base_name, r.user_id, u.username, r.status, r.reviewed_by, r.reviewed_at, r.created_at").
		Joins("JOIN knowledge_bases kb ON kb.id = r.knowledge_base_id").
		Joins("JOIN users u ON u.id = r.user_id").Where("r.user_id = ?", userID).
		Order("r.created_at DESC").Limit(100).Scan(&rows).Error
	return rows, err
}

func (s *KnowledgeBaseShareService) ListJoinRequests(ctx context.Context, kbID string) ([]KBJoinRequestView, error) {
	if _, _, err := s.ResolveAccess(ctx, kbID, types.KBRoleAdmin); err != nil {
		return nil, err
	}
	var rows []KBJoinRequestView
	err := s.db.WithContext(ctx).Table("knowledge_base_join_requests r").
		Select("r.id, r.knowledge_base_id, kb.name AS knowledge_base_name, r.user_id, u.username, r.status, r.reviewed_by, r.reviewed_at, r.created_at").
		Joins("JOIN knowledge_bases kb ON kb.id = r.knowledge_base_id").
		Joins("JOIN users u ON u.id = r.user_id").Where("r.knowledge_base_id = ?", kbID).
		Order("CASE WHEN r.status = 'pending' THEN 0 ELSE 1 END, r.created_at DESC").Limit(200).Scan(&rows).Error
	return rows, err
}

func (s *KnowledgeBaseShareService) ReviewJoinRequest(ctx context.Context, kbID, requestID, decision string) error {
	if _, _, err := s.ResolveAccess(ctx, kbID, types.KBRoleAdmin); err != nil {
		return err
	}
	if decision != types.KBJoinApproved && decision != types.KBJoinRejected {
		return apperrors.NewBadRequestError("审批结果无效")
	}
	reviewerID, _ := types.UserIDFromContext(ctx)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request types.KnowledgeBaseJoinRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND knowledge_base_id = ?", requestID, kbID).First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.NewNotFoundError("加入申请不存在")
			}
			return err
		}
		if request.Status != types.KBJoinPending {
			return apperrors.NewConflictError("该申请已经处理")
		}
		now := time.Now()
		if decision == types.KBJoinApproved {
			member := types.KnowledgeBaseMember{KnowledgeBaseID: kbID, UserID: request.UserID, Role: types.KBRoleReader, JoinedAt: now, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "knowledge_base_id"}, {Name: "user_id"}}, DoUpdates: clause.Assignments(map[string]interface{}{"role": types.KBRoleReader, "updated_at": now})}).Create(&member).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&request).Updates(map[string]interface{}{"status": decision, "reviewed_by": reviewerID, "reviewed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		action := "join_rejected"
		if decision == types.KBJoinApproved {
			action = "join_approved"
		}
		return s.auditWithDB(ctx, tx, kbID, action, request.UserID, request.ID, nil)
	})
}

func (s *KnowledgeBaseShareService) ListMembers(ctx context.Context, kbID string) ([]KBMemberView, error) {
	kb, _, err := s.ResolveAccess(ctx, kbID, types.KBRoleReader)
	if err != nil {
		return nil, err
	}
	var rows []KBMemberView
	err = s.db.WithContext(ctx).Table("knowledge_base_members m").
		Select("m.user_id, u.username, m.role, m.joined_at").Joins("JOIN users u ON u.id = m.user_id").
		Where("m.knowledge_base_id = ?", kbID).Order("CASE m.role WHEN 'admin' THEN 1 WHEN 'writer' THEN 2 ELSE 3 END, m.joined_at").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	var owner types.User
	if err := s.db.WithContext(ctx).Select("id, username, created_at").Where("id = ?", kb.OwnerUserID).First(&owner).Error; err == nil {
		rows = append([]KBMemberView{{UserID: owner.ID, Username: owner.Username, Role: types.KBRoleOwner, JoinedAt: kb.CreatedAt}}, rows...)
	}
	return rows, nil
}

func (s *KnowledgeBaseShareService) UpdateMemberRole(ctx context.Context, kbID, targetUserID, role string) error {
	_, actorAccess, err := s.ResolveAccess(ctx, kbID, types.KBRoleAdmin)
	if err != nil {
		return err
	}
	if role != types.KBRoleReader && role != types.KBRoleWriter && role != types.KBRoleAdmin {
		return apperrors.NewBadRequestError("成员角色无效")
	}
	var target types.KnowledgeBaseMember
	if err := s.db.WithContext(ctx).Where("knowledge_base_id = ? AND user_id = ?", kbID, targetUserID).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.NewNotFoundError("成员不存在")
		}
		return err
	}
	if actorAccess.Role == types.KBRoleAdmin && (target.Role == types.KBRoleAdmin || role == types.KBRoleAdmin) {
		return apperrors.NewForbiddenError("只有所有者可以管理管理员权限")
	}
	if err := s.db.WithContext(ctx).Model(&target).Updates(map[string]interface{}{"role": role, "updated_at": time.Now()}).Error; err != nil {
		return err
	}
	return s.Audit(ctx, kbID, "member_role_changed", targetUserID, "", map[string]interface{}{"from": target.Role, "to": role})
}

func (s *KnowledgeBaseShareService) RemoveMember(ctx context.Context, kbID, targetUserID string) error {
	_, actorAccess, err := s.ResolveAccess(ctx, kbID, types.KBRoleAdmin)
	if err != nil {
		return err
	}
	var target types.KnowledgeBaseMember
	if err := s.db.WithContext(ctx).Where("knowledge_base_id = ? AND user_id = ?", kbID, targetUserID).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.NewNotFoundError("成员不存在")
		}
		return err
	}
	if actorAccess.Role == types.KBRoleAdmin && target.Role == types.KBRoleAdmin {
		return apperrors.NewForbiddenError("管理员不能移除其他管理员")
	}
	if err := s.db.WithContext(ctx).Delete(&target).Error; err != nil {
		return err
	}
	return s.Audit(ctx, kbID, "member_removed", targetUserID, "", nil)
}

func (s *KnowledgeBaseShareService) Leave(ctx context.Context, kbID string) error {
	_, access, err := s.ResolveAccess(ctx, kbID, types.KBRoleReader)
	if err != nil {
		return err
	}
	if access.Role == types.KBRoleOwner {
		return apperrors.NewConflictError("所有者必须先转让或删除知识库")
	}
	userID, _ := types.UserIDFromContext(ctx)
	if err := s.db.WithContext(ctx).Where("knowledge_base_id = ? AND user_id = ?", kbID, userID).Delete(&types.KnowledgeBaseMember{}).Error; err != nil {
		return err
	}
	return s.Audit(ctx, kbID, "member_left", userID, "", nil)
}

func (s *KnowledgeBaseShareService) ListAuditLogs(ctx context.Context, kbID string) ([]KBAuditLogView, error) {
	if _, _, err := s.ResolveAccess(ctx, kbID, types.KBRoleAdmin); err != nil {
		return nil, err
	}
	var rows []KBAuditLogView
	err := s.db.WithContext(ctx).Table("knowledge_base_audit_logs l").
		Select("l.id, l.actor_user_id, actor.username AS actor_username, l.action, l.target_user_id, target.username AS target_username, l.target_resource_id, l.details, l.created_at").
		Joins("LEFT JOIN users actor ON actor.id = l.actor_user_id").Joins("LEFT JOIN users target ON target.id = l.target_user_id").
		Where("l.knowledge_base_id = ?", kbID).Order("l.created_at DESC").Limit(300).Scan(&rows).Error
	return rows, err
}

func (s *KnowledgeBaseShareService) Audit(ctx context.Context, kbID, action, targetUserID, targetResourceID string, details map[string]interface{}) error {
	return s.auditWithDB(ctx, s.db.WithContext(ctx), kbID, action, targetUserID, targetResourceID, details)
}

func (s *KnowledgeBaseShareService) auditWithDB(ctx context.Context, db *gorm.DB, kbID, action, targetUserID, targetResourceID string, details map[string]interface{}) error {
	actorUserID, _ := types.UserIDFromContext(ctx)
	if actorUserID == "" {
		return nil
	}
	var raw types.JSON
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			raw = b
		}
	}
	return db.Create(&types.KnowledgeBaseAuditLog{KnowledgeBaseID: kbID, ActorUserID: actorUserID, Action: action, TargetUserID: targetUserID, TargetResourceID: targetResourceID, Details: raw, CreatedAt: time.Now()}).Error
}
