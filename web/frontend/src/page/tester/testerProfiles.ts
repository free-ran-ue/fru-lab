import type { TesterStoredProfile } from '../../api'

// LAST_PROFILE_KEY remembers, per browser, which saved profile the setup
// page had open.
const LAST_PROFILE_KEY = 'tester-profile-id'

export function lastProfileId(): string | null {
  try {
    return localStorage.getItem(LAST_PROFILE_KEY)
  } catch {
    return null
  }
}

export function rememberProfileId(id: string | null) {
  try {
    if (id) localStorage.setItem(LAST_PROFILE_KEY, id)
    else localStorage.removeItem(LAST_PROFILE_KEY)
  } catch {
    // only a convenience
  }
}

// sameName matches names the way fru-lab does: ignoring case and
// surrounding spaces.
export function sameName(a: string, b: string): boolean {
  return a.trim().toLowerCase() === b.trim().toLowerCase()
}

// nameOwner is the saved profile other than exceptId that already uses
// name, if any.
export function nameOwner(list: TesterStoredProfile[], name: string, exceptId: string | null): TesterStoredProfile | undefined {
  return list.find((p) => p.id !== exceptId && sameName(p.profile.name, name))
}

// freeName is base, or "base 2", "base 3", … whichever no saved profile
// uses first.
export function freeName(list: TesterStoredProfile[], base: string): string {
  const b = base.trim()
  let candidate = b
  for (let i = 2; nameOwner(list, candidate, null); i++) candidate = `${b} ${i}`
  return candidate
}

// byName sorts profiles the way fru-lab lists them.
export function byName(list: TesterStoredProfile[]): TesterStoredProfile[] {
  return [...list].sort((a, b) => a.profile.name.trim().toLowerCase().localeCompare(b.profile.name.trim().toLowerCase()))
}
