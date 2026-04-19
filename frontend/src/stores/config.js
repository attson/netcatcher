import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Call } from '@wailsio/runtime'

export const useConfigStore = defineStore('config', () => {
  const config = ref({ interfaces: [] })
  const configPath = ref('')
  const saving = ref(false)

  async function fetchConfig() {
    try {
      const result = await Call.ByName('main.App.GetConfig')
      config.value = result
    } catch (e) { console.error('fetchConfig failed:', e) }
  }

  async function fetchConfigPath() {
    try {
      const result = await Call.ByName('main.App.GetConfigPath')
      configPath.value = result
    } catch (e) { console.error('fetchConfigPath failed:', e) }
  }

  async function saveConfig() {
    saving.value = true
    try {
      await Call.ByName('main.App.SaveConfig', config.value)
    } catch (e) { console.error('saveConfig failed:', e); throw e }
    finally { saving.value = false }
  }

  function addInterface(name) { config.value.interfaces.push({ name, routes: [], dns: [] }) }
  function removeInterface(index) { config.value.interfaces.splice(index, 1) }
  function addRoute(ifaceIndex, route) { config.value.interfaces[ifaceIndex].routes.push(route) }
  function removeRoute(ifaceIndex, routeIndex) { config.value.interfaces[ifaceIndex].routes.splice(routeIndex, 1) }
  function addDns(ifaceIndex, dns) {
    const iface = config.value.interfaces[ifaceIndex]
    if (!iface.dns) iface.dns = []
    iface.dns.push(dns)
  }
  function removeDns(ifaceIndex, dnsIndex) {
    const iface = config.value.interfaces[ifaceIndex]
    if (iface.dns) iface.dns.splice(dnsIndex, 1)
  }

  return { config, configPath, saving, fetchConfig, fetchConfigPath, saveConfig, addInterface, removeInterface, addRoute, removeRoute, addDns, removeDns }
})
