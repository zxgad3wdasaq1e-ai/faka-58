import test from 'node:test'
import assert from 'node:assert/strict'
import { DEFAULT_TEMPLATE, normalizeStorefrontTemplate } from '../../shared/storefrontTemplate.ts'

test('existing storefront configurations keep their template after the rename', () => {
  assert.equal(normalizeStorefrontTemplate('getgpt'), 'faka-58')
  assert.equal(normalizeStorefrontTemplate('faka-58'), 'faka-58')
  assert.equal(normalizeStorefrontTemplate('classic'), 'classic')
  assert.equal(normalizeStorefrontTemplate('vault'), 'vault')
  assert.equal(DEFAULT_TEMPLATE, 'faka-58')
})

test('invalid storefront settings cannot become template paths', () => {
  for (const value of [undefined, null, {}, '../getgpt', '', 'unknown']) {
    assert.equal(normalizeStorefrontTemplate(value), null)
  }
  assert.equal(normalizeStorefrontTemplate(' getgpt '), 'faka-58')
})
