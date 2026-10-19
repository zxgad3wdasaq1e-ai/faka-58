/** Mask code points rather than UTF-16 units so nicknames with emoji stay intact. */
export const maskActivityName = (name: string): string => {
  const characters = Array.from(name.trim())
  if (characters.length < 3) return `${characters[0] || '*'}****`
  if (characters.length < 6) return `${characters[0]}****${characters[characters.length - 1]}`
  return `${characters.slice(0, 2).join('')}****${characters.slice(-3).join('')}`
}

export const formatActivityMessage = (template: string, names: string[], random = Math.random): string => {
  const name = maskActivityName(names[Math.floor(random() * names.length)] || 'AIuser')
  const count = String(3 + Math.floor(random() * 3))
  return template.replace(/\{name\}|\{count\}/g, token => token === '{name}' ? name : count)
}

/** Every template appears once per shuffled cycle, with no repeat at cycle boundaries. */
export const createActivitySequence = (length: number, previous: number, random = Math.random): number[] => {
  const sequence = Array.from({ length }, (_, index) => index)
  for (let index = sequence.length - 1; index > 0; index--) {
    const target = Math.floor(random() * (index + 1))
    ;[sequence[index], sequence[target]] = [sequence[target]!, sequence[index]!]
  }
  if (sequence.length > 1 && sequence[0] === previous) [sequence[0], sequence[1]] = [sequence[1]!, sequence[0]!]
  return sequence
}
