import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getSettings, updateSettings } from '../api/settings'

export const useSettingsStore = defineStore('settings', () => {
  const autoSubmit = ref(false)
  const gitlabUrl = ref('')
  const gitlabToken = ref('')
  const ignorePaths = ref([])
  const maxLineComments = ref(20)
  const webhookToken = ref('')
  const logLevel = ref('info')
  const jwtExpiry = ref('24h')
  const loading = ref(false)

  async function fetchSettings() {
    loading.value = true
    try {
      const data = await getSettings()
      autoSubmit.value = data.auto_submit || false
      gitlabUrl.value = data.gitlab_url || ''
      gitlabToken.value = data.gitlab_token || ''
      ignorePaths.value = data.ignore_paths || []
      maxLineComments.value = data.max_line_comments ?? 20
      webhookToken.value = data.webhook_token || ''
      logLevel.value = data.log_level || 'info'
      jwtExpiry.value = data.jwt_expiry || '24h'
    } finally {
      loading.value = false
    }
  }

  async function updateAutoSubmit(value) {
    loading.value = true
    try {
      const data = await updateSettings({ auto_submit: value })
      autoSubmit.value = data.auto_submit || false
    } finally {
      loading.value = false
    }
  }

  async function saveGeneralSettings(settings) {
    loading.value = true
    try {
      const data = await updateSettings(settings)
      autoSubmit.value = data.auto_submit || false
      gitlabUrl.value = data.gitlab_url || ''
      gitlabToken.value = data.gitlab_token || ''
      ignorePaths.value = data.ignore_paths || []
      maxLineComments.value = data.max_line_comments ?? 20
      webhookToken.value = data.webhook_token || ''
      logLevel.value = data.log_level || 'info'
      jwtExpiry.value = data.jwt_expiry || '24h'
      return data
    } finally {
      loading.value = false
    }
  }

  return {
    autoSubmit,
    gitlabUrl,
    gitlabToken,
    ignorePaths,
    maxLineComments,
    webhookToken,
    logLevel,
    jwtExpiry,
    loading,
    fetchSettings,
    updateAutoSubmit,
    saveGeneralSettings,
  }
})
