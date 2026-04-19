import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { Call } from '@wailsio/runtime'

export const useMonitorStore = defineStore('monitor', () => {
  const status = ref({ running: false, interfaces: [], startedAt: null })

  const activeCount = computed(() => status.value.interfaces.filter(i => i.connected).length)
  const totalRoutes = computed(() => status.value.interfaces.reduce((sum, i) => sum + (i.routes?.length || 0), 0))

  async function fetchStatus() {
    try {
      const result = await Call.ByName('main.App.GetStatus')
      status.value = result
    } catch (e) { console.error('fetchStatus failed:', e) }
  }

  async function startMonitor() {
    await Call.ByName('main.App.StartMonitor')
    await fetchStatus()
  }

  async function stopMonitor() {
    await Call.ByName('main.App.StopMonitor')
    await fetchStatus()
  }

  function updateInterfaceStatus(ifaceStatus) {
    const idx = status.value.interfaces.findIndex(i => i.interfaceName === ifaceStatus.interfaceName)
    if (idx >= 0) {
      status.value.interfaces[idx] = ifaceStatus
    } else {
      status.value.interfaces.push(ifaceStatus)
    }
  }

  return { status, activeCount, totalRoutes, fetchStatus, startMonitor, stopMonitor, updateInterfaceStatus }
})
