<template>
  <div class="gpt-page gpt-plans">
    <header class="plans-heading">
      <h1 v-if="categoryTitle">{{ categoryTitle }}</h1>
      <h1 v-else>{{ copy('plansTitle') }} <span>{{ copy('plansAccent') }}</span></h1>
      <p>{{ copy('plansSubtitle') }}</p>
    </header>
    <div class="plans-filters">
      <div class="plans-categories" :aria-label="t('nav.products')">
        <button type="button" :class="{ selected: selectedCategory === null }" @click="selectCategory(null)">{{ copy('allProducts') }}</button>
        <button v-for="category in categories" :key="category.id" type="button" :class="{ selected: selectedCategory === category.id }" @click="selectCategory(category.id)">{{ getLocalizedText(category.name) }}</button>
      </div>
      <label class="plans-search"><Search :size="16" aria-hidden="true" /><input v-model="searchQuery" type="search" :placeholder="t('products.searchBoxPlaceholder')" :aria-label="t('products.searchLabel')" /></label>
    </div>
    <div v-if="loading" class="plans-grid" aria-busy="true" :aria-label="t('common.loading')"><div v-for="i in 3" :key="i" class="plan-skeleton"></div></div>
    <div v-else-if="products.length" class="plans-grid"><PlanCard v-for="(product, index) in products" :key="product.id" :product="product" :featured="index === 0" /></div>
    <div v-else class="plans-empty"><PackageOpen :size="38" aria-hidden="true" /><p>{{ searchQuery || selectedCategory ? t('products.emptyFiltered') : t('products.empty') }}</p><button v-if="searchQuery || selectedCategory" type="button" @click="resetFilters">{{ t('products.clearFilters') }}</button></div>
    <PaginationNav :current-page="currentPage" :total-pages="totalPages" :loading="loading" @change-page="changePage" />
    <section class="plans-benefits">
      <div><CreditCard :size="22" aria-hidden="true" /><span><strong>{{ copy('paymentTitle') }}</strong><small>{{ copy('paymentHint') }}</small></span></div>
      <div><ClipboardCheck :size="22" aria-hidden="true" /><span><strong>{{ copy('orderTitle') }}</strong><small>{{ copy('orderHint') }}</small></span></div>
      <div><ShieldCheck :size="22" aria-hidden="true" /><span><strong>{{ copy('deliveryTitle') }}</strong><small>{{ copy('deliveryHint') }}</small></span></div>
    </section>
    <RouterLink class="plans-order-link" to="/guest/orders">{{ copy('lookupTitle') }}<ArrowRight :size="15" aria-hidden="true" /></RouterLink>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ClipboardCheck, CreditCard, PackageOpen, Search, ShieldCheck } from 'lucide-vue-next'
import { useProductList } from '../../composables/useProductList'
import { useLocalized } from '../../composables/useProduct'
import { usePageSeo } from '../../composables/usePageSeo'
import PaginationNav from '../../components/PaginationNav.vue'
import PlanCard from './components/PlanCard.vue'
import { useCommerceCopy } from './messages'

const { t } = useI18n()
const route = useRoute()
const copy = useCommerceCopy()
const { getLocalizedText } = useLocalized()
const { loading, products, categories, selectedCategory, searchQuery, currentPage, totalPages, categoryMap, selectCategory, changePage, clearSearch, initialize, cleanup } = useProductList({ pageSize: 12, homeRouteName: 'products' })
const categoryTitle = computed(() => selectedCategory.value ? getLocalizedText(categoryMap.value.get(selectedCategory.value)?.name) : '')
const resetFilters = () => { clearSearch(); selectCategory(null) }
usePageSeo({ title: () => categoryTitle.value || `${copy('plansTitle')} ${copy('plansAccent')}` })
onMounted(() => { searchQuery.value = typeof route.query.search === 'string' ? route.query.search.slice(0, 100) : route.path === '/gptpro' ? 'Pro' : ''; void initialize() })
watch(() => route.query.search, value => { searchQuery.value = typeof value === 'string' ? value.slice(0, 100) : '' })
onUnmounted(cleanup)
</script>

<style scoped>
.gpt-plans{max-width:1280px;margin:auto;padding:28px 0 70px;color:#111827}.plans-heading{text-align:center;margin-bottom:37px}.plans-heading h1{font-size:36px;font-weight:800;line-height:1.4;letter-spacing:-1px;margin:0 0 15px}.plans-heading h1 span{color:#7c3aed}.plans-heading p{font-size:15px;color:#64748b;margin:0;line-height:1.8}.plans-filters{display:flex;justify-content:space-between;align-items:center;gap:15px;margin:0 0 27px}.plans-categories{display:flex;flex-wrap:wrap;gap:7px}.plans-categories button{border:1px solid transparent;border-radius:8px;padding:7px 12px;background:transparent;color:#64748b;font-size:12px;cursor:pointer;transition:all .2s}.plans-categories button:hover,.plans-categories button.selected{background:#f3edff;color:#7c3aed;border-color:#e8dcff}.plans-search{display:flex;align-items:center;gap:8px;border:1px solid #e2e8f0;background:#fff;border-radius:9px;padding:8px 11px;color:#94a3b8;max-width:215px}.plans-search input{width:100%;min-width:0;border:0;background:transparent;color:#334155;font-size:12px;outline:0}.plans-search:focus-within{outline:2px solid #c4b5fd;outline-offset:1px}.plans-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:24px}.plan-skeleton{height:510px;border-radius:24px;background:linear-gradient(110deg,#fafafa 15%,#f1f0f6 40%,#fafafa 65%);border:2px solid #e2e8f0;animation:pulse 1.5s ease-in-out infinite}.plans-benefits{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:30px;margin:52px 0 0;padding:26px 12px;border-top:1px solid #e2e8f0}.plans-benefits>div{display:flex;justify-content:center;gap:13px;align-items:center}.plans-benefits svg{color:#7c3aed;flex:none}.plans-benefits span{display:flex;flex-direction:column;gap:6px}.plans-benefits strong{font-size:13px;font-weight:650}.plans-benefits small{font-size:11px;line-height:1.5;color:#94a3b8}.plans-empty{display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:300px;border:1px dashed #ddd6fe;border-radius:24px;color:#94a3b8;gap:16px}.plans-empty button{border:0;background:#f3edff;color:#7c3aed;padding:9px 16px;border-radius:9px;cursor:pointer}.plans-order-link{display:flex;align-items:center;justify-content:center;gap:8px;color:#7c3aed;font-size:13px;text-decoration:none;margin:20px 0 0}button:focus-visible,a:focus-visible{outline:3px solid #c4b5fd;outline-offset:3px}@keyframes pulse{50%{opacity:.5}}@media(max-width:1350px){.gpt-plans{padding-left:32px;padding-right:32px}}@media(max-width:960px){.plans-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.plans-benefits{gap:16px}.plans-benefits>div{align-items:flex-start}.plans-heading h1{font-size:31px}}@media(max-width:640px){.gpt-plans{padding:28px 18px 50px}.plans-heading{margin-bottom:25px}.plans-heading h1{font-size:27px}.plans-heading p{font-size:13px}.plans-filters{flex-direction:column;align-items:stretch;margin-bottom:28px}.plans-search{max-width:none}.plans-grid{grid-template-columns:1fr;gap:26px}.plans-benefits{grid-template-columns:1fr;margin-top:30px;padding:24px 8px;gap:23px}.plans-benefits>div{justify-content:flex-start}}
</style>
