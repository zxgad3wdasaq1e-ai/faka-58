<template>
  <div class="gpt-page gpt-product">
    <RouterLink to="/products" class="product-back"><ArrowLeft :size="16" aria-hidden="true" />{{ copy('backPlans') }}</RouterLink>
    <div v-if="loading" class="product-loading" aria-busy="true">{{ t('common.loading') }}</div>
    <template v-else-if="product">
      <header class="product-heading"><h1>{{ copy('detailTitle') }}</h1><p>{{ copy('detailHint') }}</p></header>
      <div class="product-columns">
        <section class="product-overview">
          <div class="overview-top"><span class="product-icon"><Sparkles :size="28" aria-hidden="true" /></span><span v-if="categoryName" class="product-category">{{ categoryName }}</span></div>
          <h2>{{ getLocalizedText(product.title) }}</h2>
          <p class="product-summary">{{ getLocalizedText(product.description) }}</p>
          <div class="product-badges"><span>{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span><span>{{ getPurchaseTypeLabel(product.purchase_type) }}</span><span :class="{ 'badge-out': !canPurchase }">{{ getStockStatusLabel(product) }}</span></div>
          <img v-if="currentImage" class="product-cover" :src="currentImage" :alt="getLocalizedText(product.title)" />
          <div v-if="images.length > 1" class="product-thumbnails"><button v-for="(image, index) in images" :key="image" type="button" :class="{ active: image === currentImage }" :aria-label="`${t('productDetail.details')} ${index + 1}`" @click="currentImage = image"><img :src="image" alt="" /></button></div>
          <dl class="product-facts"><div><dt>{{ copy('delivery') }}</dt><dd>{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</dd></div><div><dt>{{ copy('purchase') }}</dt><dd>{{ getPurchaseTypeLabel(product.purchase_type) }}</dd></div></dl>
          <p class="product-information"><ShieldCheck :size="17" aria-hidden="true" />{{ copy('checkoutHint') }}</p>
        </section>
        <section class="product-purchase">
          <h2>{{ t('productDetail.skuTitle') }}</h2>
          <div class="product-skus"><button v-for="sku in activeSkus" :key="sku.id" type="button" :disabled="!isSkuPurchasable(sku)" :class="{ active: normalizeSkuId(sku.id) === selectedSkuId }" :aria-pressed="normalizeSkuId(sku.id) === selectedSkuId" @click="selectedSkuId = normalizeSkuId(sku.id)"><span>{{ skuDisplayText(sku) }}</span><small>{{ skuStockText(sku) }}</small><CircleCheck v-if="normalizeSkuId(sku.id) === selectedSkuId" :size="18" aria-hidden="true" /></button></div>
          <div class="product-price"><span>{{ t('products.price') }}</span><strong>{{ formatPrice(effectivePrice, siteCurrency) }}</strong><del v-if="hasEffectiveDiscount">{{ formatPrice(basePrice, siteCurrency) }}</del><small v-if="showSelectedSkuMemberBadge">{{ t('products.memberPriceTag') }}</small></div>
          <div v-if="selectedSkuWholesaleRules.length" class="product-rules"><strong>{{ t('products.wholesaleRulesTitle') }}</strong><p v-for="(tier, index) in selectedSkuWholesaleRules" :key="index">{{ formatWholesaleTier(tier) }}</p></div>
          <div v-if="hasPromotionRules(product)" class="product-rules"><strong>{{ t('products.promotionRulesTitle') }}</strong><p v-for="rule in getPromotionRules(product)" :key="rule.id">{{ formatPromotionRule(rule) }}</p></div>
          <div class="quantity-row"><label for="getgpt-quantity">{{ copy('quantity') }}</label><div class="quantity-control"><button type="button" :disabled="quantity <= quantityEffectiveMin" :aria-label="copy('decrease')" @click="quantity = Math.max(quantityEffectiveMin, quantity - 1)"><Minus :size="16" /></button><input id="getgpt-quantity" :value="quantity" type="number" inputmode="numeric" :min="quantityEffectiveMin" :max="quantityEffectiveLimit ?? undefined" @input="handleQuantityInput" /><button type="button" :disabled="quantityEffectiveLimit !== null && quantity >= quantityEffectiveLimit" :aria-label="copy('increase')" @click="quantity = quantityEffectiveLimit === null ? quantity + 1 : Math.min(quantityEffectiveLimit, quantity + 1)"><Plus :size="16" /></button></div></div>
          <p v-if="purchaseWarning || cannotPurchaseReason" class="product-warning" role="alert">{{ purchaseWarning || cannotPurchaseReason }}</p>
          <button v-if="requiresLogin" type="button" class="product-buy" @click="goLogin">{{ t('productDetail.loginToBuy') }}<ArrowRight :size="17" /></button>
          <template v-else><button type="button" class="product-buy" :disabled="!canPurchase" @click="buyNow">{{ t('productDetail.buyNow') }}<ArrowRight :size="17" /></button><button type="button" class="product-add" :disabled="!canPurchase" @click="addToCart"><ShoppingCart :size="17" />{{ t('productDetail.addToCart') }}</button></template>
          <p class="product-order-hint"><LockKeyhole :size="14" aria-hidden="true" />{{ copy('deliveryHint') }}</p>
        </section>
      </div>
      <section v-if="detailHtml || getLocalizedText(product.description)" class="product-content"><h2>{{ copy('description') }}</h2><div class="prose max-w-none prose-a:text-violet-600 prose-img:rounded-xl"><p>{{ getLocalizedText(product.description) }}</p><div v-if="detailHtml" v-html="detailHtml"></div></div></section>
      <section v-if="relatedPosts.length" class="product-related"><h2>{{ t('productDetail.relatedPosts') }}</h2><RouterLink v-for="post in relatedPosts" :key="post.id" :to="`/blog/${post.slug}`">{{ getLocalizedText(post.title) }}<ArrowUpRight :size="16" /></RouterLink></section>
    </template>
    <div v-else class="product-empty"><PackageOpen :size="42" aria-hidden="true" /><h1>{{ t('products.empty') }}</h1><RouterLink to="/products">{{ copy('backPlans') }}</RouterLink></div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { ArrowLeft, ArrowRight, ArrowUpRight, CircleCheck, LockKeyhole, Minus, PackageOpen, Plus, ShieldCheck, ShoppingCart, Sparkles } from 'lucide-vue-next'
import { useProductDetail } from '../../composables/useProductDetail'
import { processHtmlForDisplay } from '../../utils/content'
import { amountToCents } from '../../utils/money'
import { useCommerceCopy } from './messages'

const { t } = useI18n()
const copy = useCommerceCopy()
const {
  loading, product, relatedPosts, currentImage, images, categoryName, getLocalizedText, siteCurrency, formatPrice,
  getFulfillmentTypeLabel, getPurchaseTypeLabel, getStockStatusLabel,
  activeSkus, selectedSku, selectedSkuId, normalizeSkuId, isSkuPurchasable, skuDisplayText, skuStockText,
  quantity, quantityEffectiveMin, quantityEffectiveLimit, handleQuantityInput,
  requiresLogin, canPurchase, cannotPurchaseReason, purchaseWarning, buyNow, addToCart, goLogin,
  hasPromotionPrice, getPromotionPriceAmount, hasSkuPromotionPrice,
  hasSelectedSkuWholesalePrice, selectedSkuWholesaleFinalPrice, selectedSkuPromotionFinalPrice,
  hasMemberPrice, selectedSkuMemberPrice, showSelectedSkuMemberBadge,
  selectedSkuWholesaleRules, formatWholesaleTier, hasPromotionRules, getPromotionRules, formatPromotionRule,
} = useProductDetail()

const basePrice = computed(() => selectedSku.value?.price_amount ?? product.value?.price_amount ?? '0.00')
const effectivePrice = computed(() => {
  if (selectedSku.value) {
    if (hasSelectedSkuWholesalePrice.value) return selectedSkuWholesaleFinalPrice.value
    if (hasSkuPromotionPrice(selectedSku.value)) return selectedSkuPromotionFinalPrice.value
    if (hasMemberPrice.value) return selectedSkuMemberPrice.value
    return selectedSku.value.price_amount
  }
  return product.value && hasPromotionPrice(product.value) ? getPromotionPriceAmount(product.value) : basePrice.value
})
const hasEffectiveDiscount = computed(() => {
  const original = amountToCents(basePrice.value)
  const effective = amountToCents(effectivePrice.value)
  return original !== null && effective !== null && effective < original
})
const detailHtml = computed(() => DOMPurify.sanitize(processHtmlForDisplay(getLocalizedText(product.value?.content))))
</script>

<style scoped>
.gpt-product{max-width:1080px;margin:auto;padding:30px 28px 70px;color:#111827}.product-back{display:inline-flex;align-items:center;gap:7px;color:#64748b;font-size:12px;text-decoration:none}.product-back:hover{color:#7c3aed}.product-heading{text-align:center;padding:25px 0 36px}.product-heading h1{font-size:32px;font-weight:800;letter-spacing:-.7px;margin:0 0 12px}.product-heading p{font-size:14px;color:#64748b;margin:0;line-height:1.7}.product-columns{display:grid;grid-template-columns:1.05fr 1fr;gap:26px;align-items:start}.product-overview,.product-purchase{padding:28px;border:1px solid #e2e8f0;background:#fff;border-radius:22px}.product-overview{background:linear-gradient(155deg,#faf7ff,#fff 65%)}.overview-top{display:flex;align-items:center;gap:14px}.product-icon{display:grid;place-items:center;width:54px;height:54px;background:#efe7ff;border-radius:16px;color:#7c3aed}.product-category{font-size:12px;color:#7c3aed;background:#f3edff;border:1px solid #e9ddff;border-radius:20px;padding:5px 12px}.product-overview h2{font-size:27px;font-weight:750;line-height:1.45;letter-spacing:-.5px;margin:22px 0 12px}.product-summary{font-size:14px;line-height:1.85;color:#64748b;white-space:pre-line}.product-badges{display:flex;flex-wrap:wrap;gap:8px;margin:21px 0}.product-badges span{font-size:11px;border:1px solid #e2e8f0;background:#fff;border-radius:7px;padding:5px 9px;color:#475569}.product-badges .badge-out{color:#b45309;background:#fffbeb;border-color:#fde68a}.product-cover{width:100%;max-height:310px;object-fit:contain;border-radius:14px;margin:5px 0 15px}.product-thumbnails{display:flex;gap:10px;flex-wrap:wrap;margin-bottom:15px}.product-thumbnails button{width:60px;height:48px;padding:2px;border:1px solid #e2e8f0;border-radius:8px;overflow:hidden;cursor:pointer}.product-thumbnails button.active{border:2px solid #8b5cf6}.product-thumbnails img{width:100%;height:100%;object-fit:cover}.product-facts{padding-top:10px;border-top:1px solid #ede9f3}.product-facts>div{display:flex;justify-content:space-between;gap:15px;margin-top:16px;font-size:13px}.product-facts dt{color:#94a3b8}.product-facts dd{font-weight:550;margin:0}.product-information{display:flex;align-items:flex-start;gap:8px;margin:25px 0 0;font-size:11px;color:#94a3b8;line-height:1.7}.product-information svg{flex:none;color:#8b5cf6;margin-top:1px}.product-purchase h2{font-size:15px;font-weight:650;margin:0 0 15px}.product-skus{display:grid;gap:10px}.product-skus button{position:relative;display:flex;flex-direction:column;align-items:flex-start;gap:6px;text-align:left;border:1px solid #e2e8f0;background:#fff;padding:14px 40px 14px 15px;border-radius:10px;cursor:pointer;transition:all .2s}.product-skus button.active{border-color:#8b5cf6;background:#faf7ff}.product-skus span{font-size:13px;color:#334155;font-weight:600}.product-skus small{font-size:11px;color:#94a3b8}.product-skus svg{position:absolute;right:14px;top:19px;color:#7c3aed}.product-skus button:disabled{opacity:.45;cursor:not-allowed}.product-price{display:flex;flex-wrap:wrap;align-items:baseline;gap:8px;margin:25px 0 20px;padding-bottom:23px;border-bottom:1px solid #e2e8f0}.product-price>span{width:100%;color:#94a3b8;font-size:12px}.product-price strong{font-size:34px;color:#7c3aed;letter-spacing:-1px;font-weight:800}.product-price del{color:#94a3b8;font-size:12px}.product-price small{color:#a16207;font-size:11px;background:#fffbeb;padding:3px 8px;border-radius:5px}.product-rules{padding:12px;background:#faf7ff;border-radius:9px;color:#7c3aed;margin-bottom:16px;font-size:11px}.product-rules strong{font-size:12px}.product-rules p{margin:6px 0 0}.quantity-row{display:flex;align-items:center;justify-content:space-between;margin:21px 0}.quantity-row label{font-size:13px;color:#475569}.quantity-control{display:flex;align-items:center;border:1px solid #e2e8f0;border-radius:9px;overflow:hidden;height:37px}.quantity-control button{display:grid;place-items:center;width:34px;height:100%;border:0;background:#fafafa;cursor:pointer;color:#64748b}.quantity-control input{width:48px;min-width:0;height:100%;text-align:center;border:0;font-size:13px;appearance:textfield;background:white}.quantity-control input::-webkit-inner-spin-button{appearance:none}.quantity-control button:disabled{opacity:.4;cursor:not-allowed}.product-buy,.product-add{display:flex;justify-content:center;align-items:center;gap:9px;width:100%;min-height:46px;border-radius:10px;font-size:14px;font-weight:650;cursor:pointer;transition:background .2s}.product-buy{border:0;background:#7c3aed;color:#fff}.product-buy:hover{background:#6d28d9}.product-add{margin-top:10px;color:#64748b;border:1px solid #e2e8f0;background:white}.product-add:hover{background:#faf7ff;color:#7c3aed}.product-buy:disabled,.product-add:disabled{opacity:.45;cursor:not-allowed}.product-order-hint{display:flex;justify-content:center;align-items:center;gap:6px;font-size:10px;color:#94a3b8;margin:16px 0 0;line-height:1.6}.product-warning{color:#b45309;font-size:12px;line-height:1.7;margin-bottom:16px}.product-content{margin-top:36px;padding:28px;border:1px solid #e2e8f0;border-radius:22px;background:#fff}.product-content h2,.product-related h2{font-size:19px;font-weight:700;margin:0 0 20px}.product-content .prose{font-size:14px;color:#64748b;line-height:1.9}.product-related{margin-top:35px}.product-related a{display:flex;align-items:center;justify-content:space-between;padding:16px;border:1px solid #e2e8f0;margin-bottom:10px;border-radius:10px;font-size:14px;color:#475569;background:#fff}.product-loading{min-height:450px;display:grid;place-items:center;color:#94a3b8}.product-empty{display:flex;align-items:center;flex-direction:column;gap:18px;padding:90px 20px;color:#94a3b8}.product-empty h1{font-size:20px}.product-empty a{color:#7c3aed}button:focus-visible,a:focus-visible,input:focus-visible{outline:3px solid #c4b5fd;outline-offset:3px}@media(max-width:720px){.gpt-product{padding:22px 18px 45px}.product-heading{padding:25px 0}.product-heading h1{font-size:27px}.product-heading p{font-size:12px}.product-columns{grid-template-columns:1fr;gap:18px}.product-overview,.product-purchase,.product-content{padding:22px}.product-cover{max-height:230px}}
</style>
