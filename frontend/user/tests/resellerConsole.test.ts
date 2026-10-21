import test from 'node:test'
import assert from 'node:assert/strict'
import {
  canAccessResellerConsole,
  canRenderResellerConsoleModule,
  getResellerConsoleState,
  resolveResellerConsoleModule,
} from '../src/utils/resellerConsole.ts'

test('reseller console requires the explicit feature flag and a main site', () => {
  for (const config of [undefined, null, {}, { reseller_enabled: false }, { reseller_enabled: 'true' }]) {
    assert.equal(canAccessResellerConsole(config), false)
  }
  assert.equal(canAccessResellerConsole({ reseller_enabled: true }), true)
  assert.equal(canAccessResellerConsole({ reseller_enabled: true, tenant: { mode: ' Reseller ' } }), false)
})

test('reseller console route gate blocks active-only modules until profile is active', () => {
  const pendingState = getResellerConsoleState({
    opened: true,
    can_apply: false,
    profile: { status: 'pending_review' },
    domains: [],
  } as any)
  const activeState = getResellerConsoleState({
    opened: true,
    can_apply: false,
    profile: { status: 'active' },
    domains: [],
  } as any)

  assert.equal(resolveResellerConsoleModule('/reseller/products'), 'products')
  assert.equal(canRenderResellerConsoleModule('/reseller/apply', pendingState), true)
  assert.equal(canRenderResellerConsoleModule('/reseller', pendingState), true)
  assert.equal(canRenderResellerConsoleModule('/reseller/products', pendingState), false)
  assert.equal(canRenderResellerConsoleModule('/reseller/products', activeState), true)
})
