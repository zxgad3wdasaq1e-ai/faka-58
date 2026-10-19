<script setup lang="ts">
import { ArrowRight, Sparkles, Check } from 'lucide-vue-next'
import { useGetgptCopy } from './siteCopy'
import { usePageSeo } from '../../composables/usePageSeo'
import Products from './Products.vue'
const c = useGetgptCopy()
usePageSeo({ title: () => 'ChatGPT Pro', description: () => c.value.proDesc })
</script>
<template><div class="gpt-pro-page"><section class="gpt-pro-hero"><span class="gpt-section-tag"><Sparkles :size="14" />{{ c.proBadge }}</span><h1>ChatGPT <span>Pro</span></h1><h2>{{ c.proTitle }}</h2><p>{{ c.proDesc }}</p><a class="gpt-primary" href="#pro-plans">{{ c.seePlans }}<ArrowRight :size="18" /></a><div class="gpt-pro-benefits"><span v-for="text in [c.trust1, c.trust2, c.trust3]" :key="text"><Check :size="15" />{{ text }}</span></div></section><div id="pro-plans"><Products /></div><section class="gpt-pro-info gpt-container"><h2>{{ c.proInfo }}</h2><p>{{ c.proInfoDesc }}</p><RouterLink to="/plus-price">{{ c.proAll }} →</RouterLink></section></div></template>
<style scoped>
.gpt-pro-hero{padding:70px 24px 54px;text-align:center;background:radial-gradient(ellipse at 50% 10%,#f0eaff,#fafbff 60%,#fff)}.gpt-pro-hero>.gpt-section-tag{display:inline-flex;gap:7px;align-items:center}.gpt-pro-hero h1{font-size:80px;font-weight:800;letter-spacing:-4px;margin:12px 0}.gpt-pro-hero h1 span{background:linear-gradient(110deg,#5242ed,#9444ee);background-clip:text;color:transparent}.gpt-pro-hero h2{font-size:34px;margin:8px 0 20px}.gpt-pro-hero>p{max-width:650px;margin:0 auto 28px;color:#718096;font-size:16px;line-height:1.8}.gpt-pro-benefits{display:flex;gap:24px;justify-content:center;margin-top:26px;color:#8490a2;font-size:12px}.gpt-pro-benefits>span{display:flex;gap:5px;align-items:center}.gpt-pro-benefits svg{color:#13b18c}.gpt-pro-info{text-align:center;padding-top:30px;padding-bottom:80px}.gpt-pro-info h2{font-size:26px;margin-bottom:16px}.gpt-pro-info p{max-width:750px;margin:0 auto 22px;font-size:14px;color:#7e899c}.gpt-pro-info a{color:#6f52ee;font-size:14px}#pro-plans{scroll-margin-top:30px}@media(max-width:640px){.gpt-pro-hero{padding:45px 20px}.gpt-pro-hero h1{font-size:58px}.gpt-pro-hero h2{font-size:26px}.gpt-pro-benefits{gap:12px}}
</style>
