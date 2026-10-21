import test from 'node:test'
import assert from 'node:assert/strict'
import { nextTick } from 'vue'

test('theme rename migrates the saved preference without replacing a newer setting', async () => {
  const keys = ['localStorage', 'window', 'document'] as const
  const originals = keys.map(key => Object.getOwnPropertyDescriptor(globalThis, key))
  const values = new Map<string, string>([['dujiao_theme', 'dark']])
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  }
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: storage })
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { matchMedia: () => ({ matches: false }) } })
  Object.defineProperty(globalThis, 'document', { configurable: true, value: { documentElement: { classList: { add() {}, remove() {} } } } })
  try {
    const legacy = await import('../src/utils/theme.ts?legacy')
    assert.equal(legacy.useTheme().theme.value, 'dark')
    assert.equal(values.get('faka-58_theme'), 'dark')
    assert.equal(values.has('dujiao_theme'), false)

    values.set('faka-58_theme', 'light')
    values.set('dujiao_theme', 'dark')
    const current = await import('../src/utils/theme.ts?current')
    assert.equal(current.useTheme().theme.value, 'light')
    assert.equal(values.get('faka-58_theme'), 'light')
    assert.equal(values.has('dujiao_theme'), false)
    current.useTheme().toggleTheme()
    await nextTick()
    assert.equal(values.get('faka-58_theme'), 'dark')
  } finally {
    keys.forEach((key, index) => {
      const original = originals[index]
      if (original) Object.defineProperty(globalThis, key, original)
      else Reflect.deleteProperty(globalThis, key)
    })
  }
})
