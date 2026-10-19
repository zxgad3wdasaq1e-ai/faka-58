export type HomepageLocale = 'zh-CN' | 'zh-TW' | 'en-US'
export type HomepageLocalizedText = Record<HomepageLocale, string>

export interface HomepageReview {
  id: string
  name: string
  product: string
  rating: number
  content: HomepageLocalizedText
}

export interface HomepageSettings {
  hero_phrases: HomepageLocalizedText[]
  hero_interval_ms: number
  reviews_enabled: boolean
  reviews_demo: boolean
  reviews_title: HomepageLocalizedText
  reviews_subtitle: HomepageLocalizedText
  reviews_duration_seconds: number
  reviews: HomepageReview[]
}

const locales: HomepageLocale[] = ['zh-CN', 'zh-TW', 'en-US']
const record = (value: unknown): Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}
const text = (value: unknown, limit: number) => typeof value === 'string' ? Array.from(value.trim()).slice(0, limit).join('') : ''
const integer = (value: unknown, min: number, max: number, fallback: number) =>
  typeof value === 'number' && Number.isInteger(value) && value >= min ? Math.min(value, max) : fallback
const localized = (zh: string, tw: string, en: string): HomepageLocalizedText => ({ 'zh-CN': zh, 'zh-TW': tw, 'en-US': en })
const localizedText = (value: unknown, limit: number): HomepageLocalizedText => {
  const source = record(value)
  return Object.fromEntries(locales.map(locale => [locale, text(source[locale], limit)])) as HomepageLocalizedText
}

export const createDefaultHeroPhrases = (): HomepageLocalizedText[] => [
  localized('专业 AI 订阅与数字服务', '專業 AI 訂閱與數位服務', 'AI subscriptions, made simple'),
  localized('快速自动交付', '快速自動交付', 'Fast automatic delivery'),
  localized('安全可靠，订单可查', '安全可靠，訂單可查', 'Secure orders you can track'),
  localized('无需海外信用卡', '無需海外信用卡', 'No overseas credit card needed'),
]

export const normalizeHomepageSettings = (value?: unknown): HomepageSettings => {
  const raw = record(value)
  const phrases = Array.isArray(raw.hero_phrases)
    ? raw.hero_phrases.map(item => localizedText(item, 100)).filter(item => Object.values(item).some(Boolean)).slice(0, 8)
    : []
  const ids = new Set<string>()
  const reviews = Array.isArray(raw.reviews) ? raw.reviews.flatMap((item, index) => {
    const review = record(item)
    const content = localizedText(review.content, 1000)
    const name = text(review.name, 60)
    const product = text(review.product, 80)
    if (!name && !product && !Object.values(content).some(Boolean)) return []
    let id = text(review.id, 64)
    if (!/^[a-zA-Z0-9][a-zA-Z0-9_-]*$/.test(id) || ids.has(id)) id = `review-${index + 1}`
    while (ids.has(id)) id += '-1'
    ids.add(id)
    return [{ id, name, product, rating: integer(review.rating, 1, 5, 5), content }]
  }).slice(0, 24) : []
  return {
    hero_phrases: phrases.length ? phrases : createDefaultHeroPhrases(),
    hero_interval_ms: integer(raw.hero_interval_ms, 1000, 10000, 2600),
    reviews_enabled: typeof raw.reviews_enabled === 'boolean' ? raw.reviews_enabled : true,
    reviews_demo: typeof raw.reviews_demo === 'boolean' ? raw.reviews_demo : true,
    reviews_title: localizedText(raw.reviews_title, 200),
    reviews_subtitle: localizedText(raw.reviews_subtitle, 500),
    reviews_duration_seconds: integer(raw.reviews_duration_seconds, 20, 120, 55),
    reviews,
  }
}

/** Original sample copy. These are demonstration entries, never verified customer feedback. */
export const createDemoReviews = (): HomepageReview[] => [
  { id: 'demo-1', name: '阿远', product: 'ChatGPT Plus', rating: 5, content: localized(
    '之前一直卡在付款这一步，按页面提示走完就好了。订单能自己查，省得反复找客服。',
    '之前一直卡在付款這一步，按頁面提示走完就好了。訂單能自己查，省得反覆找客服。',
    'Payment used to be the tricky part for me. The steps were clear, and being able to check my own order saved some back and forth.',
  ) },
  { id: 'demo-2', name: '小林同学', product: 'ChatGPT Plus', rating: 5, content: localized(
    '买来改论文提纲，先试一个月。下单前把说明看清楚，后面基本没遇到问题。',
    '買來改論文提綱，先試一個月。下單前把說明看清楚，後面基本沒遇到問題。',
    'Trying a month to help with my thesis outline. I read the instructions first and have had no trouble getting started.',
  ) },
  { id: 'demo-3', name: 'Leah', product: 'ChatGPT Pro', rating: 5, content: localized(
    '晚上临时要赶方案，收到后就用上了。之前有点担心，查到订单状态才放下心。',
    '晚上臨時要趕方案，收到後就用上了。之前有點擔心，查到訂單狀態才放下心。',
    'I had a proposal to finish that evening. I was a little unsure at first, but seeing the delivery in my order helped.',
  ) },
  { id: 'demo-4', name: '阿澈', product: 'ChatGPT Plus', rating: 4, content: localized(
    '整体挺顺手的。第一次没找到卡密在哪，回订单页看了一下就找到了，入口能再醒目一点更好。',
    '整體挺順手的。第一次沒找到卡密在哪，回訂單頁看了一下就找到了，入口能再醒目一點更好。',
    'Fairly straightforward overall. I missed the key at first and found it back on the order page. A more obvious link would help.',
  ) },
  { id: 'demo-5', name: '小周', product: 'ChatGPT Pro', rating: 5, content: localized(
    '价格和交付方式都写明白了，这点我比较喜欢。后续要不要续期，打算再用一阵看看。',
    '價格和交付方式都寫明白了，這點我比較喜歡。後續要不要續期，打算再用一陣看看。',
    'I appreciate having the price and delivery method explained up front. Still trying it out before deciding whether to renew.',
  ) },
  { id: 'demo-6', name: '小安', product: 'ChatGPT Plus', rating: 5, content: localized(
    '手机上也能操作，没有让我来回切好几个页面。希望之后一直这么稳。',
    '手機上也能操作，沒有讓我來回切好幾個頁面。希望之後一直這麼穩。',
    'Managed everything on my phone without jumping between a pile of pages. Hoping it stays this straightforward.',
  ) },
  { id: 'demo-7', name: 'July', product: 'ChatGPT Plus', rating: 5, content: localized(
    '从选套餐到拿到交付内容，流程挺清楚。给同事推荐前，我自己先用了几天。',
    '從選方案到拿到交付內容，流程挺清楚。給同事推薦前，我自己先用了幾天。',
    'The path from choosing a plan to finding the delivery was clear. I tried it myself for a few days before mentioning it to a colleague.',
  ) },
  { id: 'demo-8', name: '一颗栗子', product: 'ChatGPT Plus', rating: 5, content: localized(
    '付款之后等了一小会儿，刷新订单就看到了。建议保存好查询密码，我差点给忘了。',
    '付款之後等了一小會兒，重新整理訂單就看到了。建議保存好查詢密碼，我差點給忘了。',
    'Refreshed my order after a short wait and the delivery was there. Save your order password somewhere; I nearly forgot mine.',
  ) },
  { id: 'demo-9', name: '何同学', product: 'ChatGPT Pro', rating: 5, content: localized(
    '主要用来写代码和看英文文档。订阅这边没什么问题，具体好不好用还是看自己的使用场景。',
    '主要用來寫程式和看英文文件。訂閱這邊沒什麼問題，具體好不好用還是看自己的使用情境。',
    'Mostly using it for code and English documentation. The subscription side has been fine; how useful the tool is depends on your work.',
  ) },
  { id: 'demo-10', name: 'Ricky', product: 'ChatGPT Plus', rating: 5, content: localized(
    '不是第一次买这类服务了。这里的信息比较全，先看清规则再下单，心里有底一些。',
    '不是第一次買這類服務了。這裡的資訊比較全，先看清規則再下單，心裡有底一些。',
    'Not my first time buying this sort of service. There was enough detail to read through the terms before placing an order.',
  ) },
  { id: 'demo-11', name: '小鱼', product: 'ChatGPT Plus', rating: 5, content: localized(
    '本来只想先试试，目前够用了。能找到订单和说明，比我之前自己折腾的方式方便。',
    '本來只想先試試，目前夠用了。能找到訂單和說明，比我之前自己折騰的方式方便。',
    'Just wanted to try it at first, and it covers what I need for now. Having the order and instructions in one place is handy.',
  ) },
  { id: 'demo-12', name: '木木', product: 'ChatGPT Pro', rating: 4, content: localized(
    '先给四星，流程是顺的。套餐之间的区别如果再解释细一点，新用户会更好选。',
    '先給四星，流程是順的。方案之間的差別如果再解釋細一點，新使用者會更好選。',
    'Four stars for now. The process was smooth, though a little more explanation of the differences between plans would help new users.',
  ) },
]
