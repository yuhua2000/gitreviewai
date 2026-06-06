<template>
  <n-card size="small" :bordered="true">
    <template #header>
      <n-space align="center" :size="8">
        <n-tag :type="comment.comment_type === 'line' ? 'info' : 'warning'" size="small">
          {{ comment.comment_type === 'line' ? '行级评论' : '整体意见' }}
        </n-tag>
        <n-text v-if="comment.file_path" code style="font-size: 12px;">
          {{ comment.file_path }}:{{ comment.line_number }}
        </n-text>
        <StatusBadge :status="comment.status" />
        <n-text v-if="comment.created_at" depth="3" style="font-size: 12px;">
          {{ formatTime(comment.created_at) }}
        </n-text>
      </n-space>
    </template>
    <template #header-extra>
      <n-button
        v-if="comment.status === 'pending'"
        type="primary"
        size="small"
        @click="$emit('submit')"
      >
        提交
      </n-button>
    </template>

    <!-- Diff context (collapsible) -->
    <n-collapse v-if="comment.comment_type === 'line' && parsedDiffContext.length" default-expanded-names="">
      <n-collapse-item name="diff" title="代码上下文">
        <div class="diff-block">
          <div
            v-for="(line, idx) in parsedDiffContext"
            :key="idx"
            class="diff-line"
            :class="'diff-' + line.type"
          >
            <span class="diff-line-num">{{ line.type === 'del' ? '' : line.line }}</span>
            <span class="diff-prefix">{{ line.type === 'add' ? '+' : line.type === 'del' ? '-' : ' ' }}</span>
            <span class="diff-text">{{ line.text }}</span>
          </div>
        </div>
      </n-collapse-item>
    </n-collapse>

    <div class="markdown-body" v-html="renderedContent"></div>
  </n-card>
</template>

<script setup>
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  comment: {
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
  return md.render(props.comment.content || '')
})

const parsedDiffContext = computed(() => {
  if (!props.comment.diff_context) return []
  try {
    return JSON.parse(props.comment.diff_context)
  } catch {
    return []
  }
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

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 20px;
}

.diff-block {
  background: #f8f9fa;
  border-radius: 6px;
  padding: 8px 0;
  overflow-x: auto;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
}

.diff-line {
  display: flex;
  padding: 0 12px;
}

.diff-line-num {
  display: inline-block;
  width: 40px;
  text-align: right;
  padding-right: 12px;
  color: #8c959f;
  user-select: none;
  flex-shrink: 0;
}

.diff-prefix {
  display: inline-block;
  width: 16px;
  flex-shrink: 0;
  user-select: none;
}

.diff-text {
  white-space: pre;
}

.diff-context {
  color: #24292f;
}

.diff-add {
  background: #dafbe1;
  color: #1a7f37;
}

.diff-del {
  background: #ffebe9;
  color: #cf222e;
}
</style>
