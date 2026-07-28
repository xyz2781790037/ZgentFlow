<template>
  <div class="api-config-settings">
    <header class="api-config-header">
      <span>密钥配置</span>
      <t-button v-if="canManage" theme="primary" @click="openCreate()">
        <template #icon><t-icon name="add" /></template>
        添加 API
      </t-button>
    </header>

    <div class="api-summary" aria-label="API 配置概览">
      <div>
        <strong>{{ configs.length }}</strong>
        <span>API 配置</span>
      </div>
      <div>
        <strong>{{ referencedModelCount }}</strong>
        <span>模型引用</span>
      </div>
      <p><t-icon name="info-circle" />服务商用于归类和筛选，Base URL 仍由每个模型单独配置。</p>
    </div>

    <t-loading :loading="loading" size="small" class="api-config-loading">
      <div class="provider-groups">
        <section v-for="provider in providers" :key="provider.id" class="provider-group">
          <header class="provider-heading">
            <span class="provider-symbol" :class="`provider-symbol--${provider.id}`">
              <t-icon :name="provider.icon" />
            </span>
            <div>
              <h3>{{ provider.name }}</h3>
              <p>{{ provider.description }}</p>
            </div>
            <span class="provider-count">{{ configsFor(provider.id).length }}</span>
          </header>

          <div v-if="configsFor(provider.id).length" class="config-list">
            <article v-for="config in configsFor(provider.id)" :key="config.id" class="config-row">
              <div class="key-mark"><t-icon name="key" /></div>
              <div class="config-main">
                <strong>{{ config.name }}</strong>
                <span :class="{ configured: config.configured }">
                  <i></i>{{ config.configured ? '密钥已配置' : '密钥不可用' }}
                </span>
              </div>
              <div class="reference-count">
                <strong>{{ config.model_count }}</strong>
                <span>个模型引用</span>
              </div>
              <div v-if="canManage" class="config-actions">
                <t-tooltip content="编辑" placement="top">
                  <t-button variant="text" shape="square" @click="openEdit(config)">
                    <template #icon><t-icon name="edit" /></template>
                  </t-button>
                </t-tooltip>
                <t-popconfirm
                  :content="deleteMessage(config)"
                  :confirm-btn="{ content: '删除', theme: 'danger' }"
                  cancel-btn="取消"
                  placement="bottom-right"
                  @confirm="removeConfig(config)"
                >
                  <t-tooltip content="删除" placement="top">
                    <t-button theme="danger" variant="text" shape="square">
                      <template #icon><t-icon name="delete" /></template>
                    </t-button>
                  </t-tooltip>
                </t-popconfirm>
              </div>
            </article>
          </div>
          <button v-else-if="canManage" type="button" class="empty-provider" @click="openCreate(provider.id)">
            <t-icon name="add-circle" />
            <span>添加第一个 {{ provider.name }} API</span>
          </button>
          <div v-else class="empty-provider empty-provider--readonly">暂无配置</div>
        </section>
      </div>
    </t-loading>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editingConfig ? '编辑 API 配置' : '添加 API 配置'"
      :confirm-btn="{ content: editingConfig ? '保存修改' : '添加配置', loading: saving }"
      cancel-btn="取消"
      width="520px"
      @confirm="saveConfig"
      @closed="resetForm"
    >
      <div class="api-form">
        <div class="form-item">
          <label class="form-label required">配置名称</label>
          <t-input v-model="form.name" maxlength="100" placeholder="例如：生产环境、个人账号" />
          <p>同一服务商下名称不可重复，用于添加模型时区分不同 Key。</p>
        </div>
        <div class="form-item">
          <label class="form-label required">服务商</label>
          <t-select v-model="form.provider" :disabled="!!editingConfig">
            <t-option v-for="provider in providers" :key="provider.id" :value="provider.id" :label="provider.name" />
          </t-select>
        </div>
        <div class="form-item">
          <label class="form-label" :class="{ required: !editingConfig }">API Key</label>
          <t-input
            v-model="form.apiKey"
            type="password"
            autocomplete="off"
            :placeholder="editingConfig ? '留空则保持当前 Key' : '输入 API Key'"
          >
            <template #prefix-icon><t-icon name="lock-on" /></template>
          </t-input>
          <p v-if="editingConfig">已关联的模型会在下次调用时自动使用新 Key。</p>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  createModelAPIConfig,
  deleteModelAPIConfig,
  listModelAPIConfigs,
  updateModelAPIConfig,
  type ModelAPIConfig,
  type ModelAPIConfigProvider,
} from '@/api/model-api-config'

const canManage = true
const configs = ref<ModelAPIConfig[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingConfig = ref<ModelAPIConfig | null>(null)
const form = ref({ name: '', provider: 'siliconflow' as ModelAPIConfigProvider, apiKey: '' })

const providers: Array<{
  id: ModelAPIConfigProvider
  name: string
  description: string
  icon: string
}> = [
  { id: 'siliconflow', name: '硅基流动', description: 'SiliconFlow 模型服务', icon: 'layers' },
  { id: 'deepseek', name: 'DeepSeek', description: 'DeepSeek 官方模型服务', icon: 'chat' },
  { id: 'hunyuan', name: '腾讯混元', description: '腾讯混元模型服务', icon: 'cloud' },
  { id: 'generic', name: '自定义', description: 'OpenAI 兼容或私有模型服务', icon: 'link' },
]

const referencedModelCount = computed(() =>
  configs.value.reduce((total, config) => total + config.model_count, 0),
)

const configsFor = (provider: ModelAPIConfigProvider) =>
  configs.value.filter(config => config.provider === provider)

const loadConfigs = async () => {
  loading.value = true
  try {
    configs.value = await listModelAPIConfigs()
  } catch (error: any) {
    MessagePlugin.error(error?.message || 'API 配置加载失败')
  } finally {
    loading.value = false
  }
}

const openCreate = (provider: ModelAPIConfigProvider = 'siliconflow') => {
  editingConfig.value = null
  form.value = { name: '', provider, apiKey: '' }
  dialogVisible.value = true
}

const openEdit = (config: ModelAPIConfig) => {
  editingConfig.value = config
  form.value = { name: config.name, provider: config.provider, apiKey: '' }
  dialogVisible.value = true
}

const resetForm = () => {
  editingConfig.value = null
  form.value = { name: '', provider: 'siliconflow', apiKey: '' }
}

const saveConfig = async () => {
  const name = form.value.name.trim()
  const apiKey = form.value.apiKey.trim()
  if (!name) {
    MessagePlugin.warning('请输入配置名称')
    return
  }
  if (!editingConfig.value && !apiKey) {
    MessagePlugin.warning('请输入 API Key')
    return
  }
  saving.value = true
  try {
    if (editingConfig.value) {
      await updateModelAPIConfig(editingConfig.value.id, {
        name,
        provider: form.value.provider,
        ...(apiKey ? { api_key: apiKey } : {}),
      })
      MessagePlugin.success('API 配置已更新')
    } else {
      await createModelAPIConfig({ name, provider: form.value.provider, api_key: apiKey })
      MessagePlugin.success('API 配置已添加')
    }
    dialogVisible.value = false
    await loadConfigs()
  } catch (error: any) {
    MessagePlugin.error(error?.message || 'API 配置保存失败')
  } finally {
    saving.value = false
  }
}

const deleteMessage = (config: ModelAPIConfig) =>
  config.model_count > 0
    ? `删除后，${config.model_count} 个关联模型将无法调用，请在模型服务中手动更换。`
    : '确定删除该 API 配置吗？'

const removeConfig = async (config: ModelAPIConfig) => {
  try {
    await deleteModelAPIConfig(config.id)
    MessagePlugin.success('API 配置已删除')
    await loadConfigs()
  } catch (error: any) {
    MessagePlugin.error(error?.message || 'API 配置删除失败')
  }
}

onMounted(loadConfigs)
</script>

<style scoped lang="less">
.api-config-settings { padding: 0 0 48px; color: var(--zeal-ink, #18212f); }
.api-config-header { min-height: 44px; display: flex; align-items: center; justify-content: space-between; gap: 24px; margin-bottom: 14px; }
.api-config-header > span { color: var(--zeal-muted, #778398); font-size: 11px; font-weight: 700; }
.api-summary { min-height: 80px; display: flex; align-items: center; gap: 30px; padding: 14px 18px; margin-bottom: 28px; border: 1px solid var(--zeal-line, #dbe3ed); border-radius: 8px; background: var(--zeal-surface, #fff); box-shadow: var(--zeal-shadow-xs); }
.api-summary > div { display: grid; min-width: 78px; }
.api-summary strong { font-size: 23px; line-height: 1.1; color: #0d4fae; }
.api-summary span { margin-top: 4px; color: #68758a; font-size: 12px; }
.api-summary p { display: flex; align-items: center; gap: 7px; margin: 0 0 0 auto; color: #526176; font-size: 12px; }
.api-config-loading { width: 100%; min-height: 240px; }
.provider-groups { display: grid; gap: 30px; }
.provider-group { width: 100%; }
.provider-heading { display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: 12px; padding-bottom: 11px; border-bottom: 1px solid var(--zeal-line, #dbe3ed); }
.provider-symbol { width: 36px; height: 36px; display: grid; place-items: center; border-radius: 7px; color: #fff; background: var(--zeal-primary, #1769dc); }
.provider-symbol--deepseek { background: #3155a6; }
.provider-symbol--hunyuan { background: #087f8c; }
.provider-symbol--generic { background: #596579; }
.provider-heading h3 { margin: 0; font-size: 15px; letter-spacing: 0; }
.provider-heading p { margin: 3px 0 0; color: #7b8799; font-size: 12px; }
.provider-count { min-width: 28px; text-align: center; color: #526176; font-size: 12px; }
.config-list { display: grid; }
.config-row { min-height: 70px; display: grid; grid-template-columns: 36px minmax(160px, 1fr) 110px auto; align-items: center; gap: 13px; padding: 10px 8px; border-bottom: 1px solid #edf1f6; }
.config-row:hover { background: #f8fafd; }
.key-mark { width: 32px; height: 32px; display: grid; place-items: center; border: 1px solid var(--zeal-line, #dbe3ed); border-radius: 6px; color: var(--zeal-primary, #1769dc); background: var(--zeal-surface, #fff); }
.config-main { min-width: 0; display: grid; gap: 5px; }
.config-main strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
.config-main span { display: flex; align-items: center; gap: 6px; color: #a34b4b; font-size: 12px; }
.config-main span i { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.config-main span.configured { color: #15805d; }
.reference-count { display: grid; gap: 2px; text-align: right; }
.reference-count strong { font-size: 14px; }
.reference-count span { color: #7b8799; font-size: 11px; }
.config-actions { display: flex; gap: 2px; padding-left: 8px; }
.empty-provider { width: 100%; min-height: 58px; display: flex; align-items: center; justify-content: center; gap: 7px; border: 1px dashed #cad6e5; border-top: 0; color: #526b8d; background: #fbfcfe; cursor: pointer; }
.empty-provider:hover { color: #1268e3; background: #f4f8fe; }
.empty-provider--readonly { cursor: default; color: #8a95a6; }
.api-form { display: grid; gap: 18px; padding: 8px 2px; }
.form-item { display: grid; gap: 7px; }
.form-label { color: #29364a; font-size: 13px; font-weight: 600; }
.form-label.required::after { content: '*'; margin-left: 4px; color: #d54941; }
.form-item p { margin: 0; color: #7b8799; font-size: 12px; line-height: 1.55; }

@media (max-width: 760px) {
  .api-config-settings { padding: 0 0 36px; }
  .api-config-header { align-items: stretch; flex-direction: column; }
  .api-config-header > span { display: none; }
  .api-summary { align-items: flex-start; flex-wrap: wrap; gap: 18px; }
  .api-summary p { width: 100%; margin: 0; }
  .config-row { grid-template-columns: 32px minmax(0, 1fr) auto; }
  .reference-count { display: none; }
}
</style>
