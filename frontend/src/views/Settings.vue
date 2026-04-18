<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Call } from '@wailsio/runtime'
import { useConfigStore } from '../stores/config'

const { locale } = useI18n()
const configStore = useConfigStore()
const autoStart = ref(false)
const notifications = ref(true)
const currentLocale = ref(locale.value)
const loading = ref(true)

onMounted(async () => {
  await configStore.fetchConfigPath()
  try {
    const result = await Call.ByName('main.App.GetAutoStart')
    autoStart.value = result
  } catch (e) { console.error('GetAutoStart failed:', e) }
  loading.value = false
})

async function toggleAutoStart() {
  autoStart.value = !autoStart.value
  try {
    await Call.ByName('main.App.SetAutoStart', autoStart.value)
  } catch (e) {
    console.error('SetAutoStart failed:', e)
    autoStart.value = !autoStart.value
  }
}

function toggleNotifications() { notifications.value = !notifications.value }

function changeLocale() {
  locale.value = currentLocale.value
  localStorage.setItem('locale', currentLocale.value)
}
</script>

<template>
  <div>
    <h1>{{ $t('settings.title') }}</h1>
    <div class="card">
      <h2>{{ $t('settings.general') }}</h2>
      <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-bottom: 1px solid var(--border-color);">
        <div>
          <div style="font-weight: 500;">{{ $t('settings.launchStartup') }}</div>
          <div style="color: var(--text-secondary); font-size: 13px;">{{ $t('settings.launchStartupDesc') }}</div>
        </div>
        <div class="toggle" :class="{ active: autoStart }" @click="toggleAutoStart"></div>
      </div>
      <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-bottom: 1px solid var(--border-color);">
        <div>
          <div style="font-weight: 500;">{{ $t('settings.notifications') }}</div>
          <div style="color: var(--text-secondary); font-size: 13px;">{{ $t('settings.notificationsDesc') }}</div>
        </div>
        <div class="toggle" :class="{ active: notifications }" @click="toggleNotifications"></div>
      </div>
      <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0;">
        <div>
          <div style="font-weight: 500;">{{ $t('settings.language') }}</div>
          <div style="color: var(--text-secondary); font-size: 13px;">{{ $t('settings.languageDesc') }}</div>
        </div>
        <select v-model="currentLocale" @change="changeLocale" style="font-size: 13px; width: 120px;">
          <option value="en">English</option>
          <option value="zh-CN">中文</option>
        </select>
      </div>
    </div>
    <div class="card" style="margin-top: 16px;">
      <h2>{{ $t('settings.configuration') }}</h2>
      <div style="padding: 8px 0;">
        <div style="color: var(--text-secondary); font-size: 13px; margin-bottom: 4px;">{{ $t('settings.configLocation') }}</div>
        <div style="font-family: var(--font-mono); font-size: 13px; color: var(--text-link); background: var(--bg-primary); padding: 8px 12px; border-radius: var(--radius);">
          {{ configStore.configPath }}
        </div>
      </div>
    </div>
    <div class="card" style="margin-top: 16px;">
      <h2>{{ $t('settings.about') }}</h2>
      <div style="padding: 8px 0; color: var(--text-secondary); font-size: 13px;">
        <div style="margin-bottom: 4px;"><span style="color: var(--text-primary);">{{ $t('settings.appName') }}</span> {{ $t('settings.appDesc') }}</div>
        <div style="margin-bottom: 4px;">{{ $t('settings.version') }}</div>
        <div><a href="https://github.com/attson/netcatcher" target="_blank" style="color: var(--text-link); text-decoration: none;">{{ $t('settings.github') }}</a></div>
      </div>
    </div>
  </div>
</template>
