<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAuthStore } from '@/stores/auth'
import {
  getKnowledgeBaseSharing,
  leaveKnowledgeBase,
  listKnowledgeBaseAuditLogs,
  listKnowledgeBaseJoinRequests,
  listKnowledgeBaseMembers,
  removeKnowledgeBaseMember,
  reviewKnowledgeBaseJoinRequest,
  updateKnowledgeBaseInvitation,
  updateKnowledgeBaseMemberRole,
  type KnowledgeBaseRole,
} from '@/api/knowledge-base'

const props = defineProps<{ visible: boolean; kbId: string; accessRole?: KnowledgeBaseRole }>()
const emit = defineEmits<{ (e: 'update:visible', value: boolean): void; (e: 'changed'): void; (e: 'left'): void }>()
const auth = useAuthStore()

const loading = ref(false)
const settings = ref<any>(null)
const members = ref<any[]>([])
const requests = ref<any[]>([])
const logs = ref<any[]>([])
const tab = ref<'members' | 'requests' | 'logs'>('members')

const role = computed(() => settings.value?.access_role || props.accessRole || 'reader')
const isOwner = computed(() => role.value === 'owner')
const canAdmin = computed(() => role.value === 'owner' || role.value === 'admin')

const load = async () => {
  if (!props.kbId) return
  loading.value = true
  try {
    const [settingRes, memberRes] = await Promise.all([
      getKnowledgeBaseSharing(props.kbId),
      listKnowledgeBaseMembers(props.kbId),
    ]) as any[]
    settings.value = settingRes?.data || null
    members.value = Array.isArray(memberRes?.data) ? memberRes.data : []
    if (canAdmin.value) {
      const [requestRes, logRes] = await Promise.all([
        listKnowledgeBaseJoinRequests(props.kbId),
        listKnowledgeBaseAuditLogs(props.kbId),
      ]) as any[]
      requests.value = Array.isArray(requestRes?.data) ? requestRes.data : []
      logs.value = Array.isArray(logRes?.data) ? logRes.data : []
    } else {
      requests.value = []
      logs.value = []
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || '共享信息加载失败')
  } finally { loading.value = false }
}

const toggleSharing = async (enabled: boolean) => {
  try {
    const res: any = await updateKnowledgeBaseInvitation(props.kbId, enabled)
    settings.value = { ...settings.value, ...(res?.data || {}) }
    MessagePlugin.success(enabled ? '邀请码共享已开启' : '已停止接收新的加入申请')
  } catch (error: any) { MessagePlugin.error(error?.message || '操作失败') }
}

const regenerate = async () => {
  try {
    const res: any = await updateKnowledgeBaseInvitation(props.kbId, true, true)
    settings.value = { ...settings.value, ...(res?.data || {}) }
    MessagePlugin.success('已生成新的邀请码，旧邀请码已失效')
  } catch (error: any) { MessagePlugin.error(error?.message || '重新生成失败') }
}

const copyCode = async () => {
  if (!settings.value?.invite_code) return
  await navigator.clipboard.writeText(settings.value.invite_code)
  MessagePlugin.success('邀请码已复制')
}

const review = async (item: any, decision: 'approved' | 'rejected') => {
  try {
    await reviewKnowledgeBaseJoinRequest(props.kbId, item.id, decision)
    MessagePlugin.success(decision === 'approved' ? '已批准，成员默认为读取用户' : '已拒绝申请')
    await load()
    emit('changed')
  } catch (error: any) { MessagePlugin.error(error?.message || '审批失败') }
}

const changeRole = async (member: any, nextRole: 'admin' | 'writer' | 'reader') => {
  try {
    await updateKnowledgeBaseMemberRole(props.kbId, member.user_id, nextRole)
    MessagePlugin.success('成员权限已更新')
    await load()
  } catch (error: any) { MessagePlugin.error(error?.message || '权限更新失败') }
}

const removeMember = async (member: any) => {
  try {
    await removeKnowledgeBaseMember(props.kbId, member.user_id)
    MessagePlugin.success('成员已移除')
    await load()
  } catch (error: any) { MessagePlugin.error(error?.message || '移除成员失败') }
}

const leave = async () => {
  try {
    await leaveKnowledgeBase(props.kbId)
    MessagePlugin.success('已退出共享知识库')
    emit('update:visible', false)
    emit('left')
  } catch (error: any) { MessagePlugin.error(error?.message || '退出失败') }
}

const roleText = (value: string) => ({ owner: '所有者', admin: '管理员', writer: '写入用户', reader: '读取用户' }[value] || value)
const actionText = (value: string) => ({
  sharing_enabled: '开启了邀请码共享', sharing_disabled: '关闭了邀请码共享',
  invite_code_regenerated: '重新生成了邀请码', join_requested: '提交了加入申请',
  join_approved: '批准了加入申请', join_rejected: '拒绝了加入申请',
  member_role_changed: '修改了成员权限', member_removed: '移除了成员', member_left: '退出了知识库',
  document_uploaded: '上传了文档', document_deleted: '删除了文档',
  document_reparse_requested: '重新解析了文档', documents_cleared: '清空了文档',
  knowledge_base_rebuild_requested: '发起了知识库重建',
}[value] || value)

watch(() => props.visible, (visible) => { if (visible) void load() })
</script>

<template>
  <t-dialog :visible="visible" header="知识库共享" width="760px" :footer="false" @update:visible="emit('update:visible', $event)">
    <div v-if="loading" class="loading"><t-loading /> 正在加载共享信息...</div>
    <div v-else class="share-dialog">
      <section v-if="isOwner" class="invite-panel">
        <div>
          <strong>邀请码共享</strong>
          <p>关闭后只停止新的申请，现有成员不会受到影响。</p>
        </div>
        <t-switch :value="!!settings?.sharing_enabled" @change="toggleSharing" />
        <div v-if="settings?.sharing_enabled" class="invite-code">
          <code>{{ settings?.invite_code }}</code>
          <t-button size="small" variant="outline" @click="copyCode">复制</t-button>
          <t-button size="small" variant="outline" @click="regenerate">重新生成</t-button>
        </div>
      </section>

      <div class="tabs">
        <button :class="{ active: tab === 'members' }" @click="tab = 'members'">成员 {{ members.length }}</button>
        <button v-if="canAdmin" :class="{ active: tab === 'requests' }" @click="tab = 'requests'">加入申请 {{ requests.filter(i => i.status === 'pending').length }}</button>
        <button v-if="canAdmin" :class="{ active: tab === 'logs' }" @click="tab = 'logs'">操作日志</button>
      </div>

      <section v-if="tab === 'members'" class="list-panel">
        <div v-for="member in members" :key="member.user_id" class="member-row">
          <div><strong>{{ member.username }}</strong><small>{{ roleText(member.role) }}</small></div>
          <div v-if="member.role !== 'owner' && canAdmin" class="member-actions">
            <template v-if="isOwner">
              <t-select :value="member.role" size="small" style="width: 120px" @change="changeRole(member, $event as any)">
                <t-option value="reader" label="读取用户" /><t-option value="writer" label="写入用户" /><t-option value="admin" label="管理员" />
              </t-select>
            </template>
            <template v-else-if="member.role !== 'admin'">
              <t-button size="small" variant="outline" @click="changeRole(member, member.role === 'writer' ? 'reader' : 'writer')">
                {{ member.role === 'writer' ? '取消写入' : '授予写入' }}
              </t-button>
            </template>
            <t-popconfirm v-if="isOwner || member.role !== 'admin'" content="确定移除该成员吗？" @confirm="removeMember(member)">
              <t-button size="small" theme="danger" variant="text">移除</t-button>
            </t-popconfirm>
          </div>
        </div>
        <t-button v-if="!isOwner" class="leave-button" theme="danger" variant="outline" @click="leave">退出该知识库</t-button>
      </section>

      <section v-else-if="tab === 'requests'" class="list-panel">
        <div v-if="!requests.length" class="empty">暂无加入申请</div>
        <div v-for="item in requests" :key="item.id" class="member-row">
          <div><strong>{{ item.username }}</strong><small>{{ new Date(item.created_at).toLocaleString() }} · {{ item.status === 'pending' ? '待审批' : item.status === 'approved' ? '已通过' : '已拒绝' }}</small></div>
          <div v-if="item.status === 'pending'" class="member-actions">
            <t-button size="small" theme="primary" @click="review(item, 'approved')">批准</t-button>
            <t-button size="small" variant="outline" @click="review(item, 'rejected')">拒绝</t-button>
          </div>
        </div>
      </section>

      <section v-else class="list-panel">
        <div v-if="!logs.length" class="empty">暂无操作日志</div>
        <div v-for="item in logs" :key="item.id" class="log-row">
          <span><strong>{{ item.actor_username }}</strong> {{ actionText(item.action) }}<template v-if="item.target_username">：{{ item.target_username }}</template></span>
          <small>{{ new Date(item.created_at).toLocaleString() }}</small>
        </div>
      </section>
    </div>
  </t-dialog>
</template>

<style scoped lang="less">
.loading { min-height: 260px; display: grid; place-content: center; gap: 10px; color: var(--td-text-color-secondary); }
.share-dialog { display: flex; flex-direction: column; gap: 16px; }
.invite-panel { padding: 16px; display: grid; grid-template-columns: 1fr auto; align-items: center; gap: 12px; border: 1px solid var(--td-component-stroke); border-radius: 8px; background: var(--td-bg-color-container-hover); }
.invite-panel p { margin: 5px 0 0; color: var(--td-text-color-secondary); font-size: 12px; }
.invite-code { grid-column: 1 / -1; display: flex; align-items: center; gap: 8px; }
.invite-code code { flex: 1; padding: 9px 12px; border-radius: 6px; background: var(--td-bg-color-container); letter-spacing: 2px; font-weight: 700; }
.tabs { display: flex; gap: 4px; border-bottom: 1px solid var(--td-component-stroke); }
.tabs button { padding: 9px 12px; border: 0; border-bottom: 2px solid transparent; color: var(--td-text-color-secondary); background: transparent; cursor: pointer; }
.tabs button.active { border-bottom-color: var(--td-brand-color); color: var(--td-brand-color); font-weight: 650; }
.list-panel { max-height: 420px; overflow-y: auto; }
.member-row, .log-row { min-height: 58px; padding: 8px 4px; display: flex; align-items: center; justify-content: space-between; gap: 14px; border-bottom: 1px solid var(--td-component-stroke); }
.member-row > div:first-child { display: flex; flex-direction: column; gap: 4px; }
.member-row small, .log-row small { color: var(--td-text-color-secondary); font-size: 12px; }
.member-actions { display: flex; align-items: center; gap: 7px; }
.log-row { align-items: flex-start; flex-direction: column; justify-content: center; gap: 4px; }
.empty { padding: 36px; text-align: center; color: var(--td-text-color-placeholder); }
.leave-button { margin-top: 18px; }
</style>
