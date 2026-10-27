import {useCallback, useEffect, useState} from 'react'
import {WindowSetBackgroundColour} from '../wailsjs/runtime/runtime'

export type Theme = 'dark' | 'light'

const KEY = 'kofna.theme'

// Fond de fenêtre natif assorti, pour éviter un flash de couleur au redimensionnement.
const background: Record<Theme, [number, number, number]> = {dark: [18, 20, 24], light: [243, 244, 246]}

function initial(): Theme {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'dark' || v === 'light') return v
  } catch {
    // Stockage indisponible : on suit le système.
  }
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

/** Thème choisi par l'utilisateur ; au premier lancement, celui de Windows. */
export function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(initial)

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    const [r, g, b] = background[theme]
    try {
      WindowSetBackgroundColour(r, g, b, 255)
    } catch {
      // Hors Wails (aperçu dans un navigateur) : rien à faire.
    }
  }, [theme])

  const toggle = useCallback(() => {
    setTheme((t) => {
      const next = t === 'dark' ? 'light' : 'dark'
      try {
        localStorage.setItem(KEY, next)
      } catch {
        // Préférence non mémorisée, sans conséquence.
      }
      return next
    })
  }, [])

  return [theme, toggle]
}
