import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listAIModels, createAIModel, updateAIModel, deleteAIModel, setDefaultModel } from '../api/aiModels'

export const useAIModelsStore = defineStore('aiModels', () => {
  const models = ref([])
  const loading = ref(false)

  async function fetchModels() {
    loading.value = true
    try {
      models.value = await listAIModels() || []
    } finally {
      loading.value = false
    }
  }

  async function addModel(data) {
    const model = await createAIModel(data)
    models.value.push(model)
    return model
  }

  async function editModel(id, data) {
    const model = await updateAIModel(id, data)
    const idx = models.value.findIndex(m => m.id === id)
    if (idx !== -1) models.value[idx] = model
    return model
  }

  async function removeModel(id) {
    await deleteAIModel(id)
    models.value = models.value.filter(m => m.id !== id)
  }

  async function makeDefault(id) {
    await setDefaultModel(id)
    models.value.forEach(m => { m.is_default = m.id === id })
  }

  return { models, loading, fetchModels, addModel, editModel, removeModel, makeDefault }
})
