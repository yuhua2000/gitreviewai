import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listProjectConfigs, getProjectConfig, updateProjectConfig, updateProjectRules } from '../api/projectConfigs'

export const useProjectConfigsStore = defineStore('projectConfigs', () => {
  const configs = ref([])
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)
  const currentConfig = ref(null)

  async function fetchConfigs(p = page.value, ps = pageSize.value) {
    loading.value = true
    try {
      const result = await listProjectConfigs(p, ps)
      configs.value = result.data || []
      total.value = result.total || 0
      page.value = p
      pageSize.value = ps
    } finally {
      loading.value = false
    }
  }

  async function fetchConfigDetail(projectId) {
    loading.value = true
    try {
      currentConfig.value = await getProjectConfig(projectId)
    } finally {
      loading.value = false
    }
  }

  async function editConfig(projectId, data) {
    await updateProjectConfig(projectId, data)
    await fetchConfigDetail(projectId)
  }

  async function editRules(projectId, rules) {
    await updateProjectRules(projectId, rules)
    await fetchConfigDetail(projectId)
  }

  return { configs, total, page, pageSize, loading, currentConfig, fetchConfigs, fetchConfigDetail, editConfig, editRules }
})
