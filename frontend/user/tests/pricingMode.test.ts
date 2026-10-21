import test from 'node:test'
import assert from 'node:assert/strict'
import { isUniformPricing } from '../src/utils/pricingMode.ts'

test('uniform pricing requires an explicit false to enable legacy pricing', () => {
  for (const config of [undefined, null, {}, { uniform_pricing: true }, { uniform_pricing: 'false' }]) {
    assert.equal(isUniformPricing(config), true)
  }
  assert.equal(isUniformPricing({ uniform_pricing: false }), false)
})
