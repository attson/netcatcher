<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUpdaterStore } from '../stores/updater'

const { t } = useI18n()
const updater = useUpdaterStore()
const showNotes = ref(false)

const version = computed(() => updater.state.latestVersion)
const pct = computed(() => Math.max(0, Math.min(100, updater.state.downloadPct || 0)))

async function onDownload() { await updater.download() }
async function onInstall() { await updater.installAndQuit() }
async function onSkip()    { await updater.skip(version.value); updater.dismiss() }
function onLater()         { updater.dismiss() }
function toggleNotes()     { showNotes.value = !showNotes.value }
</script>

<template>
  <div v-if="updater.showBanner" class="update-banner" :data-status="updater.state.status">
    <div class="update-banner-row">
      <span class="update-banner-label">
        <template v-if="updater.state.status === 'available'">
          {{ t('banner.update.available', { version }) }}
        </template>
        <template v-else-if="updater.state.status === 'downloading'">
          {{ t('banner.update.downloading', { version }) }} {{ pct }}%
        </template>
        <template v-else-if="updater.state.status === 'ready'">
          {{ t('banner.update.ready', { version }) }}
        </template>
      </span>

      <span class="update-banner-actions">
        <template v-if="updater.state.status === 'available'">
          <button class="banner-btn" @click="toggleNotes">
            {{ showNotes ? t('banner.update.hideNotes') : t('banner.update.viewNotes') }}
          </button>
          <button class="banner-btn" @click="onSkip">{{ t('banner.update.skip') }}</button>
          <button class="banner-btn banner-btn-primary" @click="onDownload">{{ t('banner.update.download') }}</button>
          <button class="banner-btn" @click="onLater">{{ t('banner.update.later') }}</button>
        </template>
        <template v-else-if="updater.state.status === 'ready'">
          <button class="banner-btn banner-btn-primary" @click="onInstall">{{ t('banner.update.installRestart') }}</button>
          <button class="banner-btn" @click="onLater">{{ t('banner.update.later') }}</button>
        </template>
      </span>
    </div>

    <div v-if="updater.state.status === 'downloading'" class="update-banner-progress">
      <div class="update-banner-progress-fill" :style="{ width: pct + '%' }"></div>
    </div>

    <pre v-if="showNotes && updater.state.releaseNotes" class="update-banner-notes">{{ updater.state.releaseNotes }}</pre>
  </div>
</template>

<style scoped>
.update-banner {
  background: var(--bg-accent-soft, #1f2a3a);
  color: var(--text-primary, #e5e7eb);
  border-bottom: 1px solid var(--border-color, #2d3748);
  font-size: 13px;
}
.update-banner-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 16px;
  min-height: 32px;
}
.update-banner-label { font-weight: 500; }
.update-banner-actions { display: flex; gap: 8px; }
.banner-btn {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid var(--border-color, #2d3748);
  background: transparent;
  color: inherit;
  cursor: pointer;
}
.banner-btn:hover { background: rgba(255,255,255,0.06); }
.banner-btn-primary {
  background: var(--text-link, #3b82f6);
  border-color: var(--text-link, #3b82f6);
  color: #fff;
}
.update-banner-progress {
  height: 3px;
  background: rgba(255,255,255,0.08);
  overflow: hidden;
}
.update-banner-progress-fill {
  height: 100%;
  background: var(--text-link, #3b82f6);
  transition: width 200ms linear;
}
.update-banner-notes {
  max-height: 200px;
  overflow-y: auto;
  margin: 0;
  padding: 8px 16px;
  background: rgba(0,0,0,0.2);
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
  white-space: pre-wrap;
}
</style>
