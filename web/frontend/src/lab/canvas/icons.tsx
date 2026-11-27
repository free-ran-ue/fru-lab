import type { ReactElement } from 'react'

// Node glyphs for the lab canvas, drawn in a 24×24 box with currentColor.
export type IconName = 'core' | 'nf' | 'gnb' | 'ue' | 'fleet' | 'cloud'

const PATHS: Record<IconName, ReactElement> = {
  core: (
    <>
      <rect x="3.5" y="4" width="17" height="6" rx="1.6" />
      <rect x="3.5" y="14" width="17" height="6" rx="1.6" />
      <path d="M7 7h.01M7 17h.01M11 7h6M11 17h6" />
    </>
  ),
  nf: (
    <>
      <path d="M12 3.5l7.5 4.25v8.5L12 20.5l-7.5-4.25v-8.5z" />
      <path d="M12 12l7.5-4.25M12 12v8.5M12 12L4.5 7.75" />
    </>
  ),
  gnb: (
    <>
      <path d="M12 10v10.5M8.5 20.5h7M12 10l-3.2 10.5M12 10l3.2 10.5" />
      <path d="M8.4 6.6a5 5 0 0 1 7.2 0M5.8 4a8.6 8.6 0 0 1 12.4 0" />
      <circle cx="12" cy="9.4" r="1.2" />
    </>
  ),
  ue: (
    <>
      <rect x="7" y="2.5" width="10" height="19" rx="2.2" />
      <path d="M10.5 18h3" />
    </>
  ),
  fleet: (
    <>
      <rect x="3" y="5" width="7" height="13" rx="1.6" />
      <rect x="14" y="5" width="7" height="13" rx="1.6" />
      <path d="M5.5 15.5h2M16.5 15.5h2" />
    </>
  ),
  cloud: (
    <path d="M7.5 18.5h9.25a4.25 4.25 0 0 0 .6-8.46A5.75 5.75 0 0 0 6.2 11.3 3.6 3.6 0 0 0 7.5 18.5z" />
  ),
}

export function NodeIcon({ name, x, y, size = 22 }: { name: IconName; x: number; y: number; size?: number }) {
  return (
    <svg x={x} y={y} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {PATHS[name]}
    </svg>
  )
}
