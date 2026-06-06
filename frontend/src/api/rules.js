import client from './client'

export function listRules() {
  return client.get('/rules')
}

export function createRule(data) {
  return client.post('/rules', data)
}

export function updateRule(id, data) {
  return client.put(`/rules/${id}`, data)
}

export function deleteRule(id) {
  return client.delete(`/rules/${id}`)
}

export function toggleRule(id, enabled) {
  return client.put(`/rules/${id}/toggle`, { enabled })
}
