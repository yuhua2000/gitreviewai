import client from './client'

export function login(password) {
  return client.post('/login', { password })
}
