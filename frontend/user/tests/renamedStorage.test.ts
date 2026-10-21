import test from 'node:test'
import assert from 'node:assert/strict'
import { readRenamedStorageValue } from '../src/utils/renamedStorage.ts'

const createStorage = (initial: Record<string, string>) => {
    const values = new Map(Object.entries(initial))
    return {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => { values.set(key, value) },
        removeItem: (key: string) => { values.delete(key) },
    }
}

test('affiliate attribution and visitor migrate without extending expiry or replacing identity', () => {
    const attribution = JSON.stringify({ code: 'AFF123', expires_at: 1234567890123 })
    const visitor = 'original-visitor-42'
    const storage = createStorage({ dj_affiliate_attribution: attribution, dj_affiliate_visitor_key: visitor })
    for (const [name, expected] of [['attribution', attribution], ['visitor_key', visitor]]) {
        const key = `faka-58_affiliate_${name}`
        const legacyKey = `dj_affiliate_${name}`
        assert.equal(readRenamedStorageValue(storage, key, legacyKey), expected)
        assert.equal(storage.getItem(key), expected)
        assert.equal(storage.getItem(legacyKey), null)
    }
})

test('new affiliate values take precedence over stale legacy values', () => {
    const storage = createStorage({ current: 'new-visitor', legacy: 'old-visitor' })
    assert.equal(readRenamedStorageValue(storage, 'current', 'legacy'), 'new-visitor')
    assert.equal(storage.getItem('current'), 'new-visitor')
    assert.equal(storage.getItem('legacy'), null)
})

test('a failed migration preserves both the usable value and the legacy storage entry', () => {
    const storage = createStorage({ legacy: 'original-visitor' })
    storage.setItem = () => { throw new Error('read-only storage') }
    assert.equal(readRenamedStorageValue(storage, 'current', 'legacy'), 'original-visitor')
    assert.equal(storage.getItem('current'), null)
    assert.equal(storage.getItem('legacy'), 'original-visitor')
})
