<template>
  <div class="projects-list">
    <n-card title="项目列表">
      <n-data-table
        :columns="columns"
        :data="store.configs"
        :loading="store.loading"
        :pagination="pagination"
        :row-props="rowProps"
        :bordered="false"
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
import { useProjectConfigsStore } from '../stores/projectConfigs'

const router = useRouter()
const store = useProjectConfigsStore()

const columns = [
  {
    title: '项目名称',
    key: 'project_name',
    ellipsis: { tooltip: true },
    width: 240,
  },
  {
    title: '描述',
    key: 'description',
    ellipsis: { tooltip: true },
    width: 280,
  },
  {
    title: '绑定模型',
    key: 'model_name',
    width: 150,
    render(row) {
      return row.model_name || row.ai_model?.name || '-'
    },
  },
  {
    title: '自动提交',
    key: 'auto_submit',
    width: 100,
    render(row) {
      return h(
        NTag,
        { type: row.auto_submit ? 'success' : 'default', size: 'small' },
        { default: () => (row.auto_submit ? '是' : '否') }
      )
    },
  },
  {
    title: '状态',
    key: 'enabled',
    width: 80,
    render(row) {
      return h(
        NTag,
        { type: row.enabled !== false ? 'success' : 'default', size: 'small' },
        { default: () => (row.enabled !== false ? '启用' : '禁用') }
      )
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    render(row) {
      return h(
        'a',
        {
          href: 'javascript:void(0)',
          onClick: (e) => {
            e.stopPropagation()
            router.push(`/projects/${row.project_id || row.id}`)
          },
          style: 'color: #18a058; cursor: pointer;',
        },
        '详情'
      )
    },
  },
]

const pagination = computed(() => ({
  page: store.page,
  pageSize: store.pageSize,
  itemCount: store.total,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
}))

const rowProps = (row) => ({
  style: 'cursor: pointer',
  onClick: () => {
    router.push(`/projects/${row.project_id || row.id}`)
  },
})

function handlePageChange(page) {
  store.fetchConfigs(page, store.pageSize)
}

function handlePageSizeChange(pageSize) {
  store.fetchConfigs(1, pageSize)
}

onMounted(() => {
  store.fetchConfigs()
})
</script>

<style scoped>
.projects-list {
  padding: 0;
}
</style>
