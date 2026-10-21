<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { usePreferredReducedMotion } from '@vueuse/core'

const props = defineProps<{ phrases: string[]; interval: number }>()
const reducedMotion = usePreferredReducedMotion()
const index = ref(0)
const activePhrase = computed(() => props.phrases[index.value] || props.phrases[0] || '')
let timer: ReturnType<typeof setInterval> | undefined

function restart() {
  if (timer) clearInterval(timer)
  timer = undefined
  index.value = 0
  if (reducedMotion.value === 'reduce' || props.phrases.length < 2) return
  timer = setInterval(() => { index.value = (index.value + 1) % props.phrases.length }, props.interval)
}

onMounted(restart)
watch([() => props.phrases, () => props.interval, reducedMotion], restart, { deep: true })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <h2 class="faka-58-rotating-headline">
    <span class="sr-only">{{ phrases.join(' · ') }}</span>
    <span :key="`${index}-${activePhrase}`" class="faka-58-rotating-phrase" aria-hidden="true"><span class="faka-58-rotating-text">{{ activePhrase }}</span></span>
  </h2>
</template>

<style scoped>
.faka-58-rotating-headline{position:relative;display:flex;justify-content:center;align-items:center;height:56px;margin:0 0 16px;font-size:30px;line-height:1.25;font-weight:600;letter-spacing:-.025em}
.faka-58-rotating-phrase{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;min-width:0;animation:faka-58-headline-reveal .5s ease-out both}
.faka-58-rotating-text{display:block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;background:linear-gradient(90deg,#4f46e5,#7c3aed);color:transparent;background-clip:text;-webkit-background-clip:text}
@keyframes faka-58-headline-reveal{from{opacity:0;transform:translateY(20px)}to{opacity:1;transform:translateY(0)}}
@media(min-width:640px){.faka-58-rotating-headline{height:80px;font-size:36px}}
@media(min-width:768px){.faka-58-rotating-headline{height:96px;font-size:48px}}
@media(max-width:639px){.faka-58-rotating-headline:lang(en){font-size:24px}}
@media(prefers-reduced-motion:reduce){.faka-58-rotating-phrase{animation:none}}
</style>
