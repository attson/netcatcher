<script setup>
import { onMounted, ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Call } from '@wailsio/runtime'
import { useUpdaterStore } from '../stores/updater'

const { t, d } = useI18n()
const updater = useUpdaterStore()
const autoCheck = ref(true)
const showNotes = ref(false)

onMounted(async () => {
  await updater.init()
  try {
    const cfg = await Call.ByName('main.App.GetConfig')
    autoCheck.value = cfg?.updater?.autoCheck !== false
  } catch (e) {
    console.error('SettingsUpdate: GetConfig failed', e)
  }
})

const statusText = computed(() => {
  if (updater.isDev) return t('settings.update.devDisabled')
  const s = updater.state.status
  if (s === 'available') return t('settings.update.status.available', { version: updater.state.latestVersion })
  return t(`settings.update.status.${s}`)
})

const lastCheckedDisplay = computed(() => {
  const v = updater.state.lastCheckedAt
  if (!v || v.startsWith('0001-')) return t('settings.update.never')
  try { return d(new Date(v), 'short') } catch { return v }
})

async function toggleAutoCheck() {
  autoCheck.value = !autoCheck.value
  await updater.setAutoCheck(autoCheck.value)
}
async function onCheck()   { await updater.check(true) }
async function onDownload(){ await updater.download() }
async function onInstall() { await updater.installAndQuit() }
async function onUnskip()  { await updater.unskip() }
</script>

<template>
  <div class="card" style="margin-top: 16px;">
    <h2>{{ t('settings.update.title') }}</h2>

    <div style="padding: 8px 0; font-size: 13px;">
      <div><span style="color: var(--text-secondary);">{{ t('settings.update.currentVersion') }}:</span> {{ updater.state.currentVersion }}</div>
      <div>
        <span style="color: var(--text-secondary);">{{ t('settings.update.latestVersion') }}:</span>
        <span style="margin-left: 4px;">{{ updater.state.latestVersion || '—' }}</span>
        <span style="margin-left: 8px; color: var(--text-secondary);">({{ statusText }})</span>
      </div>
      <div><span style="color: var(--text-secondary);">{{ t('settings.update.lastChecked') }}:</span> {{ lastCheckedDisplay }}</div>
      <div v-if="updater.state.skippedVersion" style="margin-top: 4px;">
        {{ t('settings.update.skipped', { version: updater.state.skippedVersion }) }}
        <button class="banner-btn" style="margin-left: 8px;" @click="onUnskip">{{ t('settings.update.unskip') }}</button>
      </div>
      <div v-if="updater.state.error" style="color: var(--error, #ef4444); margin-top: 4px;">{{ updater.state.error }}</div>
    </div>

    <div style="display: flex; gap: 8px; padding: 8px 0; flex-wrap: wrap;">
      <button class="banner-btn" :disabled="updater.isDev || updater.state.status === 'checking'" @click="onCheck">{{ t('settings.update.checkNow') }}</button>
      <button class="banner-btn" :disabled="updater.state.status !== 'available'" @click="onDownload">{{ t('settings.update.download') }}</button>
      <button class="banner-btn banner-btn-primary" :disabled="updater.state.status !== 'ready'" @click="onInstall">{{ t('settings.update.installRestart') }}</button>
    </div>

    <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-top: 1px solid var(--border-color);">
      <div>
        <div style="font-weight: 500;">{{ t('settings.update.autoCheck') }}</div>
        <div style="color: var(--text-secondary); font-size: 13px;">{{ t('settings.update.autoCheckDesc') }}</div>
      </div>
      <div class="toggle" :class="{ active: autoCheck }" @click="toggleAutoCheck"></div>
    </div>

    <div style="padding: 8px 0;">
      <button class="banner-btn" @click="showNotes = !showNotes">
        {{ showNotes ? t('banner.update.hideNotes') : t('settings.update.notes') }}
      </button>
      <pre v-if="showNotes" style="margin-top: 8px; padding: 8px 12px; background: var(--bg-primary); border-radius: var(--radius); font-family: var(--font-mono); font-size: 12px; white-space: pre-wrap; max-height: 240px; overflow-y: auto;">{{ updater.state.releaseNotes || t('settings.update.notesNone') }}</pre>
    </div>
  </div>
</template>

<style scoped>
.banner-btn {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid var(--border-color);
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
}
.banner-btn:hover:not(:disabled) { background: rgba(255,255,255,0.06); }
.banner-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.banner-btn-primary:not(:disabled) {
  background: var(--text-link);
  border-color: var(--text-link);
  color: #fff;
}
</style>
