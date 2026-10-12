<script setup>
import { ref, onMounted } from 'vue'
import * as api from '../api'

const error = ref('')

// Users
const users = ref([])
const form = ref({ email: '', password: '', tuya_uid: '', is_admin: false })

async function loadUsers() {
  try {
    users.value = await api.listUsers()
  } catch (e) {
    error.value = e.message
  }
}

async function create() {
  error.value = ''
  try {
    users.value.push(await api.createUser(form.value))
    form.value = { email: '', password: '', tuya_uid: '', is_admin: false }
  } catch (e) {
    error.value = e.message
  }
}

async function reset(u) {
  const password = prompt(`New password for ${u.email} (min 8 characters):`)
  if (!password) return
  error.value = ''
  try {
    await api.resetPassword(u.id, password)
    alert('Password updated.')
  } catch (e) {
    error.value = e.message
  }
}

// Devices
const devices = ref([])
const page = ref(1)
const total = ref(0)
const pageSize = ref(20)
const loading = ref(false)

async function loadDevices() {
  loading.value = true
  try {
    const r = await api.listAllDevices(page.value)
    devices.value = r.items
    total.value = r.total
    pageSize.value = r.page_size
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function go(p) {
  page.value = p
  loadDevices()
}

const lastPage = () => Math.max(1, Math.ceil(total.value / pageSize.value))

onMounted(() => {
  loadUsers()
  loadDevices()
})
</script>

<template>
  <h1>Admin</h1>
  <p v-if="error" class="error" role="alert">{{ error }}</p>

  <section>
    <h2>Users</h2>
    <div class="scroll">
      <table>
        <thead><tr><th>Email</th><th>Admin</th><th>Tuya UID</th><th>Devices</th><th></th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.email }}</td>
            <td>{{ u.is_admin ? 'yes' : 'no' }}</td>
            <td>{{ u.tuya_uid }}</td>
            <td>{{ u.device_count }}</td>
            <td><button @click="reset(u)">Reset password</button></td>
          </tr>
        </tbody>
      </table>
    </div>

    <form class="panel" @submit.prevent="create">
      <h3>Create user</h3>
      <label>Email <input v-model="form.email" type="email" required /></label>
      <label>Password <input v-model="form.password" type="password" minlength="8" required autocomplete="new-password" /></label>
      <label>Tuya UID <input v-model="form.tuya_uid" required /></label>
      <p class="muted">
        The Tuya UID comes from the Tuya developer console after linking the user's Smart Life account.
      </p>
      <label class="check"><input v-model="form.is_admin" type="checkbox" /> Admin</label>
      <button class="primary">Create user</button>
    </form>
  </section>

  <section>
    <div class="row">
      <h2>All devices</h2>
      <span class="spacer"></span>
      <button @click="loadDevices" :disabled="loading">Refresh</button>
    </div>
    <div class="scroll">
      <table>
        <thead><tr><th>Name</th><th>Device ID</th><th>Owner</th><th>Online</th><th>State</th></tr></thead>
        <tbody>
          <tr v-for="d in devices" :key="d.id">
            <td>{{ d.name }}</td>
            <td>{{ d.id }}</td>
            <td>{{ d.user_email }}</td>
            <td>{{ d.online ? 'online' : 'offline' }}</td>
            <td>{{ d.on ? 'on' : 'off' }}</td>
          </tr>
          <tr v-if="!devices.length"><td colspan="5" class="muted">No devices.</td></tr>
        </tbody>
      </table>
    </div>
    <div class="row">
      <button :disabled="page <= 1 || loading" @click="go(page - 1)">Previous</button>
      <span class="muted">Page {{ page }} of {{ lastPage() }}</span>
      <button :disabled="page >= lastPage() || loading" @click="go(page + 1)">Next</button>
    </div>
  </section>
</template>
