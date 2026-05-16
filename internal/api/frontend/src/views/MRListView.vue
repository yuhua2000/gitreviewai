<template>
  <div class="mr-list">
    <n-card title="合并请求列表">
      <n-data-table
        :columns="columns"
        :data="mrsStore.mrs"
        :loading="mrsStore.loading"
        :pagination="pagination"
        :row-props="rowProps"
        @update:page="handlePageChange"
        @update:page-size="handlePageSizeChange"
      />
    </n-card>
  </div>
</template>

<script setup>
import { h, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { NTag } from 'naive-ui'
import { useMrsStore } from '../stores/mrs'

const router = useRouter()
const mrsStore = useMrsStore()

const columns = [
  {
    title: '标题',
    key: 'title',
    ellipsis: { tooltip: true },
    width: 300,
  },
  {
    title: '源分支',
    key: 'source_branch',
    width: 150,
    ellipsis: { tooltip: true },
  },
  {
    title: '目标分支',
    key: 'target_branch',
    width: 150,
    ellipsis: { tooltip: true },
  },
  {
    title: '状态',
    key: 'state',
    width: 80,
    render(row) {
      const typeMap = {
        open: 'success',
        closed: 'warning',
        merged: 'info',
      }
      return h(NTag, { type: typeMap[row.state] || 'default', size: 'small' }, { default: () => row.state })
    },
  },
  {
    title: '审核状态',
    key: 'review_status',
    width: 100,
    render(row) {
      const typeMap = {
        pending: 'default',
        reviewing: 'warning',
        completed: 'success',
        failed: 'error',
      }
      const labelMap = {
        pending: '待审核',
        reviewing: '审核中',
        completed: '已完成',
        failed: '失败',
      }
      return h(NTag, { type: typeMap[row.review_status] || 'default', size: 'small' }, { default: () => labelMap[row.review_status] || row.review_status })
    },
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 180,
    render(row) {
      return new Date(row.created_at).toLocaleString('zh-CN')
    },
  },
]

const pagination = computed(() => ({
  page: mrsStore.page,
  pageSize: mrsStore.pageSize,
  itemCount: mrsStore.total,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
}))

const rowProps = (row) => ({
  style: 'cursor: pointer',
  onClick: () => {
    router.push(`/mrs/${row.id}`)
  },
})

function handlePageChange(page) {
  mrsStore.fetchMRs(page, mrsStore.pageSize)
}

function handlePageSizeChange(pageSize) {
  mrsStore.fetchMRs(1, pageSize)
}

onMounted(() => {
  mrsStore.fetchMRs()
})
</script>

<style scoped>
.mr-list {
  padding: 0;
}
</style>
