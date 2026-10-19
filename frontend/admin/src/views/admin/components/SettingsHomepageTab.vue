<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  createDemoReviews,
  type HomepageLocalizedText,
  type HomepageSettings,
} from '../../../../../shared/homepage'

const { t } = useI18n()
const props = defineProps<{ currentLang: 'zh-CN' | 'zh-TW' | 'en-US' }>()
const homepage = defineModel<HomepageSettings>({ required: true })
const blankText = (): HomepageLocalizedText => ({ 'zh-CN': '', 'zh-TW': '', 'en-US': '' })

const addPhrase = () => {
  if (homepage.value.hero_phrases.length < 8) homepage.value.hero_phrases.push(blankText())
}

const addReview = () => {
  if (homepage.value.reviews.length >= 24) return
  homepage.value.reviews.push({
    id: `review-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
    name: '',
    product: '',
    rating: 5,
    content: blankText(),
  })
}

const loadDemoReviews = () => {
  homepage.value.reviews = createDemoReviews()
  homepage.value.reviews_demo = true
}

</script>

<template>
  <div class="space-y-6">
    <div class="rounded-lg border border-border bg-muted/40 px-5 py-4 text-sm text-muted-foreground">
      {{ t('admin.settings.homepage.intro') }}
    </div>

    <section class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">{{ t('admin.settings.homepage.heroTitle') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.homepage.heroHint') }}</p>
      </div>
      <div class="space-y-5 p-6">
        <div class="max-w-sm space-y-2">
          <Label for="homepage-hero-interval">{{ t('admin.settings.homepage.heroInterval') }}</Label>
          <Input id="homepage-hero-interval" v-model.number="homepage.hero_interval_ms" type="number" min="1000" max="10000" step="100" />
        </div>
        <div class="space-y-3">
          <div v-for="(phrase, index) in homepage.hero_phrases" :key="index" class="flex items-start gap-3">
            <div class="min-w-0 flex-1 space-y-2">
              <Label :for="`homepage-phrase-${index}`">{{ t('admin.settings.homepage.phrase', { number: index + 1 }) }}</Label>
              <Input :id="`homepage-phrase-${index}`" v-model="phrase[props.currentLang]" maxlength="100" :placeholder="t('admin.settings.homepage.phrasePlaceholder')" />
            </div>
            <Button variant="outline" size="sm" class="mt-7 shrink-0" :aria-label="t('admin.settings.homepage.removePhrase', { number: index + 1 })" @click="homepage.hero_phrases.splice(index, 1)">
              {{ t('admin.settings.homepage.remove') }}
            </Button>
          </div>
          <Button variant="outline" size="sm" data-action="add-phrase" :disabled="homepage.hero_phrases.length >= 8" @click="addPhrase">
            {{ t('admin.settings.homepage.addPhrase') }}
          </Button>
          <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.phraseLimit') }}</p>
        </div>
      </div>
    </section>

    <section class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">{{ t('admin.settings.homepage.reviewsTitle') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.homepage.reviewsHint') }}</p>
      </div>
      <div class="space-y-6 p-6">
        <div class="flex items-start justify-between gap-4">
          <div class="space-y-1">
            <Label for="homepage-reviews-enabled">{{ t('admin.settings.homepage.reviewsEnabled') }}</Label>
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.reviewsEnabledHint') }}</p>
          </div>
          <Switch id="homepage-reviews-enabled" v-model="homepage.reviews_enabled" />
        </div>
        <div class="flex items-start justify-between gap-4">
          <div class="space-y-1">
            <Label for="homepage-reviews-demo">{{ t('admin.settings.homepage.demoLabel') }}</Label>
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.demoHint') }}</p>
          </div>
          <Switch id="homepage-reviews-demo" v-model="homepage.reviews_demo" />
        </div>
        <div class="grid gap-5 md:grid-cols-2">
          <div class="space-y-2">
            <Label for="homepage-reviews-title">{{ t('admin.settings.homepage.heading') }}</Label>
            <Input id="homepage-reviews-title" v-model="homepage.reviews_title[props.currentLang]" maxlength="200" />
          </div>
          <div class="space-y-2">
            <Label for="homepage-reviews-duration">{{ t('admin.settings.homepage.duration') }}</Label>
            <Input id="homepage-reviews-duration" v-model.number="homepage.reviews_duration_seconds" type="number" min="20" max="120" step="1" />
          </div>
          <div class="space-y-2 md:col-span-2">
            <Label for="homepage-reviews-subtitle">{{ t('admin.settings.homepage.subtitle') }}</Label>
            <Input id="homepage-reviews-subtitle" v-model="homepage.reviews_subtitle[props.currentLang]" maxlength="500" />
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-3 border-t border-border pt-5">
          <Button variant="outline" size="sm" data-action="add-review" :disabled="homepage.reviews.length >= 24" @click="addReview">{{ t('admin.settings.homepage.addReview') }}</Button>
          <Button variant="outline" size="sm" data-action="load-demo" @click="loadDemoReviews">{{ t('admin.settings.homepage.loadDemo') }}</Button>
          <span class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.reviewCount', { count: homepage.reviews.length }) }}</span>
        </div>
        <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.demoReplaceHint') }}</p>
        <div v-if="!homepage.reviews.length" class="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
          {{ t('admin.settings.homepage.emptyReviews') }}
        </div>
        <article v-for="(review, index) in homepage.reviews" :key="review.id" class="space-y-4 rounded-xl border border-border bg-background p-5">
          <div class="flex items-center justify-between gap-4">
            <h3 class="text-sm font-semibold">{{ t('admin.settings.homepage.review', { number: index + 1 }) }}</h3>
            <Button variant="ghost" size="sm" :aria-label="t('admin.settings.homepage.removeReview', { number: index + 1 })" @click="homepage.reviews.splice(index, 1)">{{ t('admin.settings.homepage.remove') }}</Button>
          </div>
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div class="space-y-2">
              <Label :for="`homepage-review-name-${review.id}`">{{ t('admin.settings.homepage.name') }}</Label>
              <Input :id="`homepage-review-name-${review.id}`" v-model="review.name" maxlength="60" />
            </div>
            <div class="space-y-2">
              <Label :for="`homepage-review-product-${review.id}`">{{ t('admin.settings.homepage.product') }}</Label>
              <Input :id="`homepage-review-product-${review.id}`" v-model="review.product" maxlength="80" />
            </div>
            <div class="space-y-2">
              <Label :for="`homepage-review-rating-${review.id}`">{{ t('admin.settings.homepage.rating') }}</Label>
              <Input :id="`homepage-review-rating-${review.id}`" v-model.number="review.rating" type="number" min="1" max="5" step="1" />
            </div>
            <div class="space-y-2 sm:col-span-2 lg:col-span-3">
              <Label :for="`homepage-review-content-${review.id}`">{{ t('admin.settings.homepage.content') }}</Label>
              <Textarea :id="`homepage-review-content-${review.id}`" v-model="review.content[props.currentLang]" maxlength="1000" rows="3" />
            </div>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>
