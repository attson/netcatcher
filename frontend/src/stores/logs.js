import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { Call } from '@wailsio/runtime'

export const useLogStore = defineStore('logs', () => {
  const entries = ref([])
  const levelFilter = ref('all')
  const searchQuery = ref('')
  const maxEntries = 1000

  const filtered = computed(() => entries.value.filter(e => {
    if (levelFilter.value !== 'all' && e.level !== levelFilter.value) return false
    if (searchQuery.value && !e.message.toLowerCase().includes(searchQuery.value.toLowerCase())) return false
    return true
  }))

  async function fetchRecent() {
    try {
      const result = await Call.ByName('main.App.GetRecentLogs', 200)
      entries.value = result || []
    } catch (e) { console.error('fetchRecent failed:', e) }
  }

  function addEntry(entry) {
    entries.value.push(entry)
    if (entries.value.length > maxEntries) entries.value = entries.value.slice(-maxEntries)
  }

  function clear() { entries.value = [] }

  return { entries, levelFilter, searchQuery, filtered, fetchRecent, addEntry, clear }
})
