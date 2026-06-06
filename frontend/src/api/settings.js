import client from './client'

export function getSettings() {
  return client.get('/settings')
}

export function updateSettings(data) {
  return client.put('/settings', data)
}
