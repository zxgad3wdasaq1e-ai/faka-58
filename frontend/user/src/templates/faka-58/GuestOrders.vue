<template>
  <CommerceShell>
    <div class="faka-58-order-lookup">
      <header class="lookup-heading"><span class="lookup-icon"><ClipboardList :size="28" aria-hidden="true" /></span><h1>{{ copy('lookupTitle') }}</h1><p>{{ copy('lookupSubtitle') }}</p></header>
      <form class="lookup-form" @submit.prevent="submitSearch">
        <div v-if="hasSavedAuth" class="lookup-saved"><span>{{ t('guestOrders.savedHint', { email: savedAuth.email || '-' }) }}</span><button type="button" @click="clearSearch">{{ t('guestOrders.clearSaved') }}</button></div>
        <div class="lookup-inputs">
          <label for="lookup-email">{{ t('guestOrders.emailPlaceholder') }}<input id="lookup-email" v-model="email" type="email" autocomplete="email" required :placeholder="t('guestOrders.emailPlaceholder')" /></label>
          <label for="lookup-password">{{ copy('queryPassword') }}<input id="lookup-password" v-model="orderPassword" type="password" autocomplete="off" required :placeholder="t('guestOrders.passwordPlaceholder')" /></label>
          <label for="lookup-order">{{ copy('optionalOrder') }}<input id="lookup-order" v-model="orderNo" type="text" autocomplete="off" :placeholder="t('guestOrders.orderNoPlaceholder')" /></label>
        </div>
        <p class="lookup-help"><LockKeyhole :size="14" aria-hidden="true" />{{ copy('lookupHelp') }}</p>
        <p v-if="error" class="lookup-error" role="alert">{{ error }}</p>
        <button type="submit" class="lookup-submit" :disabled="loading"><LoaderCircle v-if="loading" :size="18" class="lookup-spinner" aria-hidden="true" /><Search v-else :size="18" aria-hidden="true" />{{ loading ? t('guestOrders.searching') : t('guestOrders.search') }}</button>
      </form>
      <div class="lookup-account"><RouterLink to="/me/orders">{{ copy('memberOrders') }}<ArrowUpRight :size="14" aria-hidden="true" /></RouterLink></div>
      <section v-if="orders.length" class="lookup-results" aria-live="polite">
        <h2>{{ copy('resultTitle') }}<span>{{ pagination.total }}</span></h2>
        <article v-for="order in orders" :key="order.order_no" class="lookup-order">
          <div class="lookup-order-meta"><small>{{ t('orders.orderNo') }}</small><strong>{{ order.order_no }}</strong><time>{{ formatDate(order.created_at) }}</time></div>
          <div class="lookup-order-amount"><strong>{{ formatMoney(order.total_amount, order.currency) }}</strong><small v-if="hasDiscountAmount(order.discount_amount)">{{ t('orderDetail.couponDiscountLabel') }} {{ formatDiscountMoney(order.discount_amount, order.currency) }}</small><small v-if="hasDiscountAmount(order.promotion_discount_amount)">{{ t('orderDetail.promotionDiscountLabel') }} {{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</small></div>
          <div class="lookup-order-actions"><Badge :variant="statusVariant(order.status)" size="sm">{{ statusLabel(order.status) }}</Badge><RouterLink class="lookup-detail" :to="{ name: 'guest-order-detail', params: { order_no: order.order_no } }">{{ t('guestOrders.viewDetails') }}<ArrowRight :size="14" aria-hidden="true" /></RouterLink><RouterLink v-if="order.status === 'pending_payment'" class="lookup-pay" :to="{ name: 'payment', query: { guest: '1', order_no: order.order_no } }">{{ t('guestOrders.payNow') }}</RouterLink></div>
        </article>
        <PaginationNav :current-page="pagination.page" :total-pages="pagination.total_page" :loading="loading" @change-page="changePage" />
      </section>
      <div v-else-if="!loading" class="lookup-empty"><PackageSearch :size="33" aria-hidden="true" /><p>{{ searched || hasSavedAuth ? emptyMessage : copy('resultsEmpty') }}</p></div>
    </div>
  </CommerceShell>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ArrowUpRight, ClipboardList, LoaderCircle, LockKeyhole, PackageSearch, Search } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import PaginationNav from '../../components/PaginationNav.vue'
import { useGuestOrders } from '../../composables/useGuestOrders'
import { usePageSeo } from '../../composables/usePageSeo'
import CommerceShell from './components/CommerceShell.vue'
import { useCommerceCopy } from './messages'

const { t } = useI18n()
const copy = useCommerceCopy()
const searched = ref(false)
const { savedAuth, email, orderPassword, orderNo, loading, error, orders, pagination, hasSavedAuth, clearSaved, handleSearch, emptyMessage, changePage, statusLabel, statusVariant, formatMoney, formatDiscountMoney, hasDiscountAmount, formatDate } = useGuestOrders()
const submitSearch = () => { searched.value = true; void handleSearch() }
const clearSearch = () => { searched.value = false; clearSaved() }
usePageSeo({ title: () => copy('lookupTitle'), noindex: () => true })
</script>

<style scoped>
.faka-58-order-lookup{max-width:1024px;margin:auto;padding:40px 28px 70px;color:#111827}.lookup-heading{text-align:center;max-width:620px;margin:0 auto 30px}.lookup-icon{display:grid;place-items:center;width:58px;height:58px;margin:0 auto 19px;background:#ede5fb;color:#7c3aed;border-radius:18px}.lookup-heading h1{font-size:30px;font-weight:750;margin:0 0 12px;letter-spacing:-.8px}.lookup-heading p{color:#94a3b8;font-size:13px;line-height:1.9;margin:0}.lookup-form{max-width:780px;background:#fff;border:1px solid #e2e8f0;border-radius:20px;padding:28px;margin:0 auto;box-shadow:0 3px 18px #0f172a02}.lookup-inputs{display:grid;grid-template-columns:1fr 1fr;gap:20px}.lookup-inputs label{display:flex;flex-direction:column;gap:10px;font-size:12px;font-weight:600;color:#475569}.lookup-inputs label:last-child{grid-column:1/-1}.lookup-inputs input{height:44px;border:1px solid #e2e8f0;border-radius:9px;padding:0 13px;min-width:0;width:100%;color:#334155;background:#fff;font-size:13px;font-weight:400;transition:border-color .2s}.lookup-inputs input::placeholder{color:#b0b8c5}.lookup-inputs input:focus{border-color:#8b5cf6;outline:3px solid #ede9fe}.lookup-help{display:flex;align-items:flex-start;gap:7px;color:#94a3b8;font-size:11px;line-height:1.75;margin:17px 0 22px}.lookup-help svg{flex:none;margin-top:2px}.lookup-submit{display:flex;align-items:center;justify-content:center;gap:9px;width:100%;min-height:44px;border:0;background:#7c3aed;color:white;border-radius:9px;font-size:13px;font-weight:650;cursor:pointer;transition:background .2s}.lookup-submit:hover{background:#6d28d9}.lookup-submit:disabled{opacity:.6;cursor:wait}.lookup-error{font-size:12px;color:#dc2626;padding:11px 13px;background:#fff1f2;border:1px solid #ffe4e6;border-radius:8px;margin:0 0 16px}.lookup-saved{display:flex;justify-content:space-between;align-items:center;gap:15px;padding:12px;border-radius:8px;background:#faf9fc;color:#94a3b8;font-size:11px;margin:0 0 22px;line-height:1.7}.lookup-saved button{border:0;background:transparent;color:#7c3aed;cursor:pointer;flex:none;font-size:11px}.lookup-account{text-align:center;margin:20px 0 0}.lookup-account a{display:inline-flex;align-items:center;gap:5px;color:#94a3b8;font-size:11px;text-decoration:none}.lookup-account a:hover{color:#7c3aed}.lookup-empty{display:flex;flex-direction:column;align-items:center;gap:14px;margin:50px 0 0;color:#b0b8c5}.lookup-empty p{font-size:12px}.lookup-results{margin:40px auto 0;max-width:900px}.lookup-results h2{display:flex;align-items:center;gap:10px;font-size:18px;font-weight:650;margin:0 0 18px}.lookup-results h2 span{display:grid;place-items:center;min-width:23px;height:23px;font-size:11px;background:#ede9fe;color:#7c3aed;border-radius:7px}.lookup-order{display:flex;align-items:center;justify-content:space-between;gap:22px;background:white;border:1px solid #e2e8f0;border-radius:15px;padding:21px;margin-bottom:13px}.lookup-order-meta{display:flex;flex-direction:column;gap:6px;min-width:0}.lookup-order-meta small{font-size:10px;color:#94a3b8}.lookup-order-meta strong{font-size:12px;font-weight:600;color:#475569;overflow-wrap:anywhere}.lookup-order-meta time{font-size:10px;color:#94a3b8}.lookup-order-amount{display:flex;flex-direction:column;gap:6px}.lookup-order-amount strong{font-size:16px;font-weight:750;white-space:nowrap}.lookup-order-amount small{font-size:10px;color:#e11d48}.lookup-order-actions{display:flex;align-items:center;gap:12px;flex-wrap:wrap;justify-content:flex-end}.lookup-detail{display:flex;align-items:center;gap:5px;font-size:11px;color:#64748b;text-decoration:none;white-space:nowrap}.lookup-detail:hover{color:#7c3aed}.lookup-pay{padding:7px 11px;background:#7c3aed;color:#fff;border-radius:7px;font-size:11px;text-decoration:none;white-space:nowrap}.lookup-spinner{animation:spin 1s linear infinite}button:focus-visible,a:focus-visible{outline:3px solid #c4b5fd;outline-offset:3px}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:700px){.faka-58-order-lookup{padding:28px 18px 45px}.lookup-heading h1{font-size:26px}.lookup-form{padding:22px}.lookup-inputs{grid-template-columns:1fr;gap:17px}.lookup-order{flex-wrap:wrap;gap:16px}.lookup-order-actions{width:100%;justify-content:flex-start}.lookup-order-amount{margin-left:auto}.lookup-saved{align-items:flex-start}}
</style>
