import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listMRs, getMR, getMRChanges, submitComment as submitCommentApi, submitReport as submitReportApi, submitAll as submitAllApi } from '../api/mrs'

export const useMrsStore = defineStore('mrs', () => {
  const mrs = ref([])
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)
  const currentMR = ref(null)

  const changes = ref([])
  const changesTotal = ref(0)
  const changesPage = ref(1)
  const changesPageSize = ref(20)
  const changesLoading = ref(false)

  async function fetchMRs(p = 1, ps = 20) {
    loading.value = true
    try {
      const data = await listMRs(p, ps)
      mrs.value = data.data || []
      total.value = data.total || 0
      page.value = data.page || 1
      pageSize.value = data.page_size || 20
    } finally {
      loading.value = false
    }
  }

  async function fetchMRDetail(id) {
    loading.value = true
    try {
      const data = await getMR(id)
      currentMR.value = data
      return data
    } finally {
      loading.value = false
    }
  }

  async function submitComment(id) {
    const data = await submitCommentApi(id)
    if (currentMR.value) {
      const comment = currentMR.value.comments?.find(c => c.id === id)
      if (comment) {
        comment.status = 'submitted'
      }
    }
    return data
  }

  async function submitReport(id) {
    const data = await submitReportApi(id)
    if (currentMR.value) {
      const report = currentMR.value.reports?.find(r => r.id === id)
      if (report) {
        report.status = 'submitted'
      }
    }
    return data
  }

  async function submitAll(mrId) {
    const data = await submitAllApi(mrId)
    await fetchMRDetail(mrId)
    return data
  }

  async function fetchMRChanges(mrId, p = 1, ps = 20) {
    changesLoading.value = true
    try {
      const data = await getMRChanges(mrId, p, ps)
      changes.value = data.data || []
      changesTotal.value = data.total || 0
      changesPage.value = data.page || 1
      changesPageSize.value = data.page_size || 20
    } finally {
      changesLoading.value = false
    }
  }

  return {
    mrs,
    total,
    page,
    pageSize,
    loading,
    currentMR,
    changes,
    changesTotal,
    changesPage,
    changesPageSize,
    changesLoading,
    fetchMRs,
    fetchMRDetail,
    fetchMRChanges,
    submitComment,
    submitReport,
    submitAll,
  }
})
