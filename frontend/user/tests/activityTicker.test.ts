import test from 'node:test'
import assert from 'node:assert/strict'
import { createActivitySequence, formatActivityMessage, maskActivityName } from '../src/templates/faka-58/activityTicker.ts'

test('broadcast nicknames are masked without splitting emoji or exposing short names', () => {
  assert.equal(maskActivityName(' wx_kai109 '), 'wx****109')
  assert.equal(maskActivityName('王小猫'), '王****猫')
  assert.equal(maskActivityName('阿橙'), '阿****')
  assert.equal(maskActivityName('😀小橙子'), '😀****子')
  assert.equal(maskActivityName('13752861042'), '13****042')
})

test('broadcast placeholders use a masked identity and a count strictly between three and five', () => {
  const counts = new Set<string>()
  for (const value of [0, 0.5, 0.999999]) {
    const output = formatActivityMessage('{name} / {name} / {count} / {other}', ['wx_kai109'], () => value)
    assert.ok(output.startsWith('wx****109 / wx****109 / '))
    assert.ok(!output.includes('wx_kai109'))
    assert.ok(output.endsWith(' / {other}'))
    counts.add(output.split(' / ')[2]!)
  }
  assert.deepEqual([...counts], ['3', '4', '5'])
})

test('shuffled broadcast cycles cover every template and avoid a boundary repeat', () => {
  for (const random of [() => 0, () => 0.999999, Math.random]) {
    for (let previous = 0; previous < 5; previous++) {
      const cycle = createActivitySequence(5, previous, random)
      assert.deepEqual([...cycle].sort(), [0, 1, 2, 3, 4])
      assert.notEqual(cycle[0], previous)
    }
  }
  assert.deepEqual(createActivitySequence(1, 0), [0])
  assert.deepEqual(createActivitySequence(0, 0), [])
})
