<script setup>
import { user } from './router'
import router from './router'
import { logout } from './api'

async function signOut() {
  await logout().catch(() => {})
  user.value = null
  router.push('/login')
}
</script>

<template>
  <header v-if="user">
    <router-link to="/" class="brand">Switch Remote</router-link>
    <nav>
      <router-link v-if="user.is_admin" to="/admin">Admin</router-link>
      <span class="muted email">{{ user.email }}</span>
      <button @click="signOut">Logout</button>
    </nav>
  </header>
  <main><router-view /></main>
</template>
