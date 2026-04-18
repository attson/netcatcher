<script setup>
import { onMounted, ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Call, Events } from '@wailsio/runtime'
import { useMonitorStore } from '../stores/monitor'
import { useConfigStore } from '../stores/config'

const { t } = useI18n()
const monitor = useMonitorStore()
const configStore = useConfigStore()
const expanded = ref({})
const pingResults = ref({})
const newIfaceName = ref('')
const newRoutes = ref({})
const saveMessage = ref('')
const systemInterfaces = ref([])
const savedSnapshot = ref('')
const dirty = computed(() => JSON.stringify(configStore.config) !== savedSnapshot.value)
const availableInterfaces = computed(() => {
  const configured = new Set(configStore.config.interfaces.map(i => i.name))
  return systemInterfaces.value.filter(name => !configured.has(name))
})

onMounted(async () => {
  await monitor.fetchStatus()
  await configStore.fetchConfig()
  savedSnapshot.value = JSON.stringify(configStore.config)
  try {
    systemInterfaces.value = await Call.ByName('main.App.GetNetworkInterfaces') || []
  } catch (e) { console.error('GetNetworkInterfaces failed:', e) }
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

function addInterface() {
  const name = newIfaceName.value.trim()
  if (!name) return
  configStore.addInterface(name)
  newIfaceName.value = ''
}

function addRoute(ifaceIndex) {
  const route = (newRoutes.value[ifaceIndex] || '').trim()
  if (!route) return
  if (!validateRoute(route)) return
  configStore.addRoute(ifaceIndex, route)
  newRoutes.value[ifaceIndex] = ''
}

function validateRoute(route) {
  const ipv4 = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/
  const cidr = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\/\d{1,2}$/
  const domain = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$/
  return ipv4.test(route) || cidr.test(route) || domain.test(route)
}

async function save() {
  try {
    await configStore.saveConfig()
    savedSnapshot.value = JSON.stringify(configStore.config)
    saveMessage.value = t('routes.saved')
    setTimeout(() => { saveMessage.value = '' }, 2000)
  } catch (e) { saveMessage.value = t('routes.saveFailed') + e }
}

function getMonitorIface(name) {
  return monitor.status.interfaces.find(i => i.interfaceName === name)
}

function getResolvedIps(ifaceName, routeName) {
  const iface = getMonitorIface(ifaceName)
  if (!iface?.routes) return []
  return iface.routes.filter(r => r.for === routeName && r.for !== r.ip).map(r => r.ip)
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
      <div style="display: flex; align-items: center; gap: 8px;">
        <span v-if="saveMessage" style="color: var(--success); font-size: 13px;">{{ saveMessage }}</span>
        <button v-if="dirty" class="btn btn-primary" @click="save" :disabled="configStore.saving" style="font-size: 12px;">
          {{ configStore.saving ? $t('routes.saving') : $t('routes.saveApply') }}
        </button>
        <button class="btn btn-primary" v-if="!monitor.status.running" @click="monitor.startMonitor()">{{ $t('dashboard.start') }}</button>
        <button class="btn btn-danger" v-if="monitor.status.running" @click="monitor.stopMonitor()">{{ $t('dashboard.stop') }}</button>
      </div>
    </div>

    <div v-if="availableInterfaces.length > 0" style="display: flex; gap: 8px; margin-bottom: 12px;">
      <select v-model="newIfaceName" style="flex: 1;">
        <option value="" disabled>{{ $t('routes.ifacePlaceholder') }}</option>
        <option v-for="name in availableInterfaces" :key="name" :value="name">{{ name }}</option>
      </select>
      <button class="btn" @click="addInterface">{{ $t('routes.addInterface') }}</button>
    </div>

    <div v-if="configStore.config.interfaces.length === 0" class="card">
      <p style="color: var(--text-secondary); text-align: center; padding: 20px;">{{ $t('dashboard.noInterfaces') }}</p>
    </div>

    <div v-for="(iface, ifaceIdx) in configStore.config.interfaces" :key="ifaceIdx" class="card">
      <div style="display: flex; align-items: center; justify-content: space-between; cursor: pointer;" @click="toggle(iface.name)">
        <div style="display: flex; align-items: center; gap: 10px;">
          <span style="font-size: 10px;" :style="{ color: getMonitorIface(iface.name)?.connected ? 'var(--success)' : 'var(--text-secondary)' }">●</span>
          <span style="font-weight: 600;">{{ iface.name }}</span>
          <span v-if="getMonitorIface(iface.name)?.connected" class="badge badge-success">
            {{ $t('dashboard.connected') }}
          </span>
        </div>
        <div style="display: flex; align-items: center; gap: 12px;">
          <span style="color: var(--text-secondary); font-size: 13px;">{{ iface.routes.length }} {{ $t('dashboard.routeCount') }}</span>
          <button class="btn btn-danger" @click.stop="configStore.removeInterface(ifaceIdx)" style="font-size: 11px; padding: 2px 8px;">{{ $t('routes.remove') }}</button>
          <span style="color: var(--text-secondary);">{{ expanded[iface.name] ? '▼' : '▶' }}</span>
        </div>
      </div>

      <div v-if="expanded[iface.name]" style="margin-top: 12px; border-top: 1px solid var(--border-color); padding-top: 12px;">
        <div v-if="getMonitorIface(iface.name)?.gateway"
             style="color: var(--text-secondary); font-size: 13px; margin-bottom: 8px;">
          {{ $t('dashboard.gateway') }}
          <span style="color: var(--text-primary); user-select: text;">{{ getMonitorIface(iface.name).gateway }}</span>
        </div>

        <div v-for="(route, routeIdx) in iface.routes" :key="routeIdx"
             style="display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px;">
          <span style="font-size: 8px; color: var(--text-secondary);">●</span>
          <span style="color: var(--text-link); font-family: var(--font-mono); user-select: text;">{{ route }}</span>
          <span v-if="getResolvedIps(iface.name, route).length" style="color: var(--text-secondary); user-select: text;">→ {{ getResolvedIps(iface.name, route).join(', ') }}</span>
          <button @click="configStore.removeRoute(ifaceIdx, routeIdx)"
                  style="background: none; border: none; color: var(--text-secondary); cursor: pointer; font-size: 16px; padding: 0 4px;" :title="$t('routes.removeRoute')">×</button>
          <button class="btn" style="font-size: 11px; padding: 2px 8px; margin-left: auto;" @click.stop="pingRoute(route)">
            {{ pingResults[route]?.loading ? '...' : $t('dashboard.ping') }}
          </button>
          <span v-if="pingResults[route] && !pingResults[route].loading" style="font-size: 12px;"
                :style="{ color: pingResults[route].reachable ? 'var(--success)' : 'var(--error)' }">
            {{ pingResults[route].reachable ? pingResults[route].latency : $t('dashboard.unreachable') }}
          </span>
        </div>

        <div style="display: flex; gap: 8px; margin-top: 8px;">
          <input v-model="newRoutes[ifaceIdx]" :placeholder="$t('routes.routePlaceholder')"
                 @keyup.enter="addRoute(ifaceIdx)" style="flex: 1; font-size: 13px;" />
          <button class="btn" @click="addRoute(ifaceIdx)" style="font-size: 13px;">{{ $t('routes.add') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
