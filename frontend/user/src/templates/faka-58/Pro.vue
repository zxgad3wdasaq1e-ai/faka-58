<script setup lang="ts">
import { ArrowRight, Sparkles, Check } from 'lucide-vue-next'
import { useFaka58Copy } from './siteCopy'
import { usePageSeo } from '../../composables/usePageSeo'
import Products from './Products.vue'
const c = useFaka58Copy()
usePageSeo({ title: () => 'ChatGPT Pro', description: () => c.value.proDesc })
</script>
<template><div class="faka-58-pro-page"><section class="faka-58-pro-hero"><span class="faka-58-section-tag"><Sparkles :size="14" />{{ c.proBadge }}</span><h1>ChatGPT <span>Pro</span></h1><h2>{{ c.proTitle }}</h2><p>{{ c.proDesc }}</p><a class="faka-58-primary" href="#pro-plans">{{ c.seePlans }}<ArrowRight :size="18" /></a><div class="faka-58-pro-benefits"><span v-for="text in [c.trust1, c.trust2, c.trust3]" :key="text"><Check :size="15" />{{ text }}</span></div></section><div id="pro-plans"><Products /></div><section class="faka-58-pro-info faka-58-container"><h2>{{ c.proInfo }}</h2><p>{{ c.proInfoDesc }}</p><RouterLink to="/plus-price">{{ c.proAll }} →</RouterLink></section></div></template>
<style scoped>
.faka-58-pro-hero{padding:70px 24px 54px;text-align:center;background:radial-gradient(ellipse at 50% 10%,#f0eaff,#fafbff 60%,#fff)}.faka-58-pro-hero>.faka-58-section-tag{display:inline-flex;gap:7px;align-items:center}.faka-58-pro-hero h1{font-size:80px;font-weight:800;letter-spacing:-4px;margin:12px 0}.faka-58-pro-hero h1 span{background:linear-gradient(110deg,#5242ed,#9444ee);background-clip:text;color:transparent}.faka-58-pro-hero h2{font-size:34px;margin:8px 0 20px}.faka-58-pro-hero>p{max-width:650px;margin:0 auto 28px;color:#718096;font-size:16px;line-height:1.8}.faka-58-pro-benefits{display:flex;gap:24px;justify-content:center;margin-top:26px;color:#8490a2;font-size:12px}.faka-58-pro-benefits>span{display:flex;gap:5px;align-items:center}.faka-58-pro-benefits svg{color:#13b18c}.faka-58-pro-info{text-align:center;padding-top:30px;padding-bottom:80px}.faka-58-pro-info h2{font-size:26px;margin-bottom:16px}.faka-58-pro-info p{max-width:750px;margin:0 auto 22px;font-size:14px;color:#7e899c}.faka-58-pro-info a{color:#6f52ee;font-size:14px}#pro-plans{scroll-margin-top:30px}@media(max-width:640px){.faka-58-pro-hero{padding:45px 20px}.faka-58-pro-hero h1{font-size:58px}.faka-58-pro-hero h2{font-size:26px}.faka-58-pro-benefits{gap:12px}}
</style>
