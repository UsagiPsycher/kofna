import {useCallback, useEffect, useMemo, useRef, useState} from 'react'
import * as api from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {Ask} from './Confirm'
import {bytes, time} from './format'
import {useI18n} from './i18n'
import {Rect, squarify} from './squarify'
import {categoryName, displayName, Treemap} from './Treemap'
import {Category, Cleanup, DeleteResult, Distro, ExplorerStatus, FsNode, FsView, ScanEvent} from './types'

type Run = <T>(fn: () => Promise<T>) => Promise<T | undefined>

interface Props {
  distro: Distro
  /** Onglet visible : le premier affichage déclenche le scan. */
  active: boolean
  busy: boolean
  run: Run
  ask: Ask
  onGoSpace: () => void
}

const cast = <T, >(p: Promise<unknown>) => p as Promise<T>

function parentOf(p: string) {
  if (p === '/') return '/'
  const i = p.lastIndexOf('/')
  return i <= 0 ? '/' : p.slice(0, i)
}

/** Total d'une sélection, sans compter deux fois un élément contenu dans un autre. */
function selectionSize(marked: Map<string, FsNode>) {
  const paths = [...marked.keys()].sort()
  let total = 0
  let last = ''
  for (const p of paths) {
    if (last && (p === last || p.startsWith(last.replace(/\/$/, '') + '/'))) continue
    total += marked.get(p)!.size
    last = p
  }
  return total
}

export function Explorer({distro, active, busy, run, ask, onGoSpace}: Props) {
  const {t} = useI18n()
  const name = distro.name
  const [status, setStatus] = useState<ExplorerStatus | null>(null)
  const [view, setView] = useState<FsView | null>(null)
  const [mode, setMode] = useState<'dirs' | 'files'>('dirs')
  const [topFiles, setTopFiles] = useState<FsNode[] | null>(null)
  const [cleanups, setCleanups] = useState<Cleanup[]>([])
  const [marked, setMarked] = useState<Map<string, FsNode>>(new Map())
  const [hovered, setHovered] = useState<string | null>(null)
  const [dim, setDim] = useState<Category | null>(null)
  const [zoom, setZoom] = useState<{from: Rect | null; key: string}>({from: null, key: '/'})
  const [focus, setFocus] = useState(0)
  const [scan, setScan] = useState<ScanEvent | null>(null)
  const [freed, setFreed] = useState<number | null>(null)
  const [scanning, setScanning] = useState(false)
  const listRef = useRef<HTMLDivElement>(null)
  const autoScanned = useRef(false)

  const path = view?.node.path ?? '/'

  const loadView = useCallback(async (p: string, from: Rect | null = null) => {
    const v = await run(() => cast<FsView>(api.View(name, p)))
    if (!v) return
    setView(v)
    setZoom({from, key: v.node.path + ':' + Date.now()})
    setFocus(0)
  }, [name, run])

  const refreshAfterChange = useCallback(async (p: string) => {
    const [st, cl] = await Promise.all([
      cast<ExplorerStatus>(api.ExplorerStatus(name)),
      cast<Cleanup[]>(api.Cleanups(name)).catch(() => []),
    ])
    setStatus(st)
    setCleanups(cl ?? [])
    setTopFiles(null)
    const v = await cast<FsView>(api.View(name, p)).catch(() => cast<FsView>(api.View(name, '/')))
    setView(v)
  }, [name])

  useEffect(() => EventsOn('scan', (e: ScanEvent) => setScan(e)), [])

  // Rien n'est chargé tant que l'onglet n'a pas été ouvert (interroger ncdu démarre la distribution).
  useEffect(() => {
    if (!active || status) return
    cast<ExplorerStatus>(api.ExplorerStatus(name)).then((st) => {
      setStatus(st)
      if (st.scanned) refreshAfterChange('/')
    })
  }, [active, status, name, refreshAfterChange])

  useEffect(() => {
    if (mode === 'files' && status?.scanned && topFiles === null) {
      cast<FsNode[]>(api.TopFiles(name)).then(setTopFiles)
    }
  }, [mode, status, topFiles, name])

  const doScan = async () => {
    setScan(null)
    setMarked(new Map())
    setFreed(null)
    setScanning(true)
    const st = await run(() => cast<ExplorerStatus>(api.Scan(name)))
    setScanning(false)
    if (!st) return
    setStatus(st)
    await refreshAfterChange('/')
    setZoom({from: null, key: 'scan:' + Date.now()})
  }

  // Premier affichage : scan automatique, dès qu'aucune autre opération n'est en cours.
  useEffect(() => {
    if (!active || autoScanned.current || !status || status.scanned || !status.ncdu || busy) return
    autoScanned.current = true
    doScan()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, status, busy])

  const install = async () => {
    const st = await run(() => cast<ExplorerStatus>(api.InstallNcdu(name)))
    if (st) {
      setStatus(st)
      doScan()
    }
  }

  const toggle = (n: FsNode) => {
    if (n.protected || (n.kind !== 'dir' && n.kind !== 'file')) return
    setMarked((m) => {
      const next = new Map(m)
      if (next.has(n.path)) next.delete(n.path)
      else next.set(n.path, n)
      return next
    })
    setFreed(null)
  }

  const open = (n: FsNode, from: Rect | null = null) => {
    if (n.kind === 'dir') {
      setMode('dirs')
      loadView(n.path, from)
    }
  }

  const deleteMarked = async () => {
    if (marked.size === 0 || busy) return
    const list = [...marked.values()].sort((a, b) => b.size - a.size)
    const total = selectionSize(marked)
    const ok = await ask({
      title: t.deleteTitle(bytes(total)),
      confirm: t.delete,
      danger: true,
      body: (
        <ul className="del-list">
          {list.slice(0, 7).map((n) => (
            <li key={n.path}><code>{n.path}</code><span className="num">{bytes(n.size)}</span></li>
          ))}
          {list.length > 7 && <li className="muted">{t.andMore(list.length - 7)}</li>}
        </ul>
      ),
    })
    if (!ok) return
    const res = await run(() => cast<DeleteResult>(api.Delete(name, list.map((n) => n.path))))
    const deleted = new Set(res?.deleted ?? [])
    setMarked((m) => new Map([...m].filter(([p]) => !deleted.has(p))))
    if (res) setFreed((f) => (f ?? 0) + res.freed)
    await refreshAfterChange(path)
  }

  const runCleanup = async (c: Cleanup) => {
    const ok = await ask({
      title: `${t.cleanups[c.key] ?? c.key}: ${bytes(c.size)}`,
      confirm: t.empty,
      danger: true,
      body: <p><code>{c.command}</code></p>,
    })
    if (!ok) return
    const before = status?.freed ?? 0
    const st = await run(() => cast<ExplorerStatus>(api.RunCleanup(name, c.id)))
    if (st) setFreed((f) => (f ?? 0) + st.freed - before)
    await refreshAfterChange(path)
  }

  const rows: FsNode[] = useMemo(
    () => (mode === 'dirs' ? view?.node.children ?? [] : topFiles ?? []),
    [mode, view, topFiles],
  )

  useEffect(() => {
    listRef.current?.querySelector('.fs-row.focus')?.scrollIntoView({block: 'nearest'})
  }, [focus])

  // --- États vides ---------------------------------------------------------

  if (distro.managed) {
    return <div className="ex-empty"><p className="muted">{t.managedDocker}</p></div>
  }
  if (!status) return <div className="ex-empty"><span className="spinner"/></div>

  if (!status.scanned || (scanning && !view)) {
    return (
      <div className="ex-empty">
        <Ghost progress={scanning ? (scan && scan.total > 0 ? scan.bytes / scan.total : 0) : null}/>
        {scanning ? (
          <div className="ex-scan">
            <strong className="num">{bytes(scan?.bytes ?? 0)}{scan && scan.total > 0 && <span className="muted"> / {bytes(scan.total)}</span>}</strong>
            <span className="muted num">{t.items(scan?.items ?? 0)}</span>
          </div>
        ) : status.ncdu ? (
          // Le scan automatique a échoué ou attend la fin d'une autre opération.
          autoScanned.current && !busy && (
            <div className="ex-cta">
              <button className="btn primary big" onClick={doScan}>{t.scanDistro(name)}</button>
            </div>
          )
        ) : (
          <div className="ex-cta">
            <button className="btn primary big" disabled={busy} onClick={install}>{t.installNcdu}</button>
            <span className="muted small">{t.ncduMissing(name)}</span>
          </div>
        )}
      </div>
    )
  }

  // --- Vue principale ----------------------------------------------------

  const max = rows.reduce((m, r) => Math.max(m, r.size), 0)
  const crumbs = path === '/' ? [] : path.slice(1).split('/')
  const cats = Object.entries(view?.categories ?? {})
    .filter(([, v]) => (v ?? 0) > 0)
    .sort((a, b) => (b[1] ?? 0) - (a[1] ?? 0)) as [Category, number][]
  const catTotal = cats.reduce((s, [, v]) => s + v, 0)
  const selTotal = selectionSize(marked)

  return (
    <div className="explorer">
      <div className="ex-bar">
        <nav className="crumbs" aria-label={t.path}>
          <button className="crumb up" disabled={mode !== 'dirs' || path === '/'} onClick={() => loadView(parentOf(path))}
                  aria-label={t.parentFolder}>↑</button>
          <button className="crumb" onClick={() => {
            setMode('dirs')
            loadView('/')
          }}>/</button>
          {mode === 'dirs' && crumbs.map((c, i) => {
            const p = '/' + crumbs.slice(0, i + 1).join('/')
            return (
              <span key={p} className="crumb-wrap">
                <span className="crumb-sep" aria-hidden>›</span>
                <button className="crumb" aria-current={i === crumbs.length - 1 ? 'page' : undefined}
                        onClick={() => loadView(p)}>{c}</button>
              </span>
            )
          })}
        </nav>
        <div className="segctl" role="tablist">
          <button role="tab" aria-selected={mode === 'dirs'} className={mode === 'dirs' ? 'on' : ''}
                  onClick={() => setMode('dirs')}>{t.folders}</button>
          <button role="tab" aria-selected={mode === 'files'} className={mode === 'files' ? 'on' : ''}
                  onClick={() => setMode('files')}>{t.bigFiles}</button>
        </div>
        <button className="btn ghost small" disabled={busy} onClick={doScan}
                title={t.rescanTip(bytes(status.total), t.items(status.items), time(status.scannedAt))}>
          {t.rescan}
        </button>
      </div>

      {mode === 'dirs' && cats.length > 0 && (
        <div className="cats" onMouseLeave={() => setDim(null)}>
          <div className="cat-bar" aria-hidden>
            {cats.map(([c, v]) => <span key={c} className={`cat-${c}`} style={{flexGrow: v / catTotal}}/>)}
          </div>
          {cats.map(([c, v]) => (
            <button key={c} className={`cat-chip cat-${c} ${dim === c ? 'on' : ''}`}
                    onMouseEnter={() => setDim(c)} onFocus={() => setDim(c)} onBlur={() => setDim(null)}>
              <i/>{categoryName(c, t)}<span className="num">{bytes(v)}</span>
            </button>
          ))}
        </div>
      )}

      <div className="ex-main">
        <Treemap items={rows.filter((r) => r.size > 0)} marked={marked} hovered={hovered}
                 dimOthers={dim} zoomFrom={zoom.from} zoomKey={zoom.key + mode}
                 onOpen={open} onToggle={toggle} onHover={setHovered}/>

        <div className="fs-rows" ref={listRef} role="listbox"
             aria-label={mode === 'dirs' ? path : t.bigFiles} aria-multiselectable>
          {mode === 'files' && topFiles === null && <div className="fs-rows-empty"><span className="spinner"/></div>}
          {rows.map((r, i) => {
            const pseudo = r.kind === 'small' || r.kind === 'rest'
            const isMarked = marked.has(r.path)
            const dir = mode === 'files' ? parentOf(r.path) : ''
            return (
              <div key={r.path}
                   className={['fs-row', `cat-${r.category}`, pseudo ? 'pseudo' : '', r.protected && !pseudo ? 'locked' : '',
                     isMarked ? 'marked' : '', i === focus ? 'focus' : '', hovered === r.path ? 'hover' : ''].join(' ')}
                   role="option" aria-selected={isMarked}
                   onMouseEnter={() => setHovered(r.path)} onMouseLeave={() => setHovered(null)}
                   onClick={() => {
                     setFocus(i)
                     if (r.kind === 'dir') open(r)
                     else toggle(r)
                   }}>
                {pseudo || r.protected ? (
                  <span className="fs-check none" aria-hidden>{r.protected && !pseudo ? '🔒︎' : ''}</span>
                ) : (
                  <button className={isMarked ? 'fs-check on' : 'fs-check'} aria-label={isMarked ? t.unselect : t.select}
                          onClick={(e) => {
                            e.stopPropagation()
                            toggle(r)
                          }}/>
                )}
                <span className="fs-bar"><i style={{width: `${max > 0 ? (r.size / max) * 100 : 0}%`}}/></span>
                <span className="fs-size num">{bytes(r.size)}</span>
                <span className="fs-name">
                  {displayName(r, t)}{r.kind === 'dir' && <span className="slash">/</span>}
                  {mode === 'files' && <span className="dirpart"> {dir}</span>}
                </span>
              </div>
            )
          })}
        </div>
      </div>

      {(marked.size > 0 || freed) ? (
        <div className={marked.size > 0 ? 'tray' : 'tray done'}>
          {marked.size > 0 ? (
            <>
              <span><strong className="num">{bytes(selTotal)}</strong> <span className="muted">{t.inItems(marked.size)}</span></span>
              <span className="tray-actions">
                <button className="btn ghost small" onClick={() => setMarked(new Map())}>{t.deselect}</button>
                <button className="btn danger" disabled={busy} onClick={deleteMarked}>{t.delete}</button>
              </span>
            </>
          ) : (
            <>
              <span><strong className="num gain-text">{bytes(freed ?? 0)}</strong> <span className="muted">{t.freedInLinux}</span></span>
              <button className="btn primary" onClick={onGoSpace}>{t.trimAndCompact}</button>
            </>
          )}
        </div>
      ) : null}

      {cleanups.length > 0 && (
        <div className="cleanups">
          {cleanups.map((c) => (
            <div key={c.id} className={`cleanup cat-${c.category}`} title={c.path}>
              <span className="cl-label">{t.cleanups[c.key] ?? c.key}</span>
              <span className="cl-path">{c.path.replace(/^\/home\/[^/]+/, '~')}</span>
              <span className="cl-size num">{bytes(c.size)}</span>
              <button className="btn small" disabled={busy} onClick={() => runCleanup(c)}>{t.empty}</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

/** Treemap fantôme des états vides ; ses tuiles se remplissent pendant le scan. */
function Ghost({progress}: {progress: number | null}) {
  const tiles = useMemo(() => {
    const values = [34, 21, 13, 9, 8, 5, 3, 3, 2, 1, 1]
    return squarify(values.map((v, i) => ({value: v, item: i})), {x: 0, y: 0, w: 520, h: 250})
  }, [])
  const cats: Category[] = ['data', 'cache', 'deps', 'data', 'logs', 'docker', 'cache', 'data', 'deps', 'logs', 'data']
  return (
    <div className="ghost-map" aria-hidden>
      {tiles.map((t, i) => {
        const filled = progress !== null && (i + 1) / tiles.length <= progress + 0.04
        return <div key={i} className={`gtile cat-${cats[i]} ${filled ? 'fill' : ''}`}
                    style={{left: t.x, top: t.y, width: t.w, height: t.h}}/>
      })}
    </div>
  )
}
