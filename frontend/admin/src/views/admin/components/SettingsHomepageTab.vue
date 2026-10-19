<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  createDefaultActivityNames,
  createDefaultActivityMessages,
  createDemoReviews,
  type HomepageLocalizedText,
  type HomepageSettings,
} from '../../../../../shared/homepage'

const { t } = useI18n()
const props = defineProps<{ currentLang: 'zh-CN' | 'zh-TW' | 'en-US' }>()
const homepage = defineModel<HomepageSettings>({ required: true })
const blankText = (): HomepageLocalizedText => ({ 'zh-CN': '', 'zh-TW': '', 'en-US': '' })
const activityNamesText = computed({
  get: () => homepage.value.activity_names.join('\n'),
  set: (value: string) => {
    homepage.value.activity_names = value.split(/\r?\n/).slice(0, 40).map((name) => Array.from(name).slice(0, 30).join(''))
  },
})

const addActivityMessage = () => {
  if (homepage.value.activity_messages.length < 12) {
    homepage.value.activity_messages.push({ kind: 'notice', text: blankText() })
  }
}

const resetActivityContent = () => {
  homepage.value.activity_names = createDefaultActivityNames()
  homepage.value.activity_messages = createDefaultActivityMessages()
}

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
        <h2 class="text-lg font-semibold">{{ t('admin.settings.homepage.activityTitle') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.homepage.activityHint') }}</p>
      </div>
      <div class="space-y-6 p-6">
        <div class="flex items-start justify-between gap-4">
          <div class="space-y-1">
            <Label for="homepage-activity-enabled">{{ t('admin.settings.homepage.activityEnabled') }}</Label>
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.activityEnabledHint') }}</p>
          </div>
          <Switch id="homepage-activity-enabled" v-model="homepage.activity_enabled" />
        </div>
        <div class="rounded-lg border border-border bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
          {{ t('admin.settings.homepage.activityDemoHint') }}
        </div>
        <div class="grid gap-5 md:grid-cols-2">
          <div class="space-y-2">
            <Label for="homepage-activity-interval">{{ t('admin.settings.homepage.activityInterval') }}</Label>
            <Input id="homepage-activity-interval" v-model.number="homepage.activity_interval_ms" type="number" min="2000" max="15000" step="100" />
          </div>
          <div class="space-y-2 md:col-span-2">
            <Label for="homepage-activity-names">{{ t('admin.settings.homepage.activityNames') }}</Label>
            <Textarea id="homepage-activity-names" v-model="activityNamesText" rows="5" :placeholder="t('admin.settings.homepage.activityNamesPlaceholder')" />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.activityNamesHint') }}</p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-3 border-t border-border pt-5">
          <Button variant="outline" size="sm" data-action="add-activity" :disabled="homepage.activity_messages.length >= 12" @click="addActivityMessage">{{ t('admin.settings.homepage.addActivity') }}</Button>
          <Button variant="outline" size="sm" data-action="reset-activity" @click="resetActivityContent">{{ t('admin.settings.homepage.resetActivity') }}</Button>
          <span class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.activityCount', { count: homepage.activity_messages.length }) }}</span>
        </div>
        <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.activityTokens', { name: '{name}', count: '{count}' }) }}</p>
        <p class="text-xs text-muted-foreground">{{ t('admin.settings.homepage.activityResetHint') }}</p>
        <div v-if="!homepage.activity_messages.length" class="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
          {{ t('admin.settings.homepage.activityEmpty') }}
        </div>
        <article v-for="(message, index) in homepage.activity_messages" :key="index" class="space-y-4 rounded-xl border border-border bg-background p-5">
          <div class="flex items-center justify-between gap-4">
            <h3 class="text-sm font-semibold">{{ t('admin.settings.homepage.activityMessage', { number: index + 1 }) }}</h3>
            <Button variant="ghost" size="sm" :aria-label="t('admin.settings.homepage.removeActivity', { number: index + 1 })" @click="homepage.activity_messages.splice(index, 1)">{{ t('admin.settings.homepage.remove') }}</Button>
          </div>
          <div class="grid gap-4 sm:grid-cols-3">
            <div class="space-y-2">
              <Label :for="`homepage-activity-kind-${index}`">{{ t('admin.settings.homepage.activityKind') }}</Label>
              <Select v-model="message.kind">
                <SelectTrigger :id="`homepage-activity-kind-${index}`"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="order">{{ t('admin.settings.homepage.activityKindOrder') }}</SelectItem>
                  <SelectItem value="presence">{{ t('admin.settings.homepage.activityKindPresence') }}</SelectItem>
                  <SelectItem value="notice">{{ t('admin.settings.homepage.activityKindNotice') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="space-y-2 sm:col-span-2">
              <Label :for="`homepage-activity-text-${index}`">{{ t('admin.settings.homepage.activityText') }}</Label>
              <Input :id="`homepage-activity-text-${index}`" v-model="message.text[props.currentLang]" maxlength="200" />
            </div>
          </div>
        </article>
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
