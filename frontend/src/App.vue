<script setup>
import { onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { useMonitorStore } from './stores/monitor'
import { useLogStore } from './stores/logs'

const monitor = useMonitorStore()
const logStore = useLogStore()

onMounted(async () => {
  await monitor.fetchStatus()
  await logStore.fetchRecent()

  Events.On('monitor:started', () => { monitor.fetchStatus() })
  Events.On('monitor:stopped', () => { monitor.fetchStatus() })
})
</script>

<template>
  <div class="titlebar">
    <span class="titlebar-title">NetCatcher</span>
  </div>
  <div class="layout">
    <nav class="sidebar">
      <ul class="sidebar-nav">
        <li><router-link to="/">Dashboard</router-link></li>
        <li><router-link to="/routes">Routes</router-link></li>
        <li><router-link to="/logs">Logs</router-link></li>
        <li><router-link to="/settings">Settings</router-link></li>
      </ul>
      <div style="margin-top: auto; padding: 12px 16px; border-top: 1px solid var(--border-color);">
        <span class="badge" :class="monitor.status.running ? 'badge-success' : 'badge-error'" style="font-size: 11px;">
          {{ monitor.status.running ? 'Monitoring' : 'Stopped' }}
        </span>
      </div>
    </nav>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>
