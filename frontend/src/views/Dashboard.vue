<script setup>
import { onMounted, ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Call } from '@wailsio/runtime'
import { useMonitorStore } from '../stores/monitor'
import { useConfigStore } from '../stores/config'

const { t } = useI18n()
const monitor = useMonitorStore()
const configStore = useConfigStore()
const expanded = ref({})
const editing = ref({})
const pingResults = ref({})
const newIfaceName = ref('')
const newRoutes = ref({})
const newDns = ref({})
const saveMessage = ref('')
const systemInterfaces = ref([])
const savedSnapshot = ref('')
const availableInterfaces = computed(() => {
  const configured = new Set(configStore.config.interfaces.map(i => i.name))
  return systemInterfaces.value
    .filter(i => !configured.has(i.name))
    .sort((a, b) => {
      const aHas = a.label !== a.name ? 0 : 1
      const bHas = b.label !== b.name ? 0 : 1
      return aHas - bHas
    })
})

function getIfaceLabel(name) {
  const iface = systemInterfaces.value.find(i => i.name === name)
  return iface ? iface.label : name
}

onMounted(async () => {
  await monitor.fetchStatus()
  await configStore.fetchConfig()
  savedSnapshot.value = JSON.stringify(configStore.config)
  try {
    systemInterfaces.value = await Call.ByName('main.App.GetNetworkInterfaces') || []
  } catch (e) { console.error('GetNetworkInterfaces failed:', e) }
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

async function pingRoute(ifaceName, host) {
  pingResults.value[host] = { loading: true }
  try {
    const result = await Call.ByName('main.App.PingRoute', ifaceName, host)
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
  editing.value[name] = true
  expanded.value[name] = true
}

function startEdit(name) {
  editing.value[name] = true
  expanded.value[name] = true
}

async function applyEdit(name) {
  await save()
  if (saveMessage.value === t('routes.saved')) {
    editing.value[name] = false
  }
}

function cancelEdit(ifaceIdx) {
  const iface = configStore.config.interfaces[ifaceIdx]
  if (!iface) return
  const name = iface.name
  try {
    const snapshot = JSON.parse(savedSnapshot.value)
    const original = snapshot.interfaces.find(i => i.name === name)
    if (original) {
      configStore.config.interfaces[ifaceIdx] = JSON.parse(JSON.stringify(original))
    } else {
      configStore.config.interfaces.splice(ifaceIdx, 1)
    }
  } catch (e) { console.error('cancelEdit failed:', e) }
  newRoutes.value[ifaceIdx] = ''
  newDns.value[ifaceIdx] = ''
  editing.value[name] = false
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

function validateDns(dns) {
  const ipv4 = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/
  const ipv4Port = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d{1,5}$/
  return ipv4.test(dns) || ipv4Port.test(dns)
}

function addDns(ifaceIndex) {
  const dns = (newDns.value[ifaceIndex] || '').trim()
  if (!dns) return
  if (!validateDns(dns)) return
  configStore.addDns(ifaceIndex, dns)
  newDns.value[ifaceIndex] = ''
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

function isRouteActive(ifaceName, routeName) {
  if (!monitor.status.running) return false
  const iface = getMonitorIface(ifaceName)
  if (!iface?.connected) return false
  const entries = (iface.routes || []).filter(r => r.for === routeName)
  if (entries.length === 0) return false
  return entries.some(r => r.active)
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
        <button class="btn btn-primary" v-if="!monitor.status.running" @click="monitor.startMonitor()">{{ $t('dashboard.start') }}</button>
        <button class="btn btn-danger" v-if="monitor.status.running" @click="monitor.stopMonitor()">{{ $t('dashboard.stop') }}</button>
      </div>
    </div>

    <div v-if="availableInterfaces.length > 0" style="display: flex; gap: 8px; margin-bottom: 12px;">
      <select v-model="newIfaceName" style="flex: 1;">
        <option value="" disabled>{{ $t('routes.ifacePlaceholder') }}</option>
        <option v-for="iface in availableInterfaces" :key="iface.name" :value="iface.name">{{ iface.label }}</option>
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
          <span style="font-weight: 600;">{{ getIfaceLabel(iface.name) }}</span>
          <span v-if="getMonitorIface(iface.name)?.connected" class="badge badge-success">
            {{ $t('dashboard.connected') }}
          </span>
        </div>
        <div style="display: flex; align-items: center; gap: 8px;">
          <span style="color: var(--text-secondary); font-size: 13px;">{{ iface.routes.length }} {{ $t('dashboard.routeCount') }}</span>
          <template v-if="editing[iface.name]">
            <button class="btn btn-primary" @click.stop="applyEdit(iface.name)" :disabled="configStore.saving" style="font-size: 11px; padding: 2px 8px;">
              {{ configStore.saving ? $t('routes.saving') : $t('routes.saveApply') }}
            </button>
            <button class="btn" @click.stop="cancelEdit(ifaceIdx)" style="font-size: 11px; padding: 2px 8px;">{{ $t('routes.cancel') }}</button>
          </template>
          <template v-else>
            <button class="btn" @click.stop="startEdit(iface.name)" style="font-size: 11px; padding: 2px 8px;">{{ $t('routes.edit') }}</button>
            <button class="btn btn-danger" @click.stop="configStore.removeInterface(ifaceIdx)" style="font-size: 11px; padding: 2px 8px;">{{ $t('routes.remove') }}</button>
          </template>
          <span style="color: var(--text-secondary);">{{ expanded[iface.name] ? '▼' : '▶' }}</span>
        </div>
      </div>

      <div v-if="expanded[iface.name]" style="margin-top: 12px; border-top: 1px solid var(--border-color); padding-top: 12px;">
        <div style="display: flex; align-items: center; flex-wrap: wrap; gap: 12px; color: var(--text-secondary); font-size: 13px; margin-bottom: 8px;">
          <div v-if="getMonitorIface(iface.name)?.gateway">
            {{ $t('dashboard.gateway') }}
            <span style="color: var(--text-primary); user-select: text;">{{ getMonitorIface(iface.name).gateway }}</span>
          </div>
          <div style="display: flex; align-items: center; flex-wrap: wrap; gap: 6px;" :title="$t('routes.dnsHint')">
            <span>{{ $t('routes.dnsLabel') }}</span>
            <span v-for="(dns, dnsIdx) in (iface.dns || [])" :key="dnsIdx"
                  style="display: inline-flex; align-items: center; gap: 2px; padding: 1px 4px 1px 6px; border: 1px solid var(--border-color); border-radius: 3px; background: var(--bg-elevated); font-family: var(--font-mono); font-size: 12px; color: var(--text-primary);">
              {{ dns }}
              <button v-if="editing[iface.name]" @click="configStore.removeDns(ifaceIdx, dnsIdx)"
                      style="background: none; border: none; color: var(--text-secondary); cursor: pointer; font-size: 14px; padding: 0 2px; line-height: 1;" :title="$t('routes.removeDns')">×</button>
            </span>
            <span v-if="!editing[iface.name] && !(iface.dns || []).length" style="color: var(--text-secondary); font-size: 12px;">—</span>
            <template v-if="editing[iface.name]">
              <input v-model="newDns[ifaceIdx]" :placeholder="$t('routes.dnsPlaceholder')"
                     @keyup.enter="addDns(ifaceIdx)" style="width: 180px; font-size: 12px; padding: 2px 6px;" />
              <button class="btn" @click="addDns(ifaceIdx)" style="font-size: 12px; padding: 2px 8px;">{{ $t('routes.add') }}</button>
            </template>
          </div>
        </div>

        <div v-for="(route, routeIdx) in iface.routes" :key="routeIdx"
             style="display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px;"
             :style="{ opacity: isRouteActive(iface.name, route) ? 1 : 0.55 }">
          <span style="font-size: 8px;"
                :style="{ color: isRouteActive(iface.name, route) ? 'var(--success)' : 'var(--text-secondary)' }">●</span>
          <span style="font-family: var(--font-mono); user-select: text;"
                :style="{ color: isRouteActive(iface.name, route) ? 'var(--text-link)' : 'var(--text-secondary)', textDecoration: isRouteActive(iface.name, route) ? 'none' : 'line-through' }">{{ route }}</span>
          <span v-if="getResolvedIps(iface.name, route).length" style="color: var(--text-secondary); user-select: text;">→ {{ getResolvedIps(iface.name, route).join(', ') }}</span>
          <button v-if="editing[iface.name]" @click="configStore.removeRoute(ifaceIdx, routeIdx)"
                  style="background: none; border: none; color: var(--text-secondary); cursor: pointer; font-size: 16px; padding: 0 4px;" :title="$t('routes.removeRoute')">×</button>
          <button class="btn" style="font-size: 11px; padding: 2px 8px; margin-left: auto;" @click.stop="pingRoute(iface.name, route)">
            {{ pingResults[route]?.loading ? '...' : $t('dashboard.ping') }}
          </button>
          <span v-if="pingResults[route] && !pingResults[route].loading" style="font-size: 12px;"
                :style="{ color: pingResults[route].reachable ? 'var(--success)' : 'var(--error)' }">
            {{ pingResults[route].reachable ? pingResults[route].latency : $t('dashboard.unreachable') }}
          </span>
        </div>

        <div v-if="editing[iface.name]" style="display: flex; gap: 8px; margin-top: 8px;">
          <input v-model="newRoutes[ifaceIdx]" :placeholder="$t('routes.routePlaceholder')"
                 @keyup.enter="addRoute(ifaceIdx)" style="flex: 1; font-size: 13px;" />
          <button class="btn" @click="addRoute(ifaceIdx)" style="font-size: 13px;">{{ $t('routes.add') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
