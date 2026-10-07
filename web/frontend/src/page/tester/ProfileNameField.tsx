import { useEffect, useRef, useState } from 'react'
import type { TesterStoredProfile } from '../../api'
import styles from './tester.module.css'

interface ProfileNameFieldProps {
  value: string
  // the saved profiles to pick from; null while loading
  profiles: TesterStoredProfile[] | null
  currentId: string | null
  message?: string
  onChange: (name: string) => void
  onPick: (profile: TesterStoredProfile) => void
  onNew: () => void
}

// ProfileNameField is the profile's name: type to name or rename it, or
// open the menu to switch to another saved profile or start a new one.
export default function ProfileNameField({ value, profiles, currentId, message, onChange, onPick, onNew }: ProfileNameFieldProps) {
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)

  // close when a click or focus lands outside the field
  useEffect(() => {
    if (!open) return
    const outside = (e: Event) => {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', outside)
    document.addEventListener('focusin', outside)
    return () => {
      document.removeEventListener('mousedown', outside)
      document.removeEventListener('focusin', outside)
    }
  }, [open])

  const choose = (f: () => void) => {
    setOpen(false)
    f()
  }

  return (
    <div className={`${styles.field} ${styles.combo}`} ref={box}
      onKeyDown={(e) => {
        if (e.key === 'Escape') setOpen(false)
        if (e.key === 'ArrowDown' && !open) setOpen(true)
      }}>
      <label htmlFor="name">Profile name</label>
      <input id="name" className={`${styles.input} ${message ? styles.inputError : ''}`} type="text" value={value}
        autoComplete="off" role="combobox" aria-expanded={open} aria-controls="profile-menu"
        onChange={(e) => onChange(e.target.value)} />
      <button type="button" className={styles.comboToggle} aria-label="Saved profiles" title="Saved profiles"
        onClick={() => setOpen((o) => !o)}>▾</button>
      {open && (
        <ul id="profile-menu" className={styles.comboMenu} role="listbox">
          <li>
            <button type="button" className={`${styles.comboItem} ${styles.comboNew}`} onClick={() => choose(onNew)}>
              + New profile
            </button>
          </li>
          {profiles === null && <li className={styles.comboItem}>Loading…</li>}
          {profiles?.length === 0 && <li className={`${styles.comboItem} ${styles.comboMeta}`}>No saved profiles yet</li>}
          {profiles?.map((p) => (
            <li key={p.id} role="option" aria-selected={p.id === currentId}>
              <button type="button" className={`${styles.comboItem} ${p.id === currentId ? styles.comboItemCurrent : ''}`}
                onClick={() => choose(() => onPick(p))}>
                <span>{p.profile.name}</span>
                <span className={styles.comboMeta}>
                  {p.profile.scale.gnbCount} gNB · {p.profile.scale.ueCount} UE
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {message && <p className={styles.fieldError}>{message}</p>}
    </div>
  )
}
