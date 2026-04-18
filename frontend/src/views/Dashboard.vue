<script setup>
import { onMounted, ref, computed } from 'vue'
import { Call, Events } from '@wailsio/runtime'
import { useMonitorStore } from '../stores/monitor'

const monitor = useMonitorStore()
const expanded = ref({})
const pingResults = ref({})

onMounted(async () => {
  await monitor.fetchStatus()
  Events.On('interface:status-changed', (event) => {
    monitor.updateInterfaceStatus(event.data)
  })
})

const uptime = computed(() => {
  if (!monitor.status.startedAt) return '—'
  const ms = Date.now() - new Date(monitor.status.startedAt).getTime()
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m ${s % 60}s`
})

function toggle(name) { expanded.value[name] = !expanded.value[name] }



async function pingRoute(host) {
  pingResults.value[host] = { loading: true }
  try {
    const result = await Call.ByName('main.App.PingRoute', host)
    pingResults.value[host] = result
  } catch (e) {
    pingResults.value[host] = { reachable: false, error: e.toString() }
  }
}
</script>

<template>
  <div>
    <h1>{{ $t('dashboard.title') }}</h1>
    <div style="display: flex; gap: 16px; margin-bottom: 20px;">
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">{{ $t('dashboard.status') }}</div>
        <span class="badge" :class="monitor.status.running ? 'badge-success' : 'badge-error'">
          {{ monitor.status.running ? $t('dashboard.running') : $t('dashboard.stopped') }}
        </span>
      </div>
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">{{ $t('dashboard.active') }}</div>
        <div style="font-size: 20px; font-weight: 600;">{{ monitor.activeCount }} / {{ monitor.status.interfaces.length }}</div>
      </div>
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">{{ $t('dashboard.routes') }}</div>
        <div style="font-size: 20px; font-weight: 600;">{{ monitor.totalRoutes }}</div>
      </div>
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">{{ $t('dashboard.uptime') }}</div>
        <div style="font-size: 20px; font-weight: 600;">{{ uptime }}</div>
      </div>
    </div>

    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
      <h2 style="margin-bottom: 0;">{{ $t('dashboard.interfaces') }}</h2>
      <div style="display: flex; gap: 8px;">
        <button class="btn btn-primary" v-if="!monitor.status.running" @click="monitor.startMonitor()">{{ $t('dashboard.start') }}</button>
        <button class="btn btn-danger" v-if="monitor.status.running" @click="monitor.stopMonitor()">{{ $t('dashboard.stop') }}</button>
      </div>
    </div>

    <div v-if="monitor.status.interfaces.length === 0" class="card">
      <p style="color: var(--text-secondary); text-align: center; padding: 20px;">{{ $t('dashboard.noInterfaces') }}</p>
    </div>

    <div v-for="iface in monitor.status.interfaces" :key="iface.interfaceName" class="card">
      <div style="display: flex; align-items: center; justify-content: space-between; cursor: pointer;" @click="toggle(iface.interfaceName)">
        <div style="display: flex; align-items: center; gap: 10px;">
          <span style="font-size: 10px;" :style="{ color: iface.connected ? 'var(--success)' : 'var(--error)' }">●</span>
          <span style="font-weight: 600;">{{ iface.interfaceName }}</span>
          <span class="badge" :class="iface.connected ? 'badge-success' : 'badge-error'">
            {{ iface.connected ? $t('dashboard.connected') : $t('dashboard.disconnected') }}
          </span>
        </div>
        <div style="display: flex; align-items: center; gap: 12px;">
          <span style="color: var(--text-secondary); font-size: 13px;">{{ iface.routes?.length || 0 }} {{ $t('dashboard.routeCount') }}</span>
          <span style="color: var(--text-secondary);">{{ expanded[iface.interfaceName] ? '▼' : '▶' }}</span>
        </div>
      </div>
      <div v-if="expanded[iface.interfaceName]" style="margin-top: 12px; border-top: 1px solid var(--border-color); padding-top: 12px;">
        <div v-if="iface.gateway" style="color: var(--text-secondary); font-size: 13px; margin-bottom: 8px;">
          {{ $t('dashboard.gateway') }} <span style="color: var(--text-primary); user-select: text;">{{ iface.gateway }}</span>
        </div>
        <div v-for="route in iface.routes" :key="route.ip" style="display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px;">
          <span style="font-size: 8px;" :style="{ color: route.active ? 'var(--success)' : 'var(--text-secondary)' }">●</span>
          <span style="color: var(--text-link); font-family: var(--font-mono); user-select: text;">{{ route.for }}</span>
          <span v-if="route.for !== route.ip" style="color: var(--text-secondary); user-select: text;">→ {{ route.ip }}</span>
          <button class="btn" style="font-size: 11px; padding: 2px 8px; margin-left: auto;" @click.stop="pingRoute(route.for)">
            {{ pingResults[route.for]?.loading ? '...' : $t('dashboard.ping') }}
          </button>
          <span v-if="pingResults[route.for] && !pingResults[route.for].loading" style="font-size: 12px;"
                :style="{ color: pingResults[route.for].reachable ? 'var(--success)' : 'var(--error)' }">
            {{ pingResults[route.for].reachable ? pingResults[route.for].latency : $t('dashboard.unreachable') }}
          </span>
        </div>
        <div v-if="!iface.routes?.length" style="color: var(--text-secondary); font-size: 13px;">{{ $t('dashboard.noRoutes') }}</div>
      </div>
    </div>
  </div>
</template>
