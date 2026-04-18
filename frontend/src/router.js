import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Logs from './views/Logs.vue'
import Settings from './views/Settings.vue'

const routes = [
  { path: '/', name: 'dashboard', component: Dashboard },
  { path: '/logs', name: 'logs', component: Logs },
  { path: '/settings', name: 'settings', component: Settings },
]

export default createRouter({ history: createWebHashHistory(), routes })
