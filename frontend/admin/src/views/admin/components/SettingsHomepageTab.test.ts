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

  it('edits announcement translations independently and preserves newlines in the random names editor', async () => {
    const { homepage, language, container } = mountEditor({
      activity_names: ['阿明', 'Leah'],
      activity_messages: [{ kind: 'notice', text: { 'zh-CN': '服务文案', 'zh-TW': '服務文案', 'en-US': 'Service notice' } }],
    })
    language.value = 'en-US'
    await nextTick()
    await input('#homepage-activity-text-0', 'Order details are available any time.')
    expect(homepage.value.activity_messages[0]!.text).toEqual({
      'zh-CN': '服务文案', 'zh-TW': '服務文案', 'en-US': 'Order details are available any time.',
    })
    await input('#homepage-activity-names', '阿明\nLeah\n')
    expect(homepage.value.activity_names).toEqual(['阿明', 'Leah', ''])
    expect(container.querySelector<HTMLTextAreaElement>('#homepage-activity-names')!.value).toBe('阿明\nLeah\n')
    await input('#homepage-activity-names', Array.from({ length: 45 }, (_, index) => `Name ${index}`).join('\n'))
    expect(homepage.value.activity_names).toHaveLength(40)
    expect(container.textContent).toContain('{name}')
    expect(container.textContent).toContain('{count}')
  })

  it('limits announcement entries and restores default content without changing its visibility or speed', async () => {
    const { homepage } = mountEditor({
      activity_enabled: true,
      activity_interval_ms: 4200,
      activity_names: ['Custom name'],
      activity_messages: [{ kind: 'notice', text: { 'zh-CN': 'Custom message' } }],
    })
    await input('#homepage-activity-interval', '9200')
    expect(homepage.value.activity_interval_ms).toBe(9200)
    await click('#homepage-activity-enabled')
    expect(homepage.value.activity_enabled).toBe(false)
    for (let i = 0; i < 15; i += 1) await click('[data-action="add-activity"]')
    expect(homepage.value.activity_messages).toHaveLength(12)
    expect(document.querySelector<HTMLButtonElement>('[data-action="add-activity"]')!.disabled).toBe(true)
    await click('button[aria-label="删除播报 1"]')
    expect(homepage.value.activity_messages).toHaveLength(11)
    await click('[data-action="reset-activity"]')
    expect(homepage.value.activity_names).toHaveLength(10)
    expect(homepage.value.activity_names).not.toContain('Custom name')
    expect(homepage.value.activity_messages).toHaveLength(5)
    expect(homepage.value.activity_messages.every((message) => message.text['zh-CN'] && message.text['zh-TW'] && message.text['en-US'])).toBe(true)
    expect(homepage.value.activity_enabled).toBe(false)
    expect(homepage.value.activity_interval_ms).toBe(9200)
  })
})
