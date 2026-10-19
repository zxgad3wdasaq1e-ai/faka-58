<template>
  <article class="plan-card" :class="{ 'plan-card-featured': featured, 'plan-card-sold': soldOut }">
    <span v-if="featured" class="plan-recommend">{{ copy('recommended') }}</span>
    <div class="plan-topline">
      <span class="plan-icon"><Sparkles :size="24" aria-hidden="true" /></span>
      <span class="plan-radio" aria-hidden="true"><span v-if="featured"></span></span>
    </div>
    <h2>{{ title }}</h2>
    <p class="plan-description">{{ getLocalizedText(product.description) || copy('service') }}</p>
    <div class="plan-price">
      <span class="plan-currency">{{ currencySymbol }}</span><strong>{{ priceDisplay }}</strong>
      <span v-if="promotion" class="plan-original">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
    </div>
    <div class="plan-signals">
      <div><span>{{ copy('delivery') }}</span><strong>{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</strong></div>
      <div><span>{{ copy('availability') }}</span><strong>{{ getStockStatusLabel(product) }}</strong></div>
      <div><span>{{ copy('purchase') }}</span><strong>{{ getPurchaseTypeLabel(product.purchase_type) }}</strong></div>
    </div>
    <ul class="plan-features">
      <li v-for="(feature, i) in features" :key="i"><Check :size="17" aria-hidden="true" /><span>{{ feature }}</span></li>
    </ul>
    <RouterLink :to="`/products/${product.slug}`" class="plan-action">
      {{ soldOut ? copy('planDetails') : copy('choosePlan') }}<ArrowRight :size="17" aria-hidden="true" />
    </RouterLink>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ArrowRight, Check, Sparkles } from 'lucide-vue-next'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'
import { amountToCents, centsToAmount } from '../../../utils/money'
import { useCommerceCopy } from '../messages'

const props = defineProps<{ product: any; featured?: boolean }>()
const copy = useCommerceCopy()
const { getLocalizedText, formatPrice, siteCurrency } = useLocalized()
const { isSoldOut, hasPromotionPrice, getPromotionPriceAmount, getStockStatusLabel, getFulfillmentTypeLabel, getPurchaseTypeLabel } = useProductLabels()
const title = computed(() => getLocalizedText(props.product.title))
const soldOut = computed(() => isSoldOut(props.product))
const promotion = computed(() => hasPromotionPrice(props.product))
const currencySymbol = computed(() => ({ CNY: '¥', USD: '$', EUR: '€', GBP: '£' }[siteCurrency.value] || siteCurrency.value))
const priceDisplay = computed(() => {
  const amount = promotion.value ? getPromotionPriceAmount(props.product) : props.product.price_amount
  const cents = amountToCents(amount)
  return cents === null ? String(amount || '0') : centsToAmount(cents).replace(/\.00$/, '')
})
const features = computed<string[]>(() => {
  const tags = Array.isArray(props.product.tags) ? props.product.tags.filter((tag: unknown) => typeof tag === 'string' && tag) : []
  return tags.length ? tags.slice(0, 4) : [getFulfillmentTypeLabel(props.product.fulfillment_type), getPurchaseTypeLabel(props.product.purchase_type), copy('deliveryHint')]
})
</script>

<style scoped>
.plan-card{position:relative;display:flex;flex-direction:column;min-width:0;min-height:510px;padding:26px 24px 24px;background:#fff;border:2px solid #e2e8f0;border-radius:24px;transition:transform .2s,border-color .2s,box-shadow .2s;color:#111827}
.plan-card:hover{transform:translateY(-4px);border-color:#a78bfa;box-shadow:0 16px 40px #4338ca0a}.plan-card-featured{border-color:#8b5cf6;background:linear-gradient(145deg,#faf7ff,#fff 65%)}.plan-card-sold{opacity:.76}.plan-recommend{position:absolute;top:-14px;left:24px;border-radius:7px;background:#7c3aed;color:white;padding:4px 12px;font-size:12px;font-weight:650}.plan-topline{display:flex;align-items:center;justify-content:space-between;margin:0 0 15px}.plan-icon{display:grid;place-items:center;width:43px;height:43px;border-radius:12px;background:#f4f0ff;color:#7c3aed}.plan-radio{display:grid;place-items:center;width:20px;height:20px;border:2px solid #d1d5db;border-radius:50%}.plan-card-featured .plan-radio{border-color:#8b5cf6}.plan-radio span{width:10px;height:10px;background:#8b5cf6;border-radius:50%}.plan-card h2{margin:0;font-size:23px;font-weight:750;line-height:1.4;letter-spacing:-.5px}.plan-description{margin:10px 0 0;min-height:40px;font-size:13px;color:#64748b;line-height:1.65;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}.plan-price{display:flex;align-items:baseline;flex-wrap:wrap;gap:3px;margin:20px 0;color:#111827}.plan-price strong{font-size:48px;font-weight:800;letter-spacing:-2px;line-height:1.1}.plan-currency{font-size:25px;font-weight:650}.plan-original{margin-left:8px;font-size:12px;color:#94a3b8;text-decoration:line-through}.plan-signals{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:5px;padding:15px 0;border-top:1px solid #ececf0;border-bottom:1px solid #ececf0}.plan-signals div{display:flex;flex-direction:column;gap:7px;min-width:0;text-align:center}.plan-signals div+div{border-left:1px solid #ececf0}.plan-signals span{color:#94a3b8;font-size:11px}.plan-signals strong{font-size:12px;font-weight:650;overflow-wrap:anywhere}.plan-features{display:grid;gap:11px;margin:23px 0;padding:0;list-style:none;font-size:13px;color:#475569;line-height:1.6}.plan-features li{display:flex;align-items:flex-start;gap:9px}.plan-features svg{flex:none;margin-top:2px;color:#8b5cf6}.plan-action{margin-top:auto;display:flex;align-items:center;justify-content:center;gap:9px;min-height:45px;padding:10px 12px;background:#111827;color:#fff;border-radius:10px;text-decoration:none;font-size:14px;font-weight:650;transition:background .2s}.plan-action:hover{background:#334155}.plan-card-featured .plan-action{background:#7c3aed}.plan-card-featured .plan-action:hover{background:#6d28d9}.plan-action:focus-visible{outline:3px solid #c4b5fd;outline-offset:3px}@media(max-width:640px){.plan-card{min-height:480px}.plan-price strong{font-size:43px}}
</style>
