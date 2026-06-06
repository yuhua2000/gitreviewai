<template>
  <div class="mr-detail" v-if="mr">
    <n-card>
      <template #header>
        <n-space align="center">
          <n-text strong style="font-size: 18px;">{{ mr.title }}</n-text>
          <n-tag :type="stateType" size="small">{{ mr.state }}</n-tag>
          <n-tag :type="reviewStatusType" size="small">{{ reviewStatusLabel }}</n-tag>
        </n-space>
      </template>
      <template #header-extra>
        <n-button tag="a" :href="mr.web_url" target="_blank" type="primary" size="small">
          在 GitLab 中查看
        </n-button>
      </template>

      <n-descriptions bordered :column="2" label-placement="left" size="small">
        <n-descriptions-item label="源分支">
          <n-text code>{{ mr.source_branch }}</n-text>
        </n-descriptions-item>
        <n-descriptions-item label="目标分支">
          <n-text code>{{ mr.target_branch }}</n-text>
        </n-descriptions-item>
        <n-descriptions-item label="项目 ID">{{ mr.project_id }}</n-descriptions-item>
        <n-descriptions-item label="MR IID">{{ mr.mr_iid }}</n-descriptions-item>
      </n-descriptions>
    </n-card>

    <n-card style="margin-top: 16px;">
      <n-tabs type="line" default-value="comments">
        <n-tab-pane name="comments" :tab="`评论 (${pendingComments.length} 待提交)`">
          <n-space vertical :size="16">
            <n-button
              v-if="pendingComments.length > 0"
              type="primary"
              @click="handleSubmitAll"
              :loading="submitting"
            >
              提交所有待提交评论 ({{ pendingComments.length }})
            </n-button>
            <CommentList
              :comments="sortedComments"
              @submit="handleSubmitComment"
            />
          </n-space>
        </n-tab-pane>

        <n-tab-pane name="reports" :tab="`报告 (${pendingReports.length} 待提交)`">
          <n-space vertical :size="16">
            <n-button
              v-if="pendingReports.length > 0"
              type="primary"
              @click="handleSubmitAll"
              :loading="submitting"
            >
              提交所有待提交报告 ({{ pendingReports.length }})
            </n-button>
            <ReportCard
              v-for="report in sortedReports"
              :key="report.id"
              :report="report"
              @submit="handleSubmitReport"
            />
            <n-empty v-if="!sortedReports.length" description="暂无报告" />
          </n-space>
        </n-tab-pane>

        <n-tab-pane name="info" tab="详情">
          <n-card title="描述" size="small">
            <n-text v-if="mr.description">{{ mr.description }}</n-text>
            <n-text v-else depth="3">无描述</n-text>
          </n-card>
        </n-tab-pane>

        <n-tab-pane name="changes" tab="变更文件">
          <n-spin :show="mrsStore.changesLoading">
            <n-space v-if="!changesLoaded" justify="center" style="padding: 40px 0;">
              <n-button type="primary" @click="loadChanges" :loading="mrsStore.changesLoading">
                获取变更文件
              </n-button>
            </n-space>
            <template v-else>
              <n-list bordered>
                <n-list-item v-for="change in mrsStore.changes" :key="change.new_path">
                  <n-thing>
                    <template #header>
                      <n-space align="center" :size="8">
                        <n-tag :type="changeType(change)" size="small">{{ changeLabel(change) }}</n-tag>
                        <n-text code>{{ change.new_path }}</n-text>
                      </n-space>
                    </template>
                    <template #header-extra>
                      <n-text v-if="change.old_path !== change.new_path" depth="3" style="font-size: 12px;">
                        {{ change.old_path }}
                      </n-text>
                    </template>
                    <n-collapse v-if="change.diff" default-expanded-names="">
                      <n-collapse-item name="diff" title="查看 Diff">
                        <n-scrollbar style="max-height: 400px;">
                          <pre class="diff-content">{{ change.diff }}</pre>
                        </n-scrollbar>
                      </n-collapse-item>
                    </n-collapse>
                  </n-thing>
                </n-list-item>
              </n-list>
              <div v-if="mrsStore.changesTotal > mrsStore.changesPageSize" style="margin-top: 16px; display: flex; justify-content: center;">
                <n-pagination
                  v-model:page="changesCurrentPage"
                  :page-count="Math.ceil(mrsStore.changesTotal / mrsStore.changesPageSize)"
                  @update:page="handleChangesPageChange"
                />
              </div>
            </template>
          </n-spin>
        </n-tab-pane>

        <n-tab-pane name="history" tab="审计历史">
          <n-space vertical :size="12">
            <n-button
              type="primary"
              size="small"
              :loading="retrying"
              @click="handleRetry"
            >
              重新审查
            </n-button>
            <n-data-table
              :columns="reviewLogColumns"
              :data="reviewLogs"
              :bordered="false"
              size="small"
            />
            <n-empty v-if="!reviewLogs.length" description="暂无审计记录" />
          </n-space>
        </n-tab-pane>
      </n-tabs>
    </n-card>
  </div>
  <n-spin v-else size="large" style="display: flex; justify-content: center; margin-top: 100px;" />
</template>

<script setup>
import { computed, onMounted, ref, h } from 'vue'
import { useRoute } from 'vue-router'
import { useMessage, NTag } from 'naive-ui'
import { useMrsStore } from '../stores/mrs'
import { getReviewLogs, retryReview } from '../api/mrs'
import CommentList from '../components/CommentList.vue'
import ReportCard from '../components/ReportCard.vue'

const route = useRoute()
const message = useMessage()
const mrsStore = useMrsStore()
const submitting = ref(false)
const retrying = ref(false)
const changesCurrentPage = ref(1)
const changesLoaded = ref(false)
const reviewLogs = ref([])

const mr = computed(() => mrsStore.currentMR)

const sortedComments = computed(() =>
  [...(mr.value?.comments || [])].reverse()
)

const sortedReports = computed(() =>
  [...(mr.value?.reports || [])].reverse()
)

const pendingComments = computed(() =>
  (mr.value?.comments || []).filter(c => c.status === 'pending')
)

const pendingReports = computed(() =>
  (mr.value?.reports || []).filter(r => r.status === 'pending')
)

const stateType = computed(() => {
  const map = { open: 'success', closed: 'warning', merged: 'info' }
  return map[mr.value?.state] || 'default'
})

const reviewStatusType = computed(() => {
  const map = { pending: 'default', reviewing: 'warning', completed: 'success', failed: 'error' }
  return map[mr.value?.review_status] || 'default'
})

const reviewStatusLabel = computed(() => {
  const map = { pending: '待审核', reviewing: '审核中', completed: '已完成', failed: '失败' }
  return map[mr.value?.review_status] || mr.value?.review_status
})

async function handleSubmitComment(commentId) {
  try {
    const result = await mrsStore.submitComment(commentId)
    if (result.warning) {
      message.warning(result.warning)
    } else {
      message.success('评论已提交')
    }
  } catch (e) {
    message.error(e.message || '提交失败')
  }
}

async function handleSubmitReport(reportId) {
  try {
    const result = await mrsStore.submitReport(reportId)
    if (result.warning) {
      message.warning(result.warning)
    } else {
      message.success('报告已提交')
    }
  } catch (e) {
    message.error(e.message || '提交失败')
  }
}

async function handleSubmitAll() {
  submitting.value = true
  try {
    const result = await mrsStore.submitAll(mr.value.id)
    message.success(`已提交 ${result.submitted_comments} 条评论和 ${result.submitted_reports} 份报告`)
    if (result.errors?.length) {
      result.errors.forEach(err => message.warning(err))
    }
  } catch (e) {
    message.error(e.message || '提交失败')
  } finally {
    submitting.value = false
  }
}

function changeType(change) {
  if (change.new_file) return 'success'
  if (change.deleted_file) return 'error'
  if (change.renamed_file) return 'warning'
  return 'default'
}

function changeLabel(change) {
  if (change.new_file) return '新增'
  if (change.deleted_file) return '删除'
  if (change.renamed_file) return '重命名'
  return '修改'
}

async function loadChanges() {
  changesLoaded.value = true
  await mrsStore.fetchMRChanges(mr.value.id, 1, 20)
}

function handleChangesPageChange(page) {
  mrsStore.fetchMRChanges(mr.value.id, page, 20)
}

const reviewLogColumns = [
  {
    title: '时间',
    key: 'created_at',
    width: 150,
    render(row) {
      return formatTime(row.created_at)
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    render(row) {
      const type = { running: 'warning', success: 'success', failed: 'error' }[row.status] || 'default'
      const label = { running: '运行中', success: '成功', failed: '失败' }[row.status] || row.status
      return h(NTag, { type, size: 'small' }, () => label)
    },
  },
  {
    title: '模型',
    key: 'model_name',
    width: 120,
  },
  {
    title: '规则',
    key: 'rules_count',
    width: 60,
  },
  {
    title: '评论',
    key: 'comments_count',
    width: 60,
  },
  {
    title: 'Token',
    key: 'total_tokens',
    width: 90,
    render(row) {
      return row.total_tokens > 0 ? row.total_tokens.toLocaleString() : '-'
    },
  },
  {
    title: '耗时',
    key: 'duration_ms',
    width: 80,
    render(row) {
      return row.duration_ms > 0 ? `${(row.duration_ms / 1000).toFixed(1)}s` : '-'
    },
  },
  {
    title: '错误信息',
    key: 'error_message',
    ellipsis: { tooltip: true },
  },
]

function formatTime(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  const pad = n => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function loadReviewLogs() {
  try {
    reviewLogs.value = await getReviewLogs(route.params.id) || []
  } catch {
    reviewLogs.value = []
  }
}

async function handleRetry() {
  retrying.value = true
  try {
    await retryReview(route.params.id)
    message.success('审查已重新启动，请稍后刷新查看结果')
    setTimeout(() => loadReviewLogs(), 2000)
  } catch (e) {
    message.error(e.message || '重试失败')
  } finally {
    retrying.value = false
  }
}

onMounted(() => {
  mrsStore.fetchMRDetail(route.params.id)
  loadReviewLogs()
})
</script>

<style scoped>
.mr-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.diff-content {
  background: #f8f9fa;
  border-radius: 6px;
  padding: 12px;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
  overflow-x: auto;
  white-space: pre;
  margin: 0;
}
</style>
