import client from './client'

export function listAIModels() {
  return client.get('/ai-models')
}

export function createAIModel(data) {
  return client.post('/ai-models', data)
}

export function updateAIModel(id, data) {
  return client.put(`/ai-models/${id}`, data)
}

export function deleteAIModel(id) {
  return client.delete(`/ai-models/${id}`)
}

export function setDefaultModel(id) {
  return client.post(`/ai-models/${id}/set-default`)
}
