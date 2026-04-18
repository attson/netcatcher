import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useMonitorStore = defineStore('monitor', () => {
  const status = ref({ running: false, interfaces: [], startedAt: null })

  const activeCount = computed(() => status.value.interfaces.filter(i => i.connected).length)
  const totalRoutes = computed(() => status.value.interfaces.reduce((sum, i) => sum + (i.routes?.length || 0), 0))

  async function fetchStatus() {
    try {
      const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetStatus' })
      status.value = result
    } catch (e) { console.error('fetchStatus failed:', e) }
  }

  async function startMonitor() {
    await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'StartMonitor' })
    await fetchStatus()
  }

  async function stopMonitor() {
    await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'StopMonitor' })
    await fetchStatus()
  }

  function updateInterfaceStatus(ifaceStatus) {
    const idx = status.value.interfaces.findIndex(i => i.interfaceName === ifaceStatus.interfaceName)
    if (idx >= 0) status.value.interfaces[idx] = ifaceStatus
  }

  return { status, activeCount, totalRoutes, fetchStatus, startMonitor, stopMonitor, updateInterfaceStatus }
})
