<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowRight, CheckCircle2 } from 'lucide-vue-next'
import { useGetgptCopy } from './siteCopy'
import { usePageSeo } from '../../composables/usePageSeo'
import FaqSection from './components/FaqSection.vue'
const c = useGetgptCopy()
const route = useRoute()
const help = computed(() => route.path === '/help')
const steps = computed(() => help.value ? [{ title: c.value.q4, desc: c.value.a4 }, { title: c.value.q2, desc: c.value.a2 }, { title: c.value.q5, desc: c.value.a5 }] : [{ title: c.value.step1, desc: c.value.step1d }, { title: c.value.step2, desc: c.value.step2d }, { title: c.value.step3, desc: c.value.step3d }])
usePageSeo({ title: () => help.value ? c.value.helpTitle : c.value.guideTitle })
</script>
<template><div class="gpt-guide-page"><section class="gpt-section"><div class="gpt-container"><div class="gpt-section-heading"><span class="gpt-section-tag">{{ help ? c.help : c.guide }}</span><h1>{{ help ? c.helpTitle : c.guideTitle }}</h1><p>{{ help ? c.helpDesc : c.guideDesc }}</p></div><div class="gpt-guide-steps"><article v-for="(item, i) in steps" :key="item.title"><span>{{ String(i + 1).padStart(2, '0') }}</span><div><h2>{{ item.title }}</h2><p>{{ item.desc }}</p><CheckCircle2 :size="20" /></div></article></div><div class="gpt-guide-actions"><RouterLink class="gpt-primary" :to="help ? '/order' : '/plus-price'">{{ help ? c.orders : c.buy }}<ArrowRight :size="18" /></RouterLink></div></div></section><FaqSection /></div></template>
<style scoped>
.gpt-guide-page{background:linear-gradient(180deg,#f8f9fe,#fff 600px)}.gpt-section-heading h1{font-size:40px;letter-spacing:-1px;margin:0 0 15px}.gpt-guide-steps{max-width:850px;display:grid;gap:20px;margin:auto}.gpt-guide-steps article{display:flex;gap:26px;border:1px solid #e4e6f2;border-radius:18px;padding:30px;background:white}.gpt-guide-steps article>span{font-size:28px;font-weight:700;color:#9274f6;line-height:1.3}.gpt-guide-steps h2{font-size:21px;margin:0 0 10px}.gpt-guide-steps p{font-size:15px;color:#7a879a;margin:0 0 16px;line-height:1.8}.gpt-guide-steps svg{color:#10b981}.gpt-guide-actions{text-align:center;margin-top:32px}@media(max-width:640px){.gpt-section-heading h1{font-size:28px}.gpt-guide-steps article{padding:24px 20px;gap:18px}.gpt-guide-steps h2{font-size:18px}}
</style>
