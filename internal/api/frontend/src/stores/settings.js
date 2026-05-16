import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getSettings, updateSettings } from '../api/settings'

export const useSettingsStore = defineStore('settings', () => {
  const autoSubmit = ref(false)
  const loading = ref(false)

  async function fetchSettings() {
    loading.value = true
    try {
      const data = await getSettings()
      autoSubmit.value = data.auto_submit || false
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

  return {
    autoSubmit,
    loading,
    fetchSettings,
    updateAutoSubmit,
  }
})
