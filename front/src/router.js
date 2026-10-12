import { ref } from 'vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import { getMe } from './api'
import Login from './views/Login.vue'
import Devices from './views/Devices.vue'
import Admin from './views/Admin.vue'

export const user = ref(null)
let loaded = false

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/login', component: Login },
    { path: '/', component: Devices },
    { path: '/admin', component: Admin },
  ],
})

router.beforeEach(async (to) => {
  if (!loaded) {
    loaded = true
    user.value = await getMe().catch(() => null)
  }
  if (to.path === '/login') return true
  if (!user.value) return '/login'
  if (to.path === '/admin' && !user.value.is_admin) return '/'
})

export default router
