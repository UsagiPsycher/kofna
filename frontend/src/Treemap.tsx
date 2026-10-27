import {useEffect, useLayoutEffect, useMemo, useRef, useState} from 'react'
import {bytes} from './format'
import {Dict, useI18n} from './i18n'
import {Rect, squarify} from './squarify'
import {Category, FsNode} from './types'

/*
 * Treemap « en coussins » (van Wijk & van de Wetering, 1999), comme WinDirStat :
 * toute la hiérarchie est dessinée bord à bord sur un canvas, et c'est l'éclairage
 * de surfaces paraboliques imbriquées qui fait apparaître les dossiers, sans
 * bordure ni en-tête. Les couleurs sont celles des catégories de suppression.
 */

const HEIGHT = 0.38 // hauteur des crêtes au premier niveau
const SCALE = 0.91 // atténuation de la hauteur à chaque niveau
const AMBIENT = 0.2
const BRIGHTNESS = 1.5 // WinDirStat : luminosité 0,88 / luminosité de palette 0,6
const LIGHT = (() => {
  const [x, y, z] = [-1, -1, 10]
  const n = Math.hypot(x, y, z)
  return [x / n, y / n, z / n]
})()

interface Props {
  items: FsNode[]
  marked: ReadonlyMap<string, unknown>
  hovered: string | null
  dimOthers: Category | null
  /** Rectangle d'origine du zoom (tuile cliquée), relatif au conteneur. */
  zoomFrom: Rect | null
  zoomKey: string
  onOpen: (node: FsNode, from: Rect | null) => void
  onToggle: (node: FsNode) => void
  onHover: (path: string | null) => void
}

type Surface = [number, number, number, number] // z = s0·x² + s1·y² + s2·x + s3·y

interface Leaf {
  r: Rect
  s: Surface
  node: FsNode
  top: FsNode // élément du niveau affiché qui contient cette feuille
}

interface Layout {
  leaves: Leaf[]
  rects: Map<string, Rect>
  tops: {r: Rect; node: FsNode}[]
}

function addRidge(r: Rect, s: Surface, h: number): Surface {
  const out: Surface = [...s]
  const x1 = r.x, x2 = r.x + r.w, y1 = r.y, y2 = r.y + r.h
  if (x2 > x1) {
    out[2] += (4 * h * (x2 + x1)) / (x2 - x1)
    out[0] -= (4 * h) / (x2 - x1)
  }
  if (y2 > y1) {
    out[3] += (4 * h * (y2 + y1)) / (y2 - y1)
    out[1] -= (4 * h) / (y2 - y1)
  }
  return out
}

function layout(items: FsNode[], w: number, h: number): Layout {
  const leaves: Leaf[] = []
  const rects = new Map<string, Rect>()
  const tops: Layout['tops'] = []

  const visit = (node: FsNode, r: Rect, s: Surface, height: number, top: FsNode) => {
    const surf = addRidge(r, s, height)
    rects.set(node.path, r)
    const kids = node.kind === 'dir' ? (node.children ?? []).filter((k) => k.size > 0) : []
    if (kids.length > 0 && r.w >= 2 && r.h >= 2) {
      for (const p of squarify(kids.map((k) => ({value: k.size, item: k})), r)) {
        visit(p.item, p, surf, height * SCALE, top)
      }
    } else {
      leaves.push({r, s: surf, node, top})
    }
  }

  const placed = squarify(items.map((n) => ({value: n.size, item: n})), {x: 0, y: 0, w, h})
  for (const p of placed) {
    tops.push({r: p, node: p.item})
    visit(p.item, p, [0, 0, 0, 0], HEIGHT, p.item)
  }
  return {leaves, rects, tops}
}

function cssColor(name: string): [number, number, number] {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  const m = /^#?([0-9a-f]{6})$/i.exec(v)
  if (!m) return [128, 128, 128]
  const n = parseInt(m[1], 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

function paint(canvas: HTMLCanvasElement, lay: Layout, w: number, h: number, dim: Category | null) {
  const dpr = window.devicePixelRatio || 1
  const W = Math.max(1, Math.round(w * dpr))
  const H = Math.max(1, Math.round(h * dpr))
  canvas.width = W
  canvas.height = H
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const img = ctx.createImageData(W, H)
  const px = img.data
  const palette = new Map<string, [number, number, number]>()
  const colorOf = (c: Category) => {
    if (!palette.has(c)) palette.set(c, cssColor(`--cat-${c}`))
    return palette.get(c)!
  }
  const [lx, ly, lz] = LIGHT

  for (const leaf of lay.leaves) {
    let [cr, cg, cb] = colorOf(leaf.node.category)
    let gain = leaf.node.kind === 'small' || leaf.node.kind === 'rest' ? 0.78 : 1
    if (dim && leaf.node.category !== dim) {
      const l = 0.3 * cr + 0.59 * cg + 0.11 * cb
      cr = cg = cb = l
      gain *= 0.35
    }
    const [s0, s1, s2, s3] = leaf.s
    const x0 = Math.round(leaf.r.x * dpr), x1 = Math.round((leaf.r.x + leaf.r.w) * dpr)
    const y0 = Math.round(leaf.r.y * dpr), y1 = Math.round((leaf.r.y + leaf.r.h) * dpr)
    for (let iy = y0; iy < y1; iy++) {
      const y = (iy + 0.5) / dpr
      const ny = -(2 * s1 * y + s3)
      let o = (iy * W + x0) * 4
      for (let ix = x0; ix < x1; ix++) {
        const x = (ix + 0.5) / dpr
        const nx = -(2 * s0 * x + s2)
        const cos = (nx * lx + ny * ly + lz) / Math.sqrt(nx * nx + ny * ny + 1)
        const k = (AMBIENT + (1 - AMBIENT) * Math.max(0, cos)) * BRIGHTNESS * gain
        px[o] = Math.min(255, cr * k)
        px[o + 1] = Math.min(255, cg * k)
        px[o + 2] = Math.min(255, cb * k)
        px[o + 3] = 255
        o += 4
      }
    }
  }
  ctx.putImageData(img, 0, 0)
}

const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches

export function Treemap({items, marked, hovered, dimOthers, zoomFrom, zoomKey, onOpen, onToggle, onHover}: Props) {
  const {t} = useI18n()
  const box = useRef<HTMLDivElement>(null)
  const layer = useRef<HTMLDivElement>(null)
  const canvas = useRef<HTMLCanvasElement>(null)
  const [size, setSize] = useState({w: 0, h: 0})
  const [tip, setTip] = useState<{leaf: Leaf; x: number; y: number} | null>(null)
  const [themeKey, setThemeKey] = useState(0)

  useLayoutEffect(() => {
    const el = box.current
    if (!el) return
    const ro = new ResizeObserver(([e]) => setSize({w: Math.floor(e.contentRect.width), h: Math.floor(e.contentRect.height)}))
    ro.observe(el)
    // Les couleurs viennent des variables CSS : on repeint quand le thème change.
    const mo = new MutationObserver(() => setThemeKey((k) => k + 1))
    mo.observe(document.documentElement, {attributes: true, attributeFilter: ['data-theme']})
    return () => {
      ro.disconnect()
      mo.disconnect()
    }
  }, [])

  const lay = useMemo(() => layout(items, size.w, size.h), [items, size])

  useEffect(() => {
    if (canvas.current && size.w > 0 && size.h > 0) paint(canvas.current, lay, size.w, size.h, dimOthers)
  }, [lay, size, dimOthers, themeKey])

  // Zoom depuis la tuile cliquée vers le plein cadre.
  useLayoutEffect(() => {
    const el = layer.current
    if (!el || !zoomFrom || !size.w || reducedMotion()) return
    el.style.transition = 'none'
    el.style.transformOrigin = '0 0'
    el.style.transform = `translate(${zoomFrom.x}px, ${zoomFrom.y}px) scale(${zoomFrom.w / size.w}, ${zoomFrom.h / size.h})`
    el.style.opacity = '0.35'
    const id = requestAnimationFrame(() => {
      el.style.transition = 'transform 280ms cubic-bezier(.2,.75,.25,1), opacity 220ms ease-out'
      el.style.transform = ''
      el.style.opacity = ''
    })
    return () => cancelAnimationFrame(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [zoomKey])

  const leafAt = (x: number, y: number) =>
    lay.leaves.find((l) => x >= l.r.x && x < l.r.x + l.r.w && y >= l.r.y && y < l.r.y + l.r.h) ?? null

  const pos = (e: React.MouseEvent) => {
    const r = box.current!.getBoundingClientRect()
    return {x: e.clientX - r.left, y: e.clientY - r.top}
  }

  const move = (e: React.MouseEvent) => {
    const {x, y} = pos(e)
    const leaf = leafAt(x, y)
    setTip(leaf ? {leaf, x, y} : null)
    onHover(leaf ? leaf.top.path : null)
  }

  const click = (e: React.MouseEvent) => {
    const {x, y} = pos(e)
    const leaf = leafAt(x, y)
    if (!leaf) return
    const top = leaf.top
    const r = lay.rects.get(top.path) ?? null
    if (top.kind === 'dir') onOpen(top, r)
    else onToggle(top)
  }

  const hoveredTop = lay.tops.find((tp) => tp.node.path === hovered) ?? null
  const markedRects = [...marked.keys()].flatMap((p) => {
    const r = lay.rects.get(p)
    return r ? [{p, r}] : []
  })

  return (
    <div className="treemap" ref={box} onMouseMove={move} onClick={click}
         onMouseLeave={() => {
           setTip(null)
           onHover(null)
         }}>
      <div className="tm-layer" ref={layer}>
        <canvas ref={canvas} className="tm-canvas" style={{width: size.w, height: size.h}}/>
        {lay.tops.map(({r, node}) => r.w > 64 && r.h > 30 && (
          <span key={node.path} className="tm-label" style={{left: r.x, top: r.y, maxWidth: r.w - 10}}>
            <span className="tm-name">{node.kind === 'dir' ? node.name + '/' : displayName(node, t)}</span>
            {r.h > 46 && <span className="tm-size">{bytes(node.size)}</span>}
          </span>
        ))}
        {markedRects.map(({p, r}) => (
          <div key={p} className="tm-marked" style={{left: r.x, top: r.y, width: r.w, height: r.h}}/>
        ))}
        {hoveredTop && (
          <div className="tm-hover" style={{left: hoveredTop.r.x, top: hoveredTop.r.y, width: hoveredTop.r.w, height: hoveredTop.r.h}}>
            {!hoveredTop.node.protected && (hoveredTop.node.kind === 'dir' || hoveredTop.node.kind === 'file') &&
              hoveredTop.r.w >= 30 && hoveredTop.r.h >= 22 && (
              <button className={marked.has(hoveredTop.node.path) ? 'tile-mark on' : 'tile-mark'}
                      aria-label={marked.has(hoveredTop.node.path) ? t.unselect : t.select}
                      aria-pressed={marked.has(hoveredTop.node.path)}
                      onClick={(e) => {
                        e.stopPropagation()
                        onToggle(hoveredTop.node)
                      }}/>
            )}
          </div>
        )}
      </div>
      {tip && <Tip node={tip.leaf.node} x={tip.x} y={tip.y} w={size.w} h={size.h}/>}
    </div>
  )
}

/** Nom affiché d'un nœud : les éléments regroupés n'ont pas de nom propre. */
export function displayName(n: FsNode, t: Dict) {
  if (n.kind === 'small') return t.smallFiles(n.count ?? 0)
  if (n.kind === 'rest') return t.otherItems(n.count ?? 0)
  return n.name
}

export function categoryName(c: Category, t: Dict) {
  return t.categories[c]
}

function Tip({node, x, y, w, h}: {node: FsNode; x: number; y: number; w: number; h: number}) {
  const {t} = useI18n()
  const left = Math.min(x + 14, w - 300)
  const top = y + 18 > h - 70 ? y - 70 : y + 18
  const pseudo = node.kind === 'small' || node.kind === 'rest'
  return (
    <div className="tm-tip" style={{left: Math.max(4, left), top: Math.max(4, top)}}>
      <div className="tip-path">{pseudo ? `${node.path.replace(/\/[^/]*$/, '')}/` : node.path}</div>
      <div className="tip-meta">
        <strong>{bytes(node.size)}</strong>
        <span className={`tip-cat cat-${node.category}`}>{t.categories[node.category]}</span>
        {pseudo && <span>{displayName(node, t)}</span>}
        {node.kind === 'dir' && <span>{t.items(node.items)}</span>}
        {node.protected && !pseudo && <span>{t.protected}</span>}
      </div>
    </div>
  )
}
