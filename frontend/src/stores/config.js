import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useConfigStore = defineStore('config', () => {
  const config = ref({ interfaces: [] })
  const configPath = ref('')
  const saving = ref(false)

  async function fetchConfig() {
    try {
      const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetConfig' })
      config.value = result
    } catch (e) { console.error('fetchConfig failed:', e) }
  }

  async function fetchConfigPath() {
    try {
      const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetConfigPath' })
      configPath.value = result
    } catch (e) { console.error('fetchConfigPath failed:', e) }
  }

  async function saveConfig() {
    saving.value = true
    try {
      await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'SaveConfig', args: [config.value] })
    } catch (e) { console.error('saveConfig failed:', e); throw e }
    finally { saving.value = false }
  }

  function addInterface(name) { config.value.interfaces.push({ name, routes: [] }) }
  function removeInterface(index) { config.value.interfaces.splice(index, 1) }
  function addRoute(ifaceIndex, route) { config.value.interfaces[ifaceIndex].routes.push(route) }
  function removeRoute(ifaceIndex, routeIndex) { config.value.interfaces[ifaceIndex].routes.splice(routeIndex, 1) }

  return { config, configPath, saving, fetchConfig, fetchConfigPath, saveConfig, addInterface, removeInterface, addRoute, removeRoute }
})
