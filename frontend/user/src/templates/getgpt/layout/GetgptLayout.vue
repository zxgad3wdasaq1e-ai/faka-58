<script setup lang="ts">
import { computed, onBeforeUnmount, watch, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowRight, ChevronDown, ClipboardList, Menu, X, MessageCircle, CheckCircle2, ShoppingCart } from 'lucide-vue-next'
import { useAppStore } from '../../../stores/app'
import { useCartStore } from '../../../stores/cart'
import { useGetgptCopy } from '../siteCopy'
import { getImageUrl } from '../../../utils/image'
import '../styles/getgpt.css'

const app = useAppStore()
const cart = useCartStore()
const c = useGetgptCopy()
const route = useRoute()
const mobile = ref(false)
const helpOpen = ref(false)
const supportOpen = ref(false)
const brand = computed(() => app.config?.brand?.site_name || 'XIOEN AI')
const logo = computed(() => app.config?.brand?.site_logo ? getImageUrl(app.config.brand.site_logo) : '')
const contact = computed(() => app.config?.contact || {})
const supportLinks = computed(() => [
  { label: 'Telegram', url: String(contact.value.telegram || '').trim() },
  { label: 'WhatsApp', url: String(contact.value.whatsapp || '').trim() },
].filter(item => /^https?:\/\//i.test(item.url)))
const links = computed(() => [
  { label: c.value.plans, to: '/plus-price' }, { label: c.value.pro, to: '/gptpro' },
  { label: c.value.blog, to: '/blog' }, { label: c.value.claude, to: '/claude' },
  { label: c.value.gemini, to: '/gemini-pro' }, { label: c.value.grok, to: '/grok' },
])
const footerGroups = computed(() => [
  { title: c.value.plans, links: [{ label: 'ChatGPT Plus', to: '/purchase' }, { label: 'ChatGPT Pro', to: '/gptpro' }, { label: c.value.plans, to: '/plus-price' }] },
  { title: c.value.models, links: [{ label: 'Claude', to: '/claude' }, { label: 'Gemini', to: '/gemini-pro' }, { label: 'Grok', to: '/grok' }] },
  { title: c.value.tutorials, links: [{ label: c.value.guide, to: '/guide' }, { label: c.value.blog, to: '/blog' }, { label: c.value.about, to: '/about' }] },
  { title: c.value.service, links: [{ label: c.value.faq, to: '/faq' }, { label: c.value.help, to: '/help' }, { label: c.value.orders, to: '/order' }] },
])
watch(() => route.fullPath, () => { mobile.value = false; helpOpen.value = false })
function escape(event: KeyboardEvent) { if (event.key === 'Escape') { supportOpen.value = false; mobile.value = false; helpOpen.value = false } }
window.addEventListener('keydown', escape)
onBeforeUnmount(() => window.removeEventListener('keydown', escape))
</script>

<template>
  <div class="gpt-scope">
    <div class="gpt-announcement"><CheckCircle2 :size="19" /><span>{{ c.announcement }}</span><RouterLink to="/gptpro">{{ c.seePlans }} <ArrowRight :size="14" /></RouterLink></div>
    <header class="gpt-header">
      <div class="gpt-nav-wrap">
        <RouterLink to="/" class="gpt-brand"><img v-if="logo" :src="logo" :alt="brand" /><span v-else class="gpt-brand-icon">{{ brand.charAt(0) }}</span><span><b>{{ brand }}</b><small>{{ c.subtitle }}</small></span></RouterLink>
        <nav class="gpt-desktop-nav" :aria-label="c.nav">
          <template v-for="(item, index) in links" :key="item.to">
            <RouterLink :to="item.to">{{ item.label }}</RouterLink>
            <div v-if="index === 1" class="gpt-dropdown" @focusout="(e: FocusEvent) => { if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) helpOpen = false }">
              <button :aria-expanded="helpOpen" @click="helpOpen = !helpOpen">{{ c.guide }} <ChevronDown :size="13" /></button>
              <div v-if="helpOpen" class="gpt-dropdown-panel"><RouterLink to="/guide">{{ c.guide }}</RouterLink><RouterLink to="/faq">{{ c.faq }}</RouterLink><RouterLink to="/help">{{ c.help }}</RouterLink></div>
            </div>
          </template>
        </nav>
        <RouterLink class="gpt-order-link" to="/order"><ClipboardList :size="17" />{{ c.orders }}</RouterLink>
        <RouterLink class="gpt-cart-link" to="/cart" :aria-label="`${c.cart} (${cart.totalItems})`"><ShoppingCart :size="18" /><span v-if="cart.totalItems" class="gpt-cart-count">{{ cart.totalItems }}</span></RouterLink>
        <button class="gpt-menu-button" :aria-label="mobile ? c.close : c.menu" :aria-expanded="mobile" @click="mobile = !mobile"><X v-if="mobile" /><Menu v-else /></button>
      </div>
      <nav v-if="mobile" class="gpt-mobile-nav" :aria-label="c.nav"><RouterLink v-for="item in links" :key="item.to" :to="item.to">{{ item.label }}</RouterLink><RouterLink to="/guide">{{ c.guide }}</RouterLink><RouterLink to="/faq">{{ c.faq }}</RouterLink><RouterLink to="/order">{{ c.orders }}</RouterLink><RouterLink to="/cart">{{ c.cart }}</RouterLink><RouterLink to="/auth/login">{{ c.login }}</RouterLink></nav>
    </header>
    <main class="gpt-main"><slot /></main>
    <footer class="gpt-footer">
      <div class="gpt-footer-grid">
        <div><RouterLink to="/" class="gpt-brand gpt-footer-brand"><span class="gpt-brand-icon">{{ brand.charAt(0) }}</span><b>{{ brand }}</b></RouterLink><p>{{ c.footerDesc }}</p><button class="gpt-footer-contact" @click="supportOpen = true"><MessageCircle :size="16" />{{ c.contact }}</button></div>
        <div v-for="group in footerGroups" :key="group.title"><h3>{{ group.title }}</h3><RouterLink v-for="item in group.links" :key="item.to" :to="item.to">{{ item.label }}</RouterLink></div>
      </div>
      <div class="gpt-footer-bottom"><p>{{ c.independent }}</p><div><span>© {{ new Date().getFullYear() }} {{ brand }} · <a href="https://github.com/dujiao-next/dujiao-next" target="_blank" rel="noopener noreferrer">Dujiao-Next · GPLv3</a></span><nav><RouterLink to="/privacy">{{ c.privacy }}</RouterLink><RouterLink to="/terms">{{ c.terms }}</RouterLink><select :value="app.locale" :aria-label="c.nav" @change="app.setLocale(($event.target as HTMLSelectElement).value)"><option value="zh-CN">简体中文</option><option value="zh-TW">繁體中文</option><option value="en-US">English</option></select></nav></div></div>
    </footer>
    <button class="gpt-support-fab" @click="supportOpen = true" :aria-label="c.support"><MessageCircle :size="22" /><span>{{ c.support }}</span></button>
    <div v-if="supportOpen" class="gpt-modal-overlay" @click.self="supportOpen = false">
      <section class="gpt-support-modal" role="dialog" aria-modal="true" aria-labelledby="gpt-support-title"><button class="gpt-modal-close" :aria-label="c.close" @click="supportOpen = false"><X :size="20" /></button><div class="gpt-icon-box"><MessageCircle /></div><h2 id="gpt-support-title">{{ c.supportTitle }}</h2><p>{{ c.supportDesc }}</p><a v-for="item in supportLinks" :key="item.label" class="gpt-primary" :href="item.url" target="_blank" rel="noopener noreferrer">{{ item.label }} <ArrowRight :size="17" /></a><p v-if="!supportLinks.length">{{ c.supportEmpty }}</p><RouterLink to="/order" class="gpt-text-link" @click="supportOpen = false">{{ c.orders }} →</RouterLink></section>
    </div>
  </div>
</template>
<style scoped>
.gpt-cart-link{position:relative;display:inline-flex;align-items:center;justify-content:center;flex:none;min-width:32px;min-height:36px;color:#475569}.gpt-cart-count{position:absolute;top:-2px;right:-7px;min-width:17px;padding:1px 4px;border-radius:10px;background:#6250ef;color:white;font-size:10px;text-align:center}
</style>
