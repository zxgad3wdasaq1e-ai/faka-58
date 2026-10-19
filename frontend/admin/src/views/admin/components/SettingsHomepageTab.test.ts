import { afterEach, describe, expect, it } from 'vitest'
import { createApp, h, nextTick, ref, type App } from 'vue'
import i18n from '@/i18n'
import SettingsHomepageTab from './SettingsHomepageTab.vue'
import { normalizeHomepageSettings, type HomepageSettings } from '../../../../../shared/homepage'

let app: App | undefined

afterEach(() => {
  app?.unmount()
  app = undefined
  document.body.replaceChildren()
})

const mountEditor = (raw?: unknown) => {
  const homepage = ref(normalizeHomepageSettings(raw))
  const language = ref<'zh-CN' | 'zh-TW' | 'en-US'>('zh-CN')
  const container = document.createElement('div')
  document.body.appendChild(container)
  app = createApp({
    setup: () => () => h(SettingsHomepageTab, {
      modelValue: homepage.value,
      'onUpdate:modelValue': (value: HomepageSettings) => { homepage.value = value },
      currentLang: language.value,
    }),
  })
  app.use(i18n).mount(container)
  return { homepage, language, container }
}

const click = async (selector: string) => {
  const button = document.querySelector<HTMLButtonElement>(selector)
  expect(button).not.toBeNull()
  button!.click()
  await nextTick()
}

const input = async (selector: string, value: string) => {
  const field = document.querySelector<HTMLInputElement | HTMLTextAreaElement>(selector)
  expect(field).not.toBeNull()
  field!.value = value
  field!.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
  await nextTick()
}

describe('homepage content editor', () => {
  it('edits one phrase translation without overwriting the other languages and caps new phrases', async () => {
    const { homepage, language } = mountEditor({
      hero_phrases: [{ 'zh-CN': '简体文案', 'zh-TW': '繁體文案', 'en-US': 'English phrase' }],
    })
    language.value = 'zh-TW'
    await nextTick()
    await input('#homepage-phrase-0', '更新後的繁體文案')
    expect(homepage.value.hero_phrases[0]).toEqual({
      'zh-CN': '简体文案', 'zh-TW': '更新後的繁體文案', 'en-US': 'English phrase',
    })
    await input('#homepage-hero-interval', '5400')
    expect(homepage.value.hero_interval_ms).toBe(5400)
    for (let i = 0; i < 10; i += 1) await click('[data-action="add-phrase"]')
    expect(homepage.value.hero_phrases).toHaveLength(8)
    expect(document.querySelector<HTMLButtonElement>('[data-action="add-phrase"]')!.disabled).toBe(true)
  })

  it('explicitly loading simulated reviews replaces the list and enables the demonstration label', async () => {
    const { homepage } = mountEditor({
      reviews_demo: false,
      reviews: [{ id: 'old-review', name: 'Existing', product: 'Plan', rating: 4, content: { 'zh-CN': 'Existing review' } }],
    })
    await click('[data-action="load-demo"]')
    expect(homepage.value.reviews_demo).toBe(true)
    expect(homepage.value.reviews).toHaveLength(12)
    expect(homepage.value.reviews.some((review) => review.id === 'old-review')).toBe(false)
    expect(homepage.value.reviews.every((review) => review.content['zh-CN'] && review.content['zh-TW'] && review.content['en-US'])).toBe(true)
  })

  it('keeps review content when hiding the section and lets each translation be edited as plain text', async () => {
    const { homepage, language, container } = mountEditor({ reviews: [], reviews_enabled: true })
    await click('[data-action="add-review"]')
    const review = homepage.value.reviews[0]!
    await input(`[id="homepage-review-content-${review.id}"]`, '中文评价')
    language.value = 'en-US'
    await nextTick()
    await input(`[id="homepage-review-content-${review.id}"]`, '<strong>Plain text</strong>')
    expect(review.content['zh-CN']).toBe('中文评价')
    expect(review.content['en-US']).toBe('<strong>Plain text</strong>')
    expect(container.querySelector('strong')).toBeNull()
    await click('#homepage-reviews-enabled')
    expect(homepage.value.reviews_enabled).toBe(false)
    expect(homepage.value.reviews).toHaveLength(1)
    await click('button[aria-label="删除评价 1"]')
    expect(homepage.value.reviews).toHaveLength(0)
  })
})
