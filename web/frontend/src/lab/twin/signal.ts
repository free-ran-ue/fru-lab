// The map's simulated radio signal of a gNB at distance d (metres), from
// its radio range R (where the signal falls to 1%):
//
//   S(d) = 1 / (1 + 99 (d / R)^2)
//
// 100% at the gNB, exactly 1% at R, and ~(R^2 / 99) / d^2 further out - the
// inverse-square law - falling towards 0 but never reaching it (a plain
// 1 / d^2 would be infinite at the gNB itself). No range (0 / unset) is
// unlimited: 100% everywhere. It only drives the UI; the UE stays attached
// in the backend whatever it shows.
export function signalAt(d: number, range?: number): number {
  if (!range || range <= 0) return 1
  const x = d / range
  return 1 / (1 + 99 * x * x)
}

// the signal in dB relative to full strength (1% is -20 dB)
export const signalDb = (s: number) => 10 * Math.log10(s)

// "37%" / "0.42%" / "3e-4%": enough digits to tell weak signals apart
export function formatSignal(s: number): string {
  const p = s * 100
  if (p >= 10) return `${p.toFixed(0)}%`
  if (p >= 0.01) return `${p.toPrecision(2)}%`
  return `${p.toExponential(1)}%`
}

// below 1% a UE counts as out of its gNB's range on the map
export const OUT_OF_RANGE = 0.01
