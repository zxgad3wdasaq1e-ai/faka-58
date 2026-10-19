<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { usePreferredReducedMotion } from '@vueuse/core'
import { Pause, Play } from 'lucide-vue-next'
import type { HomepageSettings } from '../../../../../shared/homepage'
import { useAppStore } from '../../../stores/app'
import { getLocalizedText } from '../../../utils/resellerSiteConfig'
import { createActivitySequence, formatActivityMessage } from '../activityTicker'
import { useGetgptCopy } from '../siteCopy'

const props = defineProps<{ settings: HomepageSettings }>()
const app = useAppStore()
const c = useGetgptCopy()
const reducedMotion = usePreferredReducedMotion()
const paused = ref(false)
const hovered = ref(false)
const index = ref(0)
const revision = ref(0)
const message = ref('')
const templates = computed(() => props.settings.activity_messages.map(item => ({
  kind: item.kind,
  text: getLocalizedText(item.text, app.locale),
})).filter(item => item.text))
const activeKind = computed(() => templates.value[index.value]?.kind || 'notice')
let sequence: number[] = []
let timer: ReturnType<typeof setInterval> | undefined

function updateMessage() {
  message.value = formatActivityMessage(templates.value[index.value]?.text || '', props.settings.activity_names)
  revision.value++
}

function restart() {
  if (timer) clearInterval(timer)
  timer = undefined
  index.value = 0
  sequence = []
  updateMessage()
  if (!props.settings.activity_enabled || !templates.value.length || reducedMotion.value === 'reduce') return
  timer = setInterval(() => {
    if (paused.value || hovered.value || document.hidden) return
    if (!sequence.length) sequence = createActivitySequence(templates.value.length, index.value)
    index.value = sequence.shift() ?? 0
    updateMessage()
  }, props.settings.activity_interval_ms)
}

onMounted(restart)
watch([() => props.settings, () => app.locale, reducedMotion], restart, { deep: true })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <div v-if="settings.activity_enabled && templates.length" class="gpt-activity" :data-kind="activeKind" @mouseenter="hovered = true" @mouseleave="hovered = false" @focusin="hovered = true" @focusout="hovered = false">
    <i class="gpt-activity-dot" aria-hidden="true"></i>
    <div class="gpt-activity-slot">
      <Transition name="gpt-activity-fade" mode="out-in">
        <span :key="revision" class="gpt-activity-message" :title="message">{{ message }}</span>
      </Transition>
    </div>
    <button v-if="reducedMotion !== 'reduce'" type="button" class="gpt-activity-control" :aria-label="paused ? c.activityResume : c.activityPause" :aria-pressed="paused" @click="paused = !paused">
      <Play v-if="paused" :size="12" /><Pause v-else :size="12" />
    </button>
  </div>
</template>

<style scoped>
.gpt-activity{display:flex;align-items:center;justify-content:center;gap:8px;max-width:700px;width:100%;height:40px;margin:0 auto;color:#778399;font-size:13px;line-height:1.5}
.gpt-activity-dot{width:7px;height:7px;flex:none;background:#10b981;border-radius:50%;box-shadow:0 0 0 3px #10b9810c}
.gpt-activity-slot{display:flex;align-items:center;justify-content:center;min-width:0;max-width:600px}
.gpt-activity-message{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;overflow-wrap:anywhere;text-align:center}
.gpt-activity[data-kind=order] .gpt-activity-message,.gpt-activity[data-kind=presence] .gpt-activity-message{color:#6552c8}
.gpt-activity-control{display:grid;place-items:center;border:0;background:none;color:#9ca3af;padding:4px;flex:none;border-radius:4px;opacity:.65}
.gpt-activity-control:hover,.gpt-activity-control:focus-visible{color:#6552c8;opacity:1}
.gpt-activity-fade-enter-active,.gpt-activity-fade-leave-active{transition:opacity .22s ease,transform .22s ease}
.gpt-activity-fade-enter-from{opacity:0;transform:translateY(5px)}.gpt-activity-fade-leave-to{opacity:0;transform:translateY(-5px)}
@media(max-width:700px){.gpt-activity{font-size:11px;height:40px;gap:6px}.gpt-activity-slot{max-width:calc(100% - 43px)}}
@media(prefers-reduced-motion:reduce){.gpt-activity-fade-enter-active,.gpt-activity-fade-leave-active{transition:none}}
</style>
