import router, { user } from './router'

async function request(method, path, body) {
  const res = await fetch('/api' + path, {
    method,
    credentials: 'same-origin',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 401 && path !== '/login') {
    user.value = null
    router.push('/login')
  }
  const data = res.status === 204 ? null : await res.json().catch(() => null)
  if (!res.ok) {
    const err = new Error(data?.error || `Request failed (${res.status})`)
    err.status = res.status
    throw err
  }
  return data
}

export const login = (email, password) => request('POST', '/login', { email, password })
export const logout = () => request('POST', '/logout')
export const getMe = () => request('GET', '/me')
export const listTuyaDevices = () => request('GET', '/tuya-devices')
export const listDevices = () => request('GET', '/devices')
export const addDevice = (id, name) => request('POST', '/devices', { id, name })
export const removeDevice = (id) => request('DELETE', `/devices/${encodeURIComponent(id)}`)
export const switchDevice = (id, on) => request('POST', `/devices/${encodeURIComponent(id)}/switch`, { on })
export const listUsers = () => request('GET', '/admin/users')
export const createUser = (u) => request('POST', '/admin/users', u)
export const resetPassword = (id, password) => request('PATCH', `/admin/users/${id}`, { password })
export const listAllDevices = (page) => request('GET', `/admin/devices?page=${page}`)
