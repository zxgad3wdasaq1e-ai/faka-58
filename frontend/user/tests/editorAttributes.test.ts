import test from 'node:test'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { mergeAttributes } from '@tiptap/vue-3'

const require = createRequire(import.meta.url)
const requireEditor = createRequire(require.resolve('@tiptap/vue-3'))
const cjsMergeAttributes = requireEditor('@tiptap/core').mergeAttributes as typeof mergeAttributes

for (const [name, merge] of [['ESM', mergeAttributes], ['CommonJS', cjsMergeAttributes]] as const) {
  test(`${name}: editor merges cannot inherit DOM attributes from JSON __proto__`, () => {
    const input = JSON.parse('{"__proto__":{"src":"https://invalid.example/prototype","onerror":"canary"},"class":"photo"}')
    const merged = merge(merge({}, input), { alt: 'product' })
    assert.equal(Object.getPrototypeOf(merged), Object.prototype)
    assert.equal(merged.src, undefined)
    assert.equal(merged.onerror, undefined)
    assert.equal(Object.hasOwn(merged, '__proto__'), true)
    assert.deepEqual(Object.getOwnPropertyDescriptor(merged, '__proto__')?.value, input.__proto__)
    assert.equal(Object.keys(merged).includes('onerror'), false)
    assert.equal(merge(JSON.parse(JSON.stringify(merged))).src, undefined)
    assert.equal(Object.hasOwn(Object.prototype, 'onerror'), false)
  })

  test(`${name}: editor class, style and regular attribute merging still work`, () => {
    assert.deepEqual(merge({ class: 'a b', style: 'color: red; width: 10px', title: 'old' }, { class: 'b c', style: 'color: blue', title: 'new' }), {
      class: 'a b c', style: 'color: blue; width: 10px', title: 'new',
    })
  })
}
