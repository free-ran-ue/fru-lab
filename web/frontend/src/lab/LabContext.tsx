import { createContext, useContext, type ReactNode } from 'react'
import { useLabState, type Lab } from './useLab'

const LabContext = createContext<Lab | null>(null)

// LabProvider polls the lab once for the whole signed-in app; every view
// reads the same state through useLab.
export function LabProvider({ children }: { children: ReactNode }) {
  const lab = useLabState()
  return <LabContext.Provider value={lab}>{children}</LabContext.Provider>
}

export function useLab(): Lab {
  const lab = useContext(LabContext)
  if (!lab) throw new Error('useLab outside LabProvider')
  return lab
}
