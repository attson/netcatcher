<script setup>
import { onMounted } from 'vue'
import { Events, Window } from '@wailsio/runtime'
import { useMonitorStore } from './stores/monitor'
import { useLogStore } from './stores/logs'

const monitor = useMonitorStore()
const logStore = useLogStore()

function minimize() { Window.Minimise() }
function maximize() { Window.ToggleMaximise() }
function close() { Window.Close() }

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
    <div class="window-controls" style="--wails-draggable: none;">
      <button class="win-btn win-minimize" @click="minimize" title="Minimize">&minus;</button>
      <button class="win-btn win-maximize" @click="maximize" title="Maximize">&square;</button>
      <button class="win-btn win-close" @click="close" title="Close">&times;</button>
    </div>
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
