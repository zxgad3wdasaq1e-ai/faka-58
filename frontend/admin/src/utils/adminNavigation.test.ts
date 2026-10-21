import { describe, expect, it } from 'vitest'
import { accessibleAdminSections, activeAdminPage } from './adminNavigation'

describe('admin section navigation', () => {
  it('uses an accessible secondary page as the section entry for restricted roles', () => {
    const sections = accessibleAdminSections(permission => !permission || permission === 'GET:/admin/order-refunds')
    expect(sections.find(section => section.id === 'orders')?.items[0]?.to).toBe('/order-refunds')
    expect(sections.find(section => section.id === 'products')).toBeUndefined()
    expect(sections.find(section => section.id === 'settings')?.items[0]?.to).toBe('/security')
  })

  it('keeps one active page on nested bot and customer pages', () => {
    const sections = accessibleAdminSections(() => true)
    expect(activeAdminPage(sections, '/telegram-bot/broadcasts/12')?.item.to).toBe('/telegram-bot/broadcasts')
    expect(activeAdminPage(sections, '/telegram-bot/settings')?.item.to).toBe('/telegram-bot/settings')
    expect(activeAdminPage(sections, '/settings/notifications')?.item.to).toBe('/settings/notifications')
    expect(activeAdminPage(sections, '/users/42')?.section.id).toBe('customers')
    expect(activeAdminPage(sections, '/card-secret-imports')?.section.id).toBe('inventory')
    expect(activeAdminPage(sections, '/users-unknown')).toBeUndefined()
  })
})
