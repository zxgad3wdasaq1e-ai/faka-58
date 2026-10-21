import { afterEach, describe, expect, it, vi } from 'vitest'

describe('admin runtime base', () => {
  afterEach(() => {
    document.querySelectorAll('base').forEach(base => base.remove())
    vi.unstubAllEnvs()
    vi.resetModules()
  })

  it('uses the configured runtime path once when creating native links', async () => {
    const base = document.createElement('base')
    base.setAttribute('href', '/faka-58-mgmt-7x9k2/')
    document.head.appendChild(base)
    const { ADMIN_BASE, adminUrl } = await import('./adminBase')
    expect(ADMIN_BASE).toBe('/faka-58-mgmt-7x9k2')
    expect(adminUrl('/users/1')).toBe('/faka-58-mgmt-7x9k2/users/1')
    expect(adminUrl('users/1')).toBe('/faka-58-mgmt-7x9k2/users/1')
  })

  it('never exposes an unreplaced fullstack placeholder in native links', async () => {
    const base = document.createElement('base')
    base.setAttribute('href', '__FAKA_58_ADMIN_BASE__/')
    document.head.appendChild(base)
    const { ADMIN_BASE, adminUrl } = await import('./adminBase')
    expect(ADMIN_BASE).toBe('')
    expect(adminUrl('/users/1')).toBe('/users/1')
  })

  it('treats a relative fullstack base as the current root', async () => {
    vi.stubEnv('BASE_URL', './')
    const { ADMIN_BASE } = await import('./adminBase')
    expect(ADMIN_BASE).toBe('')
  })
})
