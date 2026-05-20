import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { Call, Events } from '@wailsio/runtime'

const TERMINAL_BANNER_STATES = new Set(['available', 'downloading', 'ready'])

export const useUpdaterStore = defineStore('updater', () => {
  const state = ref({
    status: 'idle',
    currentVersion: 'dev',
    latestVersion: '',
    releaseNotes: '',
    releaseUrl: '',
    downloadPct: 0,
    assetSize: 0,
    error: '',
    lastCheckedAt: '',
    skippedVersion: '',
  })
  const dismissed = ref(false)
  const initialized = ref(false)

  const showBanner = computed(() => {
    if (dismissed.value) return false
    if (!TERMINAL_BANNER_STATES.has(state.value.status)) return false
    if (state.value.skippedVersion && state.value.skippedVersion === state.value.latestVersion) return false
    return true
  })

  const isDev = computed(() => state.value.currentVersion === 'dev')

  async function init() {
    if (initialized.value) return
    initialized.value = true
    try {
      const snap = await Call.ByName('main.App.GetUpdateState')
      state.value = { ...state.value, ...snap }
    } catch (e) {
      console.error('updater: GetUpdateState failed', e)
    }
    Events.On('update:state-changed', (e) => {
      const data = e.data
      const payload = Array.isArray(data) ? data[0] : data
      if (payload) state.value = { ...state.value, ...payload }
    })
  }

  async function check(force = true) {
    try { await Call.ByName('main.App.CheckUpdate', force) }
    catch (e) { console.error('updater: CheckUpdate failed', e) }
  }
  async function download() {
    try { await Call.ByName('main.App.StartDownload') }
    catch (e) { console.error('updater: StartDownload failed', e) }
  }
  async function installAndQuit() {
    try { await Call.ByName('main.App.InstallAndQuit') }
    catch (e) { console.error('updater: InstallAndQuit failed', e) }
  }
  async function skip(version) {
    try { await Call.ByName('main.App.SkipVersion', version) }
    catch (e) { console.error('updater: SkipVersion failed', e) }
  }
  async function unskip() {
    try { await Call.ByName('main.App.SkipVersion', '') }
    catch (e) { console.error('updater: SkipVersion("") failed', e) }
  }
  async function setAutoCheck(enabled) {
    try { await Call.ByName('main.App.SetAutoCheck', enabled) }
    catch (e) { console.error('updater: SetAutoCheck failed', e) }
  }
  function dismiss() { dismissed.value = true }

  return {
    state,
    dismissed,
    showBanner,
    isDev,
    init,
    check,
    download,
    installAndQuit,
    skip,
    unskip,
    setAutoCheck,
    dismiss,
  }
})
