<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useConfigStore } from '../stores/config'

const { t } = useI18n()
const configStore = useConfigStore()
const newIfaceName = ref('')
const newRoutes = ref({})
const saveMessage = ref('')

onMounted(async () => { await configStore.fetchConfig() })

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
    saveMessage.value = t('routes.saved')
    setTimeout(() => { saveMessage.value = '' }, 2000)
  } catch (e) { saveMessage.value = t('routes.saveFailed') + e }
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
      <h1 style="margin-bottom: 0;">{{ $t('routes.title') }}</h1>
      <div style="display: flex; align-items: center; gap: 12px;">
        <span v-if="saveMessage" style="color: var(--success); font-size: 13px;">{{ saveMessage }}</span>
        <button class="btn btn-primary" @click="save" :disabled="configStore.saving">
          {{ configStore.saving ? $t('routes.saving') : $t('routes.saveApply') }}
        </button>
      </div>
    </div>
    <div style="display: flex; gap: 8px; margin-bottom: 20px;">
      <input v-model="newIfaceName" :placeholder="$t('routes.ifacePlaceholder')" @keyup.enter="addInterface" style="flex: 1;" />
      <button class="btn" @click="addInterface">{{ $t('routes.addInterface') }}</button>
    </div>
    <div v-if="configStore.config.interfaces.length === 0" class="card">
      <p style="color: var(--text-secondary); text-align: center; padding: 20px;">{{ $t('routes.noInterfaces') }}</p>
    </div>
    <div v-for="(iface, ifaceIdx) in configStore.config.interfaces" :key="ifaceIdx" class="card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
        <h2 style="margin-bottom: 0;">{{ iface.name }}</h2>
        <button class="btn btn-danger" @click="configStore.removeInterface(ifaceIdx)" style="font-size: 12px;">{{ $t('routes.remove') }}</button>
      </div>
      <div v-for="(route, routeIdx) in iface.routes" :key="routeIdx"
           style="display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; background: var(--bg-primary); border-radius: var(--radius); margin-bottom: 4px;">
        <span style="font-family: var(--font-mono); font-size: 13px; color: var(--text-link);">{{ route }}</span>
        <button @click="configStore.removeRoute(ifaceIdx, routeIdx)"
                style="background: none; border: none; color: var(--text-secondary); cursor: pointer; font-size: 16px; padding: 0 4px;" :title="$t('routes.removeRoute')">×</button>
      </div>
      <div style="display: flex; gap: 8px; margin-top: 8px;">
        <input v-model="newRoutes[ifaceIdx]" :placeholder="$t('routes.routePlaceholder')"
               @keyup.enter="addRoute(ifaceIdx)" style="flex: 1; font-size: 13px;" />
        <button class="btn" @click="addRoute(ifaceIdx)" style="font-size: 13px;">{{ $t('routes.add') }}</button>
      </div>
    </div>
  </div>
</template>
