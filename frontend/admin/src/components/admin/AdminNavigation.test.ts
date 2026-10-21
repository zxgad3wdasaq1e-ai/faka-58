import { afterEach, describe, expect, it } from 'vitest'
import { createApp, h, nextTick, type App } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useAdminAuthStore } from '@/stores/auth'
import i18n from '@/i18n'
import AdminNavigation from './AdminNavigation.vue'
import AdminSectionNav from './AdminSectionNav.vue'

let app: App | undefined
afterEach(() => {
  app?.unmount()
  app = undefined
  document.body.replaceChildren()
  localStorage.clear()
})

describe('admin navigation rendering', () => {
  it('keeps restricted navigation, search and active child links under a custom admin base', async () => {
    const pinia = createPinia()
    const auth = useAdminAuthStore(pinia)
    auth.isSuper = false
    auth.permissions = ['GET:/admin/telegram-bot/broadcasts']
    const router = createRouter({
      history: createMemoryHistory('/control-room'),
      routes: [{ path: '/:pathMatch(.*)*', component: { render: () => null } }],
    })
    await router.push('/telegram-bot/broadcasts/12')
    const root = document.createElement('div')
    document.body.append(root)
    app = createApp({ render: () => h('div', [h(AdminNavigation), h(AdminSectionNav)]) })
    app.use(pinia).use(router).use(i18n).mount(root)
    await nextTick()
    const primary = root.querySelector('nav')!
    expect(primary.querySelector('a[aria-current="location"]')?.getAttribute('href')).toBe('/control-room/telegram-bot/broadcasts')
    expect(primary.querySelector('a[href$="/products"]')).toBeNull()
    const secondary = root.querySelectorAll('nav')[1]!
    expect(secondary.querySelectorAll('a')).toHaveLength(1)
    expect(secondary.querySelector('a[aria-current="page"]')?.getAttribute('href')).toBe('/control-room/telegram-bot/broadcasts')
    expect(root.textContent).not.toContain('admin.sections.')
    const search = root.querySelector('input')!
    search.value = i18n.global.t('admin.navItems.telegramBotBroadcasts')
    search.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    expect(primary.querySelectorAll('a')).toHaveLength(1)
    expect(primary.querySelector('a')?.getAttribute('href')).toBe('/control-room/telegram-bot/broadcasts')
  })
})
