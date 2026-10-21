import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveCartPricingSnapshot } from '../src/utils/cartPricingSnapshot.ts'

test('cart pricing snapshot replaces cached sku price and wholesale tiers', () => {
  const patch = resolveCartPricingSnapshot(
    {
      wholesale_prices: [{ sku_id: 12, min_quantity: 3, unit_price: '11.00' }],
    },
    {
      price_amount: '12.345',
    },
    false,
  )

  assert.equal(patch.priceAmount, '12.35')
  assert.deepEqual(patch.wholesalePrices, [
    { sku_id: 12, min_quantity: 3, unit_price: '11.00' },
  ])
})

test('cart pricing snapshot preserves cached price when current sku price is invalid', () => {
  for (const priceAmount of [undefined, '', 'invalid', '0', '-1']) {
    const patch = resolveCartPricingSnapshot(
      { wholesale_prices: [] },
      { price_amount: priceAmount },
      false,
    )

    assert.equal(Object.hasOwn(patch, 'priceAmount'), false)
    assert.deepEqual(patch.wholesalePrices, [])
  }
})

test('uniform pricing clears stale wholesale tiers while refreshing the base SKU price', () => {
  const cached = { priceAmount: '7.00', wholesalePrices: [{ min_quantity: 3, unit_price: '1.00' }] }
  const product = { wholesale_prices: [{ min_quantity: 3, unit_price: '2.00' }] }
  const updated = { ...cached, ...resolveCartPricingSnapshot(product, { price_amount: '12.00' }) }

  assert.equal(updated.priceAmount, '12.00')
  assert.equal(updated.wholesalePrices, undefined)
  assert.equal(Number(updated.priceAmount) * 3, 36)
  assert.deepEqual(product.wholesale_prices, [{ min_quantity: 3, unit_price: '2.00' }])
})

test('switching back to legacy pricing reloads current wholesale tiers', () => {
  const product = { wholesale_prices: [{ min_quantity: 3, unit_price: '8.00' }] }
  const sku = { price_amount: '12.00' }
  const uniform = resolveCartPricingSnapshot(product, sku, true)
  const legacy = { ...uniform, ...resolveCartPricingSnapshot(product, sku, false) }

  assert.equal(uniform.wholesalePrices, undefined)
  assert.deepEqual(legacy.wholesalePrices, product.wholesale_prices)
  assert.equal(legacy.priceAmount, '12.00')
})

test('cart pricing snapshot clears cached wholesale tiers when the current product has none', () => {
  const patch = resolveCartPricingSnapshot(
    {},
    { price_amount: '10.00' },
  )

  assert.ok(Object.hasOwn(patch, 'wholesalePrices'))
  assert.equal(patch.wholesalePrices, undefined)
})
