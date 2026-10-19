import test from 'node:test'
import assert from 'node:assert/strict'
import { createDemoReviews, normalizeHomepageSettings } from '../../shared/homepage.ts'

test('homepage animation values stay finite, integral and within supported durations', () => {
  const settings = normalizeHomepageSettings({ hero_interval_ms: 0, reviews_duration_seconds: Infinity })
  assert.equal(settings.hero_interval_ms, 2600)
  assert.equal(settings.reviews_duration_seconds, 55)
  assert.equal(normalizeHomepageSettings({ hero_interval_ms: 12000 }).hero_interval_ms, 10000)
  assert.equal(normalizeHomepageSettings({ reviews_duration_seconds: 31.5 }).reviews_duration_seconds, 55)
  assert.equal(normalizeHomepageSettings({ reviews_enabled: false, reviews_demo: false }).reviews_enabled, false)
  assert.equal(normalizeHomepageSettings({ reviews_demo: false }).reviews_demo, false)
})

test('only bounded multilingual display content survives normalization', () => {
  const settings = normalizeHomepageSettings({
    hero_phrases: Array.from({ length: 12 }, () => ({ 'zh-CN': '长'.repeat(110), unknown: 'drop' })),
    reviews_title: { 'zh-CN': '🙂'.repeat(210), 'other': 'unsupported' },
    reviews: Array.from({ length: 30 }, () => ({ id: 'same', name: 'name', content: { 'en-US': 'hello' }, script: 'drop' })),
    scripts: ['drop'],
  })
  assert.equal(settings.hero_phrases.length, 8)
  assert.equal(settings.hero_phrases[0]!['zh-CN'].length, 100)
  assert.deepEqual(Object.keys(settings.reviews_title).sort(), ['en-US', 'zh-CN', 'zh-TW'])
  assert.equal(Array.from(settings.reviews_title['zh-CN']).length, 200)
  assert.equal(settings.reviews.length, 24)
  assert.equal(new Set(settings.reviews.map(review => review.id)).size, 24)
  assert.equal('scripts' in settings, false)
  assert.equal('script' in settings.reviews[0]!, false)
})

test('partially edited review drafts are retained when saving site settings', () => {
  const settings = normalizeHomepageSettings({ reviews: [
    { id: 'draft-name', name: 'New author' },
    { id: 'draft-body', content: { 'zh-CN': '正文还在编辑' } },
    { id: 'blank', name: '', content: {} },
  ] })
  assert.equal(settings.reviews.length, 2)
  assert.equal(settings.reviews[0]!.name, 'New author')
  assert.equal(settings.reviews[1]!.content['zh-CN'], '正文还在编辑')
})

test('demo reviews are fresh independent samples and are not silently stored as real feedback', () => {
  const first = createDemoReviews()
  const second = createDemoReviews()
  first[0]!.content['zh-CN'] = 'edited locally'
  assert.notEqual(second[0]!.content['zh-CN'], first[0]!.content['zh-CN'])
  assert.ok(second.every(review => review.id.startsWith('demo-')))
  assert.equal(normalizeHomepageSettings().reviews.length, 0)
  assert.equal(normalizeHomepageSettings().reviews_demo, true)
})
