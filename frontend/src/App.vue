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
  <div class="layout">
    <nav class="sidebar">
      <ul class="sidebar-nav">
        <li><router-link to="/">{{ $t('nav.dashboard') }}</router-link></li>
        <li><router-link to="/routes">{{ $t('nav.routes') }}</router-link></li>
        <li><router-link to="/logs">{{ $t('nav.logs') }}</router-link></li>
        <li><router-link to="/settings">{{ $t('nav.settings') }}</router-link></li>
      </ul>
      <div style="margin-top: auto; padding: 12px 16px; border-top: 1px solid var(--border-color);">
        <span class="badge" :class="monitor.status.running ? 'badge-success' : 'badge-error'" style="font-size: 11px;">
          {{ monitor.status.running ? $t('status.monitoring') : $t('status.stopped') }}
        </span>
      </div>
    </nav>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>
