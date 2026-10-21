import { describe, expect, it } from 'vitest'
import { createRequire } from 'node:module'
import { mergeAttributes } from '@tiptap/vue-3'

// Regression for GHSA-cp6q-959q-f8rh. Exercise both package exports used by builds.
const require = createRequire(import.meta.url)
const requireEditor = createRequire(require.resolve('@tiptap/vue-3'))
const cjsMergeAttributes = requireEditor('@tiptap/core').mergeAttributes as typeof mergeAttributes

describe.each([['ESM', mergeAttributes], ['CommonJS', cjsMergeAttributes]] as const)('%s editor attributes', (_, merge) => {
  it('keeps JSON __proto__ input as data through repeated merges and serialization', () => {
    const input = JSON.parse('{"__proto__":{"src":"https://invalid.example/prototype","onerror":"canary"},"class":"photo"}')
    const merged = merge({}, input)
    const repeated = merge(merged, { alt: 'product' })
    expect(Object.getPrototypeOf(repeated)).toBe(Object.prototype)
    expect(repeated.src).toBeUndefined()
    expect(repeated.onerror).toBeUndefined()
    expect(Object.prototype.hasOwnProperty.call(repeated, '__proto__')).toBe(true)
    expect(Object.getOwnPropertyDescriptor(repeated, '__proto__')?.value).toEqual(input.__proto__)
    const image = document.createElement('img')
    for (const key in repeated) image.setAttribute(key, String(repeated[key]))
    expect(image.hasAttribute('src')).toBe(false)
    expect(image.hasAttribute('onerror')).toBe(false)
    expect(merge(JSON.parse(JSON.stringify(repeated))).src).toBeUndefined()
    expect(Object.prototype).not.toHaveProperty('onerror')
  })

  it('preserves class, style and ordinary attribute overrides', () => {
    expect(merge({ class: 'a b', style: 'color: red; width: 10px', title: 'old' }, { class: 'b c', style: 'color: blue', title: 'new' })).toEqual({
      class: 'a b c', style: 'color: blue; width: 10px', title: 'new',
    })
  })
})
