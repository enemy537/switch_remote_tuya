<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login } from '../api'
import { user } from '../router'

const router = useRouter()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    user.value = await login(email.value, password.value)
    router.push('/')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="panel narrow" @submit.prevent="submit">
    <h1>Log in</h1>
    <label>Email <input v-model="email" type="email" autocomplete="username" required /></label>
    <label>Password <input v-model="password" type="password" autocomplete="current-password" required /></label>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <button class="primary" :disabled="busy">Log in</button>
  </form>
</template>
