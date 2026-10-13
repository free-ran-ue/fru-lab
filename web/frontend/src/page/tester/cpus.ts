// CPU lists in Linux form ("0-3,8"), as fru-tester reads and writes them.

// parseCpus returns the CPUs of a list, sorted, or null if it is not one.
export function parseCpus(s: string): number[] | null {
  const out = new Set<number>()
  const text = s.trim()
  if (text === '') return []
  for (const raw of text.split(',')) {
    const part = raw.trim()
    const m = /^(\d+)(?:\s*-\s*(\d+))?$/.exec(part)
    if (!m) return null
    const a = Number(m[1])
    const b = m[2] === undefined ? a : Number(m[2])
    if (b < a || b > 4095) return null
    for (let c = a; c <= b; c++) out.add(c)
  }
  return [...out].sort((x, y) => x - y)
}

// formatCpus writes CPUs as a list, e.g. [0,1,2,3,8] → "0-3,8".
export function formatCpus(cpus: number[]): string {
  const s = [...new Set(cpus)].sort((x, y) => x - y)
  const parts: string[] = []
  for (let i = 0; i < s.length;) {
    let j = i
    while (j + 1 < s.length && s[j + 1] === s[j] + 1) j++
    parts.push(j === i ? `${s[i]}` : `${s[i]}-${s[j]}`)
    i = j + 1
  }
  return parts.join(',')
}
