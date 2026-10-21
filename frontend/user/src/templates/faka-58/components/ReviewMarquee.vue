<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowRight, BadgeCheck, Headphones, Pause, Play, ShieldCheck, Star, Zap } from 'lucide-vue-next'
import { createDemoReviews, type HomepageSettings } from '../../../../../shared/homepage'
import { useAppStore } from '../../../stores/app'
import { getLocalizedText } from '../../../utils/resellerSiteConfig'
import { useFaka58Copy } from '../siteCopy'

const props = defineProps<{ settings: HomepageSettings }>()
const app = useAppStore()
const c = useFaka58Copy()
const paused = ref(false)
const colors = ['#7655d9', '#0dada4', '#e39916', '#d2558a', '#469bc3', '#7c6ce0']
const entries = computed(() => {
  const configured = props.settings.reviews.filter(item => item.name && getLocalizedText(item.content, app.locale))
  return configured.length ? configured : props.settings.reviews_demo ? createDemoReviews() : []
})
const rows = computed(() => {
  if (!entries.value.length) return []
  const middle = Math.ceil(entries.value.length / 2)
  return [entries.value.slice(0, middle), entries.value.slice(middle)].map(row => {
    const source = row.length ? row : entries.value
    return { sourceLength: source.length, items: Array.from({ length: Math.max(6, source.length) }, (_, i) => source[i % source.length]!) }
  })
})
const duration = (row: number) => ({
  '--review-desktop-duration': `${props.settings.reviews_duration_seconds * (row ? 48 / 55 : 1)}s`,
  '--review-mobile-duration': `${props.settings.reviews_duration_seconds * (row ? 28 : 32) / 55}s`,
})
const title = computed(() => getLocalizedText(props.settings.reviews_title, app.locale) || c.value.reviewsTitle)
const subtitle = computed(() => getLocalizedText(props.settings.reviews_subtitle, app.locale) || c.value.reviewsDesc)
</script>

<template>
  <section v-if="settings.reviews_enabled && entries.length" id="reviews" class="faka-58-reviews" :class="{ 'is-paused': paused }" :aria-label="title">
    <div class="faka-58-reviews-heading faka-58-container">
      <span class="faka-58-review-eyebrow"><BadgeCheck :size="14" />{{ c.reviewsTag }}</span>
      <h2>{{ title }}</h2>
      <p>{{ subtitle }}</p>
      <div v-if="settings.reviews_demo" class="faka-58-review-demo"><span class="faka-58-review-stars" aria-hidden="true"><Star v-for="star in 5" :key="star" :size="16" /></span><span>{{ c.reviewsDemo }}</span></div>
      <RouterLink class="faka-58-primary faka-58-reviews-cta" to="/plus-price">{{ c.reviewsCTA }} <ArrowRight :size="17" /></RouterLink>
      <small>{{ c.reviewsCTAHint }}</small>
    </div>
    <div class="faka-58-reviews-window">
      <div v-for="(row, rowIndex) in rows" :key="rowIndex" class="faka-58-review-row" :style="duration(rowIndex)">
        <div class="faka-58-review-track" :class="{ reverse: rowIndex === 1 }">
          <div v-for="copy in 2" :key="copy" class="faka-58-review-set" :aria-hidden="copy === 2 ? true : undefined">
            <article v-for="(review, i) in row.items" :key="`${review.id}-${i}`" class="faka-58-review-card" :aria-hidden="i >= row.sourceLength ? true : undefined">
              <blockquote>{{ getLocalizedText(review.content, app.locale) }}</blockquote>
              <div class="faka-58-review-stars" :aria-label="`${c.reviewRating} ${review.rating}/5`"><Star v-for="star in 5" :key="star" :size="14" :class="{ empty: star > review.rating }" /></div>
              <footer>
                <span class="faka-58-review-avatar" :style="{ background: colors[(i + rowIndex * 3) % colors.length] }" aria-hidden="true">{{ Array.from(review.name)[0] }}</span>
                <div><b>{{ review.name }}</b><small>{{ settings.reviews_demo ? c.reviewSampleUser : c.reviewUser }}</small></div>
                <span v-if="review.product" class="faka-58-review-product">{{ review.product }}</span>
              </footer>
            </article>
          </div>
        </div>
      </div>
    </div>
    <div class="faka-58-review-footer"><span><ShieldCheck :size="16" />{{ c.advantage3 }}</span><span><Zap :size="16" />{{ c.trust2 }}</span><span><Headphones :size="16" />{{ c.contact }}</span></div>
    <div class="faka-58-review-controls"><button type="button" :aria-pressed="paused" @click="paused = !paused"><Play v-if="paused" :size="13" /><Pause v-else :size="13" />{{ paused ? c.reviewsResume : c.reviewsPause }}</button><small v-if="settings.reviews_demo">{{ c.reviewsDemoNote }}</small></div>
  </section>
</template>

<style scoped>
.faka-58-reviews{padding:80px 0;scroll-margin-top:104px;background:linear-gradient(180deg,#fff,#fafbfc 45%,#fff);overflow:hidden}.faka-58-reviews-heading{text-align:center;margin-bottom:40px}.faka-58-review-eyebrow{display:inline-flex;align-items:center;gap:6px;border-radius:999px;background:#eeebff;color:#6651d2;padding:6px 16px;font-size:12px;font-weight:600;margin-bottom:20px}.faka-58-reviews-heading h2{font-size:36px;line-height:1.25;letter-spacing:-.025em;margin:0;font-weight:700}.faka-58-reviews-heading>p{font-size:14px;color:#6b7280;margin:15px auto 0;max-width:650px}.faka-58-review-demo{display:flex;justify-content:center;align-items:center;gap:12px;color:#6b7280;font-size:14px;margin-top:20px}.faka-58-review-stars{display:flex;align-items:center;gap:2px;color:#f4b52c}.faka-58-review-stars svg{fill:currentColor;flex:none}.faka-58-review-stars .empty{fill:none;color:#ddd6c6}.faka-58-reviews-cta{min-height:48px;border-radius:12px;margin-top:24px;padding:12px 24px;font-weight:600}.faka-58-reviews-heading>small{display:block;margin-top:12px;color:#9ca3af;font-size:13px}.faka-58-reviews-window{position:relative;display:grid;gap:24px;mask-image:linear-gradient(90deg,transparent,#000 96px,#000 calc(100% - 96px),transparent)}.faka-58-review-row{display:flex;overflow:hidden}.faka-58-review-track{display:flex;width:max-content;flex-shrink:0;animation:faka-58-review-left var(--review-desktop-duration,55s) linear infinite;will-change:transform}.faka-58-review-track.reverse{animation-name:faka-58-review-right}.faka-58-review-set{display:flex;gap:20px;padding-right:20px;flex-shrink:0}.faka-58-review-card{width:340px;flex-shrink:0;padding:24px;background:#fff;border:1px solid #f0f1f5;border-radius:16px;box-shadow:0 2px 8px #11182703;transition:border-color .2s,box-shadow .2s}.faka-58-review-card:hover{border-color:#ddd5fb;box-shadow:0 8px 24px #6550ef0c}.faka-58-review-card blockquote{font-size:15px;line-height:1.625;color:#4b5563;min-height:96px;margin:0 0 16px;display:-webkit-box;-webkit-line-clamp:4;-webkit-box-orient:vertical;overflow:hidden}.faka-58-review-card>footer{display:flex;align-items:center;gap:10px;margin-top:12px;padding-top:16px;border-top:1px solid #f3f4f6}.faka-58-review-avatar{width:36px;height:36px;display:grid;place-items:center;flex:none;border-radius:50%;color:#fff;font-size:14px;font-weight:500}.faka-58-review-card footer b{display:block;font-size:14px;font-weight:500;line-height:1.4;color:#374151}.faka-58-review-card footer small{display:block;font-size:12px;line-height:1.5;color:#9ca3af}.faka-58-review-product{margin-left:auto;max-width:114px;font-size:11px;line-height:1.5;padding:3px 7px;border-radius:5px;background:#f9fafb;color:#9ca3af;text-align:center}.faka-58-review-footer{display:flex;justify-content:center;align-items:center;flex-wrap:wrap;gap:24px;max-width:672px;padding:0 16px;margin:56px auto 0;color:#6b7280;font-size:14px}.faka-58-review-footer>span{display:flex;align-items:center;gap:8px}.faka-58-review-footer>span+span{border-left:1px solid #e5e7eb;padding-left:24px}.faka-58-review-footer svg{color:#9ca3af}.faka-58-review-controls{display:flex;align-items:center;justify-content:center;gap:14px;flex-wrap:wrap;margin:20px 20px 0;color:#9ca3af}.faka-58-review-controls button{display:inline-flex;align-items:center;gap:5px;border:0;background:none;color:#8b819f;font-size:12px;padding:4px}.faka-58-review-controls small{font-size:11px}.faka-58-review-row:hover .faka-58-review-track,.faka-58-review-row:focus-within .faka-58-review-track,.is-paused .faka-58-review-track{animation-play-state:paused}
.faka-58-review-card blockquote{overflow-wrap:anywhere}.faka-58-review-card footer>div{min-width:0;flex:1}.faka-58-review-card footer b{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.faka-58-review-product{overflow-wrap:anywhere;flex-shrink:0}
@keyframes faka-58-review-left{from{transform:translateX(0)}to{transform:translateX(-50%)}}@keyframes faka-58-review-right{from{transform:translateX(-50%)}to{transform:translateX(0)}}
@media(max-width:768px){.faka-58-reviews-heading h2{font-size:30px}.faka-58-review-card{width:300px}.faka-58-review-track{animation-duration:var(--review-mobile-duration,32s)}.faka-58-reviews-window{mask-image:linear-gradient(90deg,transparent,#000 32px,#000 calc(100% - 32px),transparent)}.faka-58-review-footer{gap:16px;font-size:12px}.faka-58-review-footer>span+span{padding-left:16px}.faka-58-review-demo{font-size:12px}.faka-58-reviews{padding:64px 0}}
@media(prefers-reduced-motion:reduce){.faka-58-review-track,.faka-58-review-track.reverse{animation:none;transform:none;will-change:auto}.faka-58-review-row{overflow-x:auto;padding:8px 20px}.faka-58-review-set[aria-hidden="true"],.faka-58-review-card[aria-hidden="true"]{display:none}.faka-58-reviews-window{mask-image:none}.faka-58-review-controls button{display:none}}
</style>
