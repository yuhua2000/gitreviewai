<template>
  <n-card size="small" :bordered="true">
    <template #header>
      <n-space align="center" :size="8">
        <n-text strong>审核报告</n-text>
        <StatusBadge :status="report.status" />
        <n-text v-if="report.created_at" depth="3" style="font-size: 12px;">
          {{ formatTime(report.created_at) }}
        </n-text>
      </n-space>
    </template>
    <template #header-extra>
      <n-button
        v-if="report.status === 'pending'"
        type="primary"
        size="small"
        @click="$emit('submit', report.id)"
      >
        提交
      </n-button>
    </template>
    <n-collapse default-expanded-names="">
      <n-collapse-item name="content" title="查看报告内容">
        <div class="markdown-body" v-html="renderedContent"></div>
      </n-collapse-item>
    </n-collapse>
  </n-card>
</template>

<script setup>
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  report: {
    type: Object,
    required: true,
  },
})

defineEmits(['submit'])

function formatTime(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  const pad = n => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const md = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: true,
  highlight(str, lang) {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return hljs.highlight(str, { language: lang }).value
      } catch (_) {}
    }
    return ''
  },
})

const renderedContent = computed(() => {
  return md.render(props.report.content || '')
})
</script>

<style scoped>
.markdown-body {
  font-size: 14px;
  line-height: 1.6;
}

.markdown-body :deep(pre) {
  background: #f6f8fa;
  border-radius: 6px;
  padding: 12px;
  overflow-x: auto;
}

.markdown-body :deep(code) {
  background: #f0f0f0;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

.markdown-body :deep(pre code) {
  background: none;
  padding: 0;
}

.markdown-body :deep(p) {
  margin: 8px 0;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin: 16px 0 8px;
  border-bottom: 1px solid #eaecef;
  padding-bottom: 6px;
}

.markdown-body :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 12px 0;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid #dfe2e5;
  padding: 8px 12px;
  text-align: left;
}

.markdown-body :deep(th) {
  background: #f6f8fa;
}

.markdown-body :deep(blockquote) {
  border-left: 4px solid #18a058;
  margin: 12px 0;
  padding: 8px 16px;
  background: #f9f9f9;
}
</style>
