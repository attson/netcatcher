<script setup>
import { onMounted, ref } from 'vue'
import { useConfigStore } from '../stores/config'

const configStore = useConfigStore()
const autoStart = ref(false)
const notifications = ref(true)
const loading = ref(true)

onMounted(async () => {
  await configStore.fetchConfigPath()
  try {
    const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetAutoStart' })
    autoStart.value = result
  } catch (e) { console.error('GetAutoStart failed:', e) }
  loading.value = false
})

async function toggleAutoStart() {
  autoStart.value = !autoStart.value
  try {
    await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'SetAutoStart', args: [autoStart.value] })
  } catch (e) {
    console.error('SetAutoStart failed:', e)
    autoStart.value = !autoStart.value
  }
}

function toggleNotifications() { notifications.value = !notifications.value }
</script>

<template>
  <div>
    <h1>Settings</h1>
    <div class="card">
      <h2>General</h2>
      <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-bottom: 1px solid var(--border-color);">
        <div>
          <div style="font-weight: 500;">Launch at startup</div>
          <div style="color: var(--text-secondary); font-size: 13px;">Automatically start NetCatcher when you log in</div>
        </div>
        <div class="toggle" :class="{ active: autoStart }" @click="toggleAutoStart"></div>
      </div>
      <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0;">
        <div>
          <div style="font-weight: 500;">Notifications</div>
          <div style="color: var(--text-secondary); font-size: 13px;">Show system notifications when interfaces connect or disconnect</div>
        </div>
        <div class="toggle" :class="{ active: notifications }" @click="toggleNotifications"></div>
      </div>
    </div>
    <div class="card" style="margin-top: 16px;">
      <h2>Configuration</h2>
      <div style="padding: 8px 0;">
        <div style="color: var(--text-secondary); font-size: 13px; margin-bottom: 4px;">Config file location</div>
        <div style="font-family: var(--font-mono); font-size: 13px; color: var(--text-link); background: var(--bg-primary); padding: 8px 12px; border-radius: var(--radius);">
          {{ configStore.configPath }}
        </div>
      </div>
    </div>
    <div class="card" style="margin-top: 16px;">
      <h2>About</h2>
      <div style="padding: 8px 0; color: var(--text-secondary); font-size: 13px;">
        <div style="margin-bottom: 4px;"><span style="color: var(--text-primary);">NetCatcher</span> — Network route manager</div>
        <div style="margin-bottom: 4px;">Version: 1.0.0</div>
        <div><a href="https://github.com/attson/netcatcher" target="_blank" style="color: var(--text-link); text-decoration: none;">GitHub Repository</a></div>
      </div>
    </div>
  </div>
</template>
