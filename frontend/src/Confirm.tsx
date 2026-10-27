import {ReactNode, useCallback, useEffect, useRef, useState} from 'react'
import {useI18n} from './i18n'

export interface ConfirmOptions {
  title: string
  body: ReactNode
  confirm: string
  danger?: boolean
  /** Texte à retaper pour valider (opérations destructrices). */
  typeToConfirm?: string
}

type Pending = ConfirmOptions & {resolve: (ok: boolean) => void}

export type Ask = (o: ConfirmOptions) => Promise<boolean>

/** Boîte de confirmation modale, utilisable comme `await ask({...})`. */
export function useConfirm(): {ask: Ask; modal: ReactNode} {
  const [pending, setPending] = useState<Pending | null>(null)
  const [typed, setTyped] = useState('')
  const okRef = useRef<HTMLButtonElement>(null)
  const {t} = useI18n()

  const ask = useCallback<Ask>((o) => new Promise((resolve) => {
    setTyped('')
    setPending({...o, resolve})
  }), [])

  const close = (ok: boolean) => {
    pending?.resolve(ok)
    setPending(null)
  }

  useEffect(() => {
    if (!pending) return
    if (!pending.typeToConfirm) okRef.current?.focus()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && close(false)
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const blocked = !!pending?.typeToConfirm && typed !== pending.typeToConfirm

  const modal = pending && (
    <div className="modal-backdrop" onMouseDown={() => close(false)}>
      <div className="modal" role="dialog" aria-modal="true" aria-labelledby="modal-title"
           onMouseDown={(e) => e.stopPropagation()}>
        <h2 id="modal-title">{pending.title}</h2>
        <div className="modal-body">{pending.body}</div>
        {pending.typeToConfirm && (
          <label className="type-confirm">
            {t.typeToConfirm[0]} <code>{pending.typeToConfirm}</code> {t.typeToConfirm[1]}
            <input autoFocus value={typed} onChange={(e) => setTyped(e.target.value)}
                   onKeyDown={(e) => e.key === 'Enter' && !blocked && close(true)}/>
          </label>
        )}
        <div className="modal-actions">
          <button className="btn ghost" onClick={() => close(false)}>{t.cancel}</button>
          <button ref={okRef} className={pending.danger ? 'btn danger' : 'btn primary'}
                  disabled={blocked} onClick={() => close(true)}>{pending.confirm}</button>
        </div>
      </div>
    </div>
  )

  return {ask, modal}
}
