<template>
  <div class="project-detail">
    <n-card>
      <template #header>
        <n-space align="center">
          <n-text strong style="font-size: 18px;">
            {{ currentConfig?.project_name || '项目配置' }}
          </n-text>
          <n-tag
            v-if="currentConfig"
            :type="currentConfig.enabled !== false ? 'success' : 'default'"
            size="small"
          >
            {{ currentConfig.enabled !== false ? '启用' : '禁用' }}
          </n-tag>
        </n-space>
      </template>
    </n-card>

    <n-card style="margin-top: 16px;">
      <n-spin :show="configsStore.loading">
        <n-tabs type="line" v-model:value="activeTab">
          <!-- Tab 1: Basic Settings -->
          <n-tab-pane name="basic" tab="基本设置">
            <n-form
              ref="basicFormRef"
              :model="basicForm"
              label-placement="left"
              label-width="120"
              style="max-width: 640px; padding-top: 16px;"
            >
              <n-form-item label="AI 模型">
                <n-select
                  v-model:value="basicForm.ai_model_id"
                  :options="modelOptions"
                  placeholder="选择 AI 模型"
                  clearable
                />
              </n-form-item>

              <n-form-item label="自动提交">
                <n-space align="center" :size="12">
                  <n-switch v-model:value="basicForm.auto_submit" />
                  <n-text depth="3">
                    {{ basicForm.auto_submit ? '自动提交审查结果到 GitLab' : '手动审核后提交' }}
                  </n-text>
                </n-space>
              </n-form-item>

              <n-form-item label="跳过草稿 MR">
                <n-space align="center" :size="12">
                  <n-switch v-model:value="basicForm.skip_draft" />
                  <n-text depth="3">
                    {{ basicForm.skip_draft ? '跳过 Work in Progress 的 MR' : '审查所有 MR' }}
                  </n-text>
                </n-space>
              </n-form-item>

              <n-form-item label="目标分支">
                <n-dynamic-tags v-model:value="basicForm.target_branches" />
              </n-form-item>

              <n-form-item label="启用">
                <n-space align="center" :size="12">
                  <n-switch v-model:value="basicForm.enabled" />
                  <n-text depth="3">
                    {{ basicForm.enabled ? '该项目启用自动审查' : '该项目已禁用' }}
                  </n-text>
                </n-space>
              </n-form-item>

              <n-form-item>
                <n-button type="primary" :loading="basicSaving" @click="handleSaveBasic">
                  保存基本设置
                </n-button>
              </n-form-item>
            </n-form>
          </n-tab-pane>

          <!-- Tab 2: Ignore Paths -->
          <n-tab-pane name="paths" tab="忽略路径">
            <n-space vertical :size="16" style="padding-top: 16px; max-width: 640px;">
              <n-text depth="3">
                配置该项目中需要忽略的文件路径，支持 glob 模式（例如: *.test.js, dist/**, node_modules/**）
              </n-text>

              <n-space vertical :size="8">
                <n-card
                  v-for="(pathItem, index) in ignorePathsList"
                  :key="index"
                  size="small"
                >
                  <n-space align="center" justify="space-between" style="width: 100%;">
                    <n-text code>{{ pathItem }}</n-text>
                    <n-button
                      size="small"
                      type="error"
                      quaternary
                      @click="removeIgnorePath(index)"
                    >
                      移除
                    </n-button>
                  </n-space>
                </n-card>

                <n-empty v-if="!ignorePathsList.length" description="暂无忽略路径" />
              </n-space>

              <n-space :size="8">
                <n-input
                  v-model:value="newIgnorePath"
                  placeholder="输入路径模式，如: *.test.js"
                  style="flex: 1;"
                  @keyup.enter="addIgnorePath"
                />
                <n-button type="primary" @click="addIgnorePath" :disabled="!newIgnorePath.trim()">
                  添加
                </n-button>
              </n-space>

              <n-button type="primary" :loading="pathsSaving" @click="handleSavePaths">
                保存忽略路径
              </n-button>
            </n-space>
          </n-tab-pane>

          <!-- Tab 3: Rule Overrides -->
          <n-tab-pane name="rules" tab="规则覆盖">
            <n-space vertical :size="16" style="padding-top: 16px;">
              <n-text depth="3">
                为该项目单独配置规则的启用状态。选择"继承"将使用全局设置。
              </n-text>

              <n-data-table
                :columns="ruleColumns"
                :data="allRules"
                :loading="rulesStore.loading"
                :bordered="false"
                :pagination="false"
              />

              <n-button type="primary" :loading="rulesSaving" @click="handleSaveRules">
                保存规则覆盖
              </n-button>
            </n-space>
          </n-tab-pane>

          <!-- Tab 4: Custom Prompt -->
          <n-tab-pane name="prompt" tab="自定义提示词">
            <n-space vertical :size="16" style="padding-top: 16px; max-width: 800px;">
              <n-text depth="3">
                为该项目添加自定义提示词，将在 AI 审查时附加到系统提示中。可用于指定特殊的审查关注点或项目规范。
              </n-text>

              <n-input
                v-model:value="customPrompt"
                type="textarea"
                placeholder="输入自定义提示词..."
                :rows="12"
              />

              <n-button type="primary" :loading="promptSaving" @click="handleSavePrompt">
                保存自定义提示词
              </n-button>
            </n-space>
          </n-tab-pane>
        </n-tabs>
      </n-spin>
    </n-card>
  </div>
</template>

<script setup>
import { ref, computed, h, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NSelect } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { useProjectConfigsStore } from '../stores/projectConfigs'
import { useAIModelsStore } from '../stores/aiModels'
import { useRulesStore } from '../stores/rules'

const route = useRoute()
const message = useMessage()
const configsStore = useProjectConfigsStore()
const modelsStore = useAIModelsStore()
const rulesStore = useRulesStore()

const activeTab = ref('basic')
const basicSaving = ref(false)
const pathsSaving = ref(false)
const rulesSaving = ref(false)
const promptSaving = ref(false)
const basicFormRef = ref(null)

const projectId = computed(() => route.params.id)
const currentConfig = computed(() => configsStore.currentConfig)

// Basic form
const basicForm = ref({
  ai_model_id: null,
  auto_submit: false,
  skip_draft: false,
  target_branches: [],
  enabled: true,
})

// Ignore paths
const ignorePathsList = ref([])
const newIgnorePath = ref('')

// Rule overrides
const ruleOverrides = ref({})
const customPrompt = ref('')

const modelOptions = computed(() =>
  modelsStore.models.map(m => ({
    label: `${m.name} (${m.model_name})`,
    value: m.id,
  }))
)

const allRules = computed(() => rulesStore.rules)

const overrideOptions = [
  { label: '继承', value: 'inherit' },
  { label: '开启', value: 'on' },
  { label: '关闭', value: 'off' },
]

const ruleColumns = [
  {
    title: '规则名称',
    key: 'name',
    width: 200,
  },
  {
    title: '严重级别',
    key: 'severity',
    width: 100,
    render(row) {
      const typeMap = { ERROR: 'error', WARNING: 'warning', INFO: 'info' }
      return h('span', { style: `color: ${typeMap[row.severity] === 'error' ? '#d03050' : typeMap[row.severity] === 'warning' ? '#f0a020' : '#2080f0'}` }, row.severity)
    },
  },
  {
    title: '描述',
    key: 'description',
    ellipsis: { tooltip: true },
  },
  {
    title: '覆盖设置',
    key: 'override',
    width: 160,
    render(row) {
      return h(NSelect, {
        value: ruleOverrides.value[row.id] || 'inherit',
        options: overrideOptions,
        size: 'small',
        onUpdateValue: (val) => {
          ruleOverrides.value[row.id] = val
        },
      })
    },
  },
]

function syncFromConfig() {
  const config = currentConfig.value
  if (!config) return

  basicForm.value = {
    ai_model_id: config.ai_model_id || null,
    auto_submit: config.auto_submit || false,
    skip_draft: config.skip_draft || false,
    target_branches: [...(config.target_branches || [])],
    enabled: config.enabled !== false,
  }

  ignorePathsList.value = [...(config.ignore_paths || [])]
  customPrompt.value = config.custom_prompt || ''

  // Build rule overrides map
  const overrides = {}
  if (config.rule_overrides) {
    for (const [ruleId, state] of Object.entries(config.rule_overrides)) {
      overrides[ruleId] = state
    }
  }
  ruleOverrides.value = overrides
}

function addIgnorePath() {
  const path = newIgnorePath.value.trim()
  if (path && !ignorePathsList.value.includes(path)) {
    ignorePathsList.value.push(path)
    newIgnorePath.value = ''
  }
}

function removeIgnorePath(index) {
  ignorePathsList.value.splice(index, 1)
}

async function handleSaveBasic() {
  basicSaving.value = true
  try {
    await configsStore.editConfig(projectId.value, basicForm.value)
    message.success('基本设置已保存')
  } catch (e) {
    message.error(e.message || '保存失败')
  } finally {
    basicSaving.value = false
  }
}

async function handleSavePaths() {
  pathsSaving.value = true
  try {
    await configsStore.editConfig(projectId.value, {
      ignore_paths: ignorePathsList.value,
    })
    message.success('忽略路径已保存')
  } catch (e) {
    message.error(e.message || '保存失败')
  } finally {
    pathsSaving.value = false
  }
}

async function handleSaveRules() {
  rulesSaving.value = true
  try {
    await configsStore.editRules(projectId.value, ruleOverrides.value)
    message.success('规则覆盖已保存')
  } catch (e) {
    message.error(e.message || '保存失败')
  } finally {
    rulesSaving.value = false
  }
}

async function handleSavePrompt() {
  promptSaving.value = true
  try {
    await configsStore.editConfig(projectId.value, {
      custom_prompt: customPrompt.value,
    })
    message.success('自定义提示词已保存')
  } catch (e) {
    message.error(e.message || '保存失败')
  } finally {
    promptSaving.value = false
  }
}

watch(currentConfig, syncFromConfig, { immediate: true })

onMounted(async () => {
  await Promise.all([
    configsStore.fetchConfigDetail(projectId.value),
    modelsStore.fetchModels(),
    rulesStore.fetchRules(),
  ])
})
</script>

<style scoped>
.project-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
</style>
