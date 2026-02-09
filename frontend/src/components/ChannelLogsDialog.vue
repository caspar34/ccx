<template>
  <v-dialog :model-value="modelValue" max-width="800" @update:model-value="$emit('update:modelValue', $event)">
    <v-card>
      <v-card-title class="d-flex align-center justify-space-between">
        <span>渠道日志 - {{ channelName }}</span>
        <div class="d-flex align-center ga-2">
          <v-btn size="x-small" :variant="autoRefresh ? 'flat' : 'outlined'" :color="autoRefresh ? 'primary' : ''" @click="autoRefresh = !autoRefresh">
            {{ autoRefresh ? '自动刷新中' : '自动刷新' }}
          </v-btn>
          <v-btn icon size="small" variant="text" @click="$emit('update:modelValue', false)">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </div>
      </v-card-title>
      <v-divider />
      <v-card-text class="pa-0" style="max-height: 500px; overflow-y: auto">
        <!-- Loading -->
        <div v-if="isLoading && !logs.length" class="d-flex justify-center py-8">
          <v-progress-circular indeterminate color="primary" />
        </div>

        <!-- Empty -->
        <div v-else-if="!logs.length" class="text-center py-8 text-medium-emphasis">
          <v-icon size="40">mdi-format-list-bulleted</v-icon>
          <div class="text-caption mt-2">暂无日志记录</div>
        </div>

        <!-- Log list -->
        <v-list v-else density="compact" class="pa-0">
          <template v-for="(log, i) in logs" :key="i">
            <v-list-item :class="{ 'bg-error-subtle': !log.success }" @click="toggleExpand(i)">
              <template #prepend>
                <v-chip :color="statusColor(log.statusCode)" size="x-small" variant="flat" class="mr-2 font-weight-bold" style="min-width: 36px; justify-content: center">
                  {{ log.statusCode || 'ERR' }}
                </v-chip>
              </template>
              <v-list-item-title class="d-flex align-center ga-2 text-body-2 flex-wrap">
                <span class="text-medium-emphasis text-caption">{{ formatTime(log.timestamp) }}</span>
                <span class="font-weight-medium">{{ log.model }}</span>
                <span class="text-caption text-medium-emphasis">{{ log.durationMs }}ms</span>
                <span class="text-caption text-medium-emphasis">{{ log.keyMask }}</span>
                <v-chip v-if="log.isRetry" size="x-small" color="warning" variant="tonal">重试</v-chip>
              </v-list-item-title>
            </v-list-item>
            <!-- 展开的错误详情 -->
            <v-expand-transition>
              <div v-if="expandedIndex === i && log.errorInfo" class="px-4 py-2 text-caption" style="background: rgba(var(--v-theme-surface-variant), 0.3); white-space: pre-wrap; word-break: break-all">
                {{ log.errorInfo }}
              </div>
            </v-expand-transition>
            <v-divider v-if="i < logs.length - 1" />
          </template>
        </v-list>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { api, type ChannelLogEntry } from '../services/api'

const props = defineProps<{
  modelValue: boolean
  channelIndex: number
  channelName: string
  channelType: 'messages' | 'responses' | 'gemini'
}>()

defineEmits<{
  (_e: 'update:modelValue', _v: boolean): void
}>()

const logs = ref<ChannelLogEntry[]>([])
const isLoading = ref(false)
const autoRefresh = ref(false)
const expandedIndex = ref<number | null>(null)
let timer: ReturnType<typeof setInterval> | null = null

const toggleExpand = (i: number) => {
  expandedIndex.value = expandedIndex.value === i ? null : i
}

const statusColor = (code: number): string => {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 400 && code < 500) return 'warning'
  return 'error'
}

const formatTime = (ts: string): string => {
  const d = new Date(ts)
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const fetchLogs = async () => {
  isLoading.value = true
  try {
    const res = await api.getChannelLogs(props.channelType, props.channelIndex)
    logs.value = res.logs || []
  } catch (e) {
    console.error('Failed to fetch channel logs:', e)
  } finally {
    isLoading.value = false
  }
}

const startPolling = () => {
  stopPolling()
  timer = setInterval(fetchLogs, 3000)
}
const stopPolling = () => { if (timer) { clearInterval(timer); timer = null } }

// 打开时加载，关闭时停止
watch(() => props.modelValue, (open) => {
  if (open) {
    logs.value = []
    expandedIndex.value = null
    fetchLogs()
    if (autoRefresh.value) startPolling()
  } else {
    stopPolling()
  }
})

watch(autoRefresh, (v) => {
  if (v && props.modelValue) startPolling()
  else stopPolling()
})

onUnmounted(() => stopPolling())
</script>

<style scoped>
.bg-error-subtle {
  background: rgba(var(--v-theme-error), 0.05);
}
</style>
