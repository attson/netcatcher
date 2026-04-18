<script setup>
import { onMounted, ref, watch, nextTick } from 'vue'
import { useLogStore } from '../stores/logs'

const logStore = useLogStore()
const logContainer = ref(null)
const autoScroll = ref(true)

onMounted(async () => {
  await logStore.fetchRecent()
  window.wails.Events.On('log:new', (event) => { logStore.addEntry(event.data) })
  scrollToBottom()
})

watch(() => logStore.filtered.length, () => { if (autoScroll.value) nextTick(scrollToBottom) })

function scrollToBottom() { if (logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight }

function onScroll() {
  if (!logContainer.value) return
  const el = logContainer.value
  autoScroll.value = el.scrollHeight - el.scrollTop - el.clientHeight < 30
}

function formatTime(t) {
  const d = new Date(t)
  return d.toLocaleTimeString('en-US', { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

function levelColor(level) {
  switch (level) {
    case 'error': return 'var(--error)'
    case 'warn': return 'var(--warning)'
    case 'debug': return 'var(--text-secondary)'
    default: return 'var(--text-primary)'
  }
}
</script>

<template>
  <div style="display: flex; flex-direction: column; height: 100%;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
      <h1 style="margin-bottom: 0;">Logs</h1>
      <div style="display: flex; align-items: center; gap: 8px;">
        <input v-model="logStore.searchQuery" placeholder="Search..." style="width: 200px; font-size: 13px;" />
        <select v-model="logStore.levelFilter" style="font-size: 13px;">
          <option value="all">All levels</option>
          <option value="error">Error</option>
          <option value="warn">Warning</option>
          <option value="info">Info</option>
          <option value="debug">Debug</option>
        </select>
        <button class="btn" @click="logStore.clear()" style="font-size: 12px;">Clear</button>
      </div>
    </div>
    <div ref="logContainer" @scroll="onScroll"
         style="flex: 1; overflow-y: auto; background: var(--bg-secondary); border: 1px solid var(--border-color); border-radius: var(--radius); padding: 8px; font-family: var(--font-mono); font-size: 12px; line-height: 1.7;">
      <div v-if="logStore.filtered.length === 0" style="color: var(--text-secondary); text-align: center; padding: 40px;">No log entries</div>
      <div v-for="(entry, idx) in logStore.filtered" :key="idx" style="display: flex; gap: 8px; white-space: nowrap;">
        <span style="color: var(--text-secondary); flex-shrink: 0;">{{ formatTime(entry.time) }}</span>
        <span :style="{ color: levelColor(entry.level), flexShrink: 0, width: '44px', textAlign: 'right' }">[{{ entry.level }}]</span>
        <span style="white-space: pre-wrap; word-break: break-all;">{{ entry.message }}</span>
      </div>
    </div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 8px; font-size: 12px; color: var(--text-secondary);">
      <span>{{ logStore.filtered.length }} entries{{ logStore.levelFilter !== 'all' ? ' (filtered)' : '' }}</span>
      <span :style="{ color: autoScroll ? 'var(--success)' : 'var(--text-secondary)' }">
        {{ autoScroll ? 'Auto-scroll ON' : 'Auto-scroll paused — scroll to bottom to resume' }}
      </span>
    </div>
  </div>
</template>
