import { useCallback, useEffect, useState } from 'react'
import { api } from '../apiClient'
import type { GnbProfile, ProfileConflict, UeProfile } from '../api'

interface ProfileApi<T> {
  list: () => Promise<T[]>
  create: (p: T) => Promise<T>
  update: (id: string, p: T) => Promise<T>
  remove: (id: string) => Promise<unknown>
  render: (p: T) => Promise<string>
  check: (p: T) => Promise<ProfileConflict[]>
  bulk: (profileId: string, count: number) => Promise<T[]>
}

const gnbApi: ProfileApi<GnbProfile> = {
  list: async () => (await api.gnbProfileList()).data,
  create: async (p) => (await api.gnbProfileCreate(p)).data,
  update: async (id, p) => (await api.gnbProfileUpdate(id, p)).data,
  remove: (id) => api.gnbProfileDelete(id),
  render: async (p) => (await api.gnbProfileRender(p)).data.yaml,
  check: async (p) => (await api.gnbProfileCheck(p)).data.conflicts ?? [],
  bulk: async (profileId, count) => (await api.gnbProfileBulkCreate({ profileId, count })).data,
}

const ueApi: ProfileApi<UeProfile> = {
  list: async () => (await api.ueProfileList()).data,
  create: async (p) => (await api.ueProfileCreate(p)).data,
  update: async (id, p) => (await api.ueProfileUpdate(id, p)).data,
  remove: (id) => api.ueProfileDelete(id),
  render: async (p) => (await api.ueProfileRender(p)).data.yaml,
  check: async (p) => (await api.ueProfileCheck(p)).data.conflicts ?? [],
  bulk: async (profileId, count) => (await api.ueProfileBulkCreate({ profileId, count })).data,
}

// useProfiles loads the saved gNB or UE profiles and saves, deletes,
// copies, checks and previews them (the backend renders the config file a
// profile deploys).
function useProfilesOf<T extends { id?: string; name: string }>(a: ProfileApi<T>) {
  const [profiles, setProfiles] = useState<T[]>([])
  const [loaded, setLoaded] = useState(false)

  const refresh = useCallback(async () => {
    try {
      setProfiles(await a.list())
    } finally {
      setLoaded(true)
    }
  }, [a])

  useEffect(() => { refresh().catch(() => {}) }, [refresh])

  const save = useCallback(async (p: T): Promise<T> => {
    const saved = p.id ? await a.update(p.id, p) : await a.create(p)
    await refresh()
    return saved
  }, [a, refresh])

  const remove = useCallback(async (id: string) => {
    await a.remove(id)
    await refresh()
  }, [a, refresh])

  // bulk copies a saved profile `count` times (the backend counts the
  // unique fields up) and returns the created profiles
  const bulk = useCallback(async (profileId: string, count: number): Promise<T[]> => {
    const created = await a.bulk(profileId, count)
    await refresh()
    return created
  }, [a, refresh])

  return { profiles, loaded, refresh, save, remove, bulk, render: a.render, check: a.check }
}

export const useGnbProfiles = () => useProfilesOf(gnbApi)
export const useUeProfiles = () => useProfilesOf(ueApi)

// LAB_DEFAULT_GNB holds the lab's usual gNB values: a brand-new gNB
// profile starts from them (nothing deploys without a saved profile).
export const LAB_DEFAULT_GNB: GnbProfile = {
  name: 'New gNB profile',
  gnbId: '000314',
  gnbName: 'gNB',
  mcc: '208',
  mnc: '93',
  tac: '000001',
  sst: '1',
  sd: '010203',
  core: 'lab',
  n2Ip: '10.0.1.20',
  logLevel: 'info',
}

// free5GC's default test subscriber, as a starting UE profile
export const NEW_UE_PROFILE: UeProfile = {
  name: 'New UE profile',
  mcc: '208',
  mnc: '93',
  msin: '0000000001',
  permanentKey: '8baf473f2f8fd09487cccbd7097c6862',
  opValue: '8e27b6af0e692e750f32667a3b14605d',
  amf: '8000',
  sqn: '000000000023',
  dnn: 'internet',
  sst: '1',
  sd: '010203',
}

// the UE instance id a profile deploys, imsi-<mcc><mnc><msin>
export function profileUeId(p: UeProfile): string {
  return `imsi-${p.mcc}${p.mnc}${p.msin}`
}

// the gNB target a profile's gNB runs as (the log stream and UE deploys
// use it too)
export function gnbInstanceOf(profileId: string): string {
  return `gnb-${profileId}`
}

// useStoredState is useState kept in localStorage (a per-browser choice,
// shared by every page).
export function useStoredState<T>(key: string, initial: T): [T, (v: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key)
      return raw === null ? initial : (JSON.parse(raw) as T)
    } catch {
      return initial
    }
  })
  const set = useCallback((v: T) => {
    setValue(v)
    try { localStorage.setItem(key, JSON.stringify(v)) } catch { /* private mode */ }
    window.dispatchEvent(new CustomEvent('frulab-stored', { detail: key }))
  }, [key])
  useEffect(() => {
    const on = (e: Event) => {
      if ((e as CustomEvent).detail !== key) return
      try {
        const raw = localStorage.getItem(key)
        if (raw !== null) setValue(JSON.parse(raw) as T)
      } catch { /* ignore */ }
    }
    window.addEventListener('frulab-stored', on)
    return () => window.removeEventListener('frulab-stored', on)
  }, [key])
  return [value, set]
}
