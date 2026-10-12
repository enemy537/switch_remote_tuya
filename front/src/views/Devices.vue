<script setup>
import { ref, onMounted } from 'vue'
import * as api from '../api'

const devices = ref([])
const loading = ref(false)
const error = ref('')
const pending = ref(new Set())

async function load() {
  loading.value = true
  error.value = ''
  try {
    devices.value = await api.listDevices()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function toggle(d, on) {
  error.value = ''
  pending.value.add(d.id)
  try {
    await api.switchDevice(d.id, on)
    d.on = on
  } catch (e) {
    error.value = e.message
  } finally {
    pending.value.delete(d.id)
  }
}

async function remove(d) {
  if (!confirm(`Remove "${d.name}"?`)) return
  error.value = ''
  try {
    await api.removeDevice(d.id)
    devices.value = devices.value.filter((x) => x.id !== d.id)
  } catch (e) {
    error.value = e.message
  }
}

// Add device panel
const adding = ref(false)
const found = ref([])
const findLoading = ref(false)
const findError = ref('')
const picked = ref(null)
const name = ref('')

async function openAdd() {
  adding.value = true
  picked.value = null
  findLoading.value = true
  findError.value = ''
  try {
    found.value = await api.listTuyaDevices()
  } catch (e) {
    findError.value = e.message
  } finally {
    findLoading.value = false
  }
}

function pick(t) {
  picked.value = t
  name.value = t.name
}

async function register() {
  findError.value = ''
  try {
    const d = await api.addDevice(picked.value.id, name.value.trim())
    devices.value.push(d)
    found.value = found.value.filter((x) => x.id !== picked.value.id)
    picked.value = null
  } catch (e) {
    findError.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="row">
    <h1>My devices</h1>
    <span class="spacer"></span>
    <button @click="load" :disabled="loading">Refresh</button>
    <button class="primary" @click="adding ? (adding = false) : openAdd()">
      {{ adding ? 'Close' : 'Add device' }}
    </button>
  </div>

  <p v-if="error" class="error" role="alert">{{ error }}</p>

  <section v-if="adding" class="panel">
    <div class="row">
      <h2>Add device</h2>
      <span class="spacer"></span>
      <button @click="openAdd" :disabled="findLoading">Refresh</button>
    </div>
    <p v-if="findError" class="error" role="alert">{{ findError }}</p>
    <p v-if="findLoading" class="muted">Loading…</p>
    <p v-else-if="!found.length" class="muted">
      No new switches found. Pair the switch in the Smart Life app first, then refresh.
    </p>
    <ul v-else class="list">
      <li v-for="t in found" :key="t.id" class="row">
        <label class="grow">
          <input type="radio" name="tuya" :checked="picked?.id === t.id" @change="pick(t)" />
          {{ t.name }}
          <span class="muted">{{ t.online ? 'online' : 'offline' }}</span>
        </label>
      </li>
    </ul>
    <form v-if="picked" class="row" @submit.prevent="register">
      <label class="grow">Name <input v-model="name" required maxlength="100" /></label>
      <button class="primary">Register</button>
    </form>
  </section>

  <p v-if="loading && !devices.length" class="muted">Loading…</p>
  <p v-else-if="!devices.length && !error" class="muted">No devices yet. Click "Add device".</p>
  <ul class="list">
    <li v-for="d in devices" :key="d.id" class="panel row">
      <div class="grow">
        <strong>{{ d.name }}</strong>
        <div class="muted" :class="{ offline: !d.online }">{{ d.online ? 'Online' : 'Offline' }}</div>
      </div>
      <label class="switch">
        <input
          type="checkbox"
          role="switch"
          :checked="d.on"
          :disabled="!d.online || pending.has(d.id)"
          :aria-label="`Turn ${d.name} on or off`"
          @change="toggle(d, $event.target.checked); $event.target.checked = d.on"
        />
        <span class="track"></span>
      </label>
      <button class="danger" @click="remove(d)">Remove</button>
    </li>
  </ul>
</template>
