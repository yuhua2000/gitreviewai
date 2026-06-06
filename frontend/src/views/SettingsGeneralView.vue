<template>
  <div class="settings-general">
    <n-card title="全局设置">
      <n-spin :show="store.loading">
        <n-form
          ref="formRef"
          :model="formData"
          label-placement="left"
          label-width="120"
          style="max-width: 640px;"
        >
          <n-divider content-placement="left">GitLab</n-divider>

          <n-form-item label="GitLab URL">
            <n-input
              v-model:value="formData.gitlab_url"
              placeholder="https://gitlab.example.com"
            />
          </n-form-item>

          <n-form-item label="GitLab Token">
            <n-input
              v-model:value="formData.gitlab_token"
              type="password"
              show-password-on="click"
              :placeholder="tokenPlaceholder"
            />
          </n-form-item>

          <n-form-item label="Webhook Token">
            <n-input
              v-model:value="formData.webhook_token"
              type="password"
              show-password-on="click"
              :placeholder="webhookTokenPlaceholder"
            />
          </n-form-item>

          <n-divider content-placement="left">审核</n-divider>

          <n-form-item label="忽略路径">
            <n-dynamic-tags v-model:value="formData.ignore_paths" />
          </n-form-item>

          <n-form-item label="最大行评论数">
            <n-input-number
              v-model:value="formData.max_line_comments"
              :min="1"
              :max="100"
              style="width: 200px;"
            />
          </n-form-item>

          <n-form-item label="自动提交">
            <n-space align="center" :size="12">
              <n-switch v-model:value="formData.auto_submit" />
              <n-text depth="3">
                {{ formData.auto_submit ? '开启：评论和报告将自动提交到 GitLab' : '关闭：需要手动审核后提交' }}
              </n-text>
            </n-space>
          </n-form-item>

          <n-divider content-placement="left">系统</n-divider>

          <n-form-item label="日志级别">
            <n-select
              v-model:value="formData.log_level"
              :options="logLevelOptions"
              style="width: 200px;"
            />
          </n-form-item>

          <n-form-item label="JWT 过期时间">
            <n-input
              v-model:value="formData.jwt_expiry"
              placeholder="24h"
              style="width: 200px;"
            />
          </n-form-item>

          <n-form-item>
            <n-button type="primary" :loading="saving" @click="handleSave">
              保存设置
            </n-button>
          </n-form-item>
        </n-form>
      </n-spin>
    </n-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useSettingsStore } from '../stores/settings'

const message = useMessage()
const store = useSettingsStore()
const saving = ref(false)
const formRef = ref(null)

const formData = ref({
  gitlab_url: '',
  gitlab_token: '',
  webhook_token: '',
  ignore_paths: [],
  max_line_comments: 20,
  auto_submit: false,
  log_level: 'info',
  jwt_expiry: '24h',
})

const logLevelOptions = [
  { label: 'debug', value: 'debug' },
  { label: 'info', value: 'info' },
  { label: 'warn', value: 'warn' },
  { label: 'error', value: 'error' },
]

const tokenPlaceholder = computed(() => {
  if (store.gitlabToken && store.gitlabToken !== '****') {
    return '**********（已设置，留空则保持不变）'
  }
  return '输入 GitLab Personal Access Token'
})

const webhookTokenPlaceholder = computed(() => {
  if (store.webhookToken && store.webhookToken !== '****') {
    return '**********（已设置，留空则保持不变）'
  }
  return '输入 Webhook Secret Token（可选）'
})

function syncFromStore() {
  formData.value = {
    gitlab_url: store.gitlabUrl || '',
    gitlab_token: '',
    webhook_token: '',
    ignore_paths: [...(store.ignorePaths || [])],
    max_line_comments: store.maxLineComments ?? 20,
    auto_submit: store.autoSubmit || false,
    log_level: store.logLevel || 'info',
    jwt_expiry: store.jwtExpiry || '24h',
  }
}

async function handleSave() {
  saving.value = true
  try {
    const payload = {
      gitlab_url: formData.value.gitlab_url,
      ignore_paths: formData.value.ignore_paths,
      max_line_comments: formData.value.max_line_comments,
      auto_submit: formData.value.auto_submit,
      log_level: formData.value.log_level,
      jwt_expiry: formData.value.jwt_expiry,
    }
    // Only send tokens if user entered a new one
    if (formData.value.gitlab_token) {
      payload.gitlab_token = formData.value.gitlab_token
    }
    if (formData.value.webhook_token) {
      payload.webhook_token = formData.value.webhook_token
    }

    await store.saveGeneralSettings(payload)
    message.success('设置已保存')
    syncFromStore()
  } catch (e) {
    message.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await store.fetchSettings()
  syncFromStore()
})
</script>

<style scoped>
.settings-general {
  padding: 0;
}
</style>
