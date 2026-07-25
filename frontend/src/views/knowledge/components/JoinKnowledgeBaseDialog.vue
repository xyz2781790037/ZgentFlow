<script setup lang="ts">
import { ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  listMyKnowledgeBaseJoinRequests,
  lookupKnowledgeBaseInvitation,
  submitKnowledgeBaseJoinRequest,
} from '@/api/knowledge-base'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', value: boolean): void }>()

const code = ref('')
const preview = ref<any>(null)
const requests = ref<any[]>([])
const checking = ref(false)
const submitting = ref(false)

const loadRequests = async () => {
  try {
    const res: any = await listMyKnowledgeBaseJoinRequests()
    requests.value = Array.isArray(res?.data) ? res.data : []
  } catch { requests.value = [] }
}

const lookup = async () => {
  const value = code.value.trim()
  if (!value) { MessagePlugin.warning('请输入邀请码'); return }
  checking.value = true
  preview.value = null
  try {
    const res: any = await lookupKnowledgeBaseInvitation(value)
    preview.value = res?.data || null
  } catch (error: any) {
    MessagePlugin.error(error?.message || '邀请码无效')
  } finally { checking.value = false }
}

const submit = async () => {
  if (!preview.value) return
  submitting.value = true
  try {
    await submitKnowledgeBaseJoinRequest(code.value.trim())
    MessagePlugin.success('加入申请已提交，等待管理员审批')
    code.value = ''
    preview.value = null
    await loadRequests()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '提交申请失败')
  } finally { submitting.value = false }
}

const statusText = (status: string) => ({ pending: '待审批', approved: '已通过', rejected: '已拒绝' }[status] || status)
const statusTheme = (status: string) => status === 'approved' ? 'success' : status === 'rejected' ? 'danger' : 'warning'

watch(() => code.value, () => { preview.value = null })
watch(() => props.visible, (visible) => { if (visible) void loadRequests() })
</script>

<template>
  <t-dialog
    :visible="visible"
    header="加入已有知识库"
    width="620px"
    :footer="false"
    @update:visible="emit('update:visible', $event)"
    @opened="loadRequests"
  >
    <div class="join-dialog">
      <p class="hint">输入知识库所有者提供的邀请码。提交后默认获得读取权限，需要管理员审批。</p>
      <div class="code-row">
        <t-input v-model="code" placeholder="请输入邀请码" clearable @enter="lookup" />
        <t-button :loading="checking" @click="lookup">验证邀请码</t-button>
      </div>

      <div v-if="preview" class="invite-preview">
        <div>
          <span>知识库</span>
          <strong>{{ preview.knowledge_base_name }}</strong>
        </div>
        <div>
          <span>所有者</span>
          <strong>{{ preview.owner_username }}</strong>
        </div>
        <t-button theme="primary" :loading="submitting" @click="submit">提交加入申请</t-button>
      </div>

      <section class="request-section">
        <h3>我的申请</h3>
        <div v-if="!requests.length" class="empty">暂无加入申请</div>
        <div v-for="item in requests" :key="item.id" class="request-row">
          <div>
            <strong>{{ item.knowledge_base_name }}</strong>
            <small>{{ new Date(item.created_at).toLocaleString() }}</small>
          </div>
          <t-tag size="small" :theme="statusTheme(item.status)" variant="light-outline">{{ statusText(item.status) }}</t-tag>
        </div>
      </section>
    </div>
  </t-dialog>
</template>

<style scoped lang="less">
.join-dialog { display: flex; flex-direction: column; gap: 18px; }
.hint { margin: 0; color: var(--td-text-color-secondary); line-height: 1.7; }
.code-row { display: grid; grid-template-columns: 1fr auto; gap: 10px; }
.invite-preview { padding: 16px; display: grid; grid-template-columns: 1fr 1fr auto; align-items: center; gap: 18px; border: 1px solid var(--td-brand-color-3); border-radius: 8px; background: var(--td-brand-color-1); }
.invite-preview div { display: flex; flex-direction: column; gap: 4px; }
.invite-preview span, .request-row small { color: var(--td-text-color-secondary); font-size: 12px; }
.request-section h3 { margin: 0 0 10px; font-size: 15px; }
.request-row { min-height: 54px; padding: 0 12px; display: flex; justify-content: space-between; align-items: center; border-top: 1px solid var(--td-component-stroke); }
.request-row > div { display: flex; flex-direction: column; gap: 4px; }
.empty { padding: 22px; text-align: center; color: var(--td-text-color-placeholder); background: var(--td-bg-color-container-hover); border-radius: 8px; }
</style>
