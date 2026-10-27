export interface Rect {
  x: number
  y: number
  w: number
  h: number
}

export type Placed<T> = Rect & {item: T}

/**
 * Treemap « squarified » (Bruls, Huizing, van Wijk) : place les éléments, triés
 * par taille décroissante, en rangées dont les tuiles restent proches du carré.
 */
export function squarify<T>(items: {value: number; item: T}[], rect: Rect): Placed<T>[] {
  const valid = items.filter((i) => i.value > 0)
  const total = valid.reduce((s, i) => s + i.value, 0)
  if (total <= 0 || rect.w <= 0 || rect.h <= 0) return []
  const scale = (rect.w * rect.h) / total
  const areas = valid.map((i) => ({item: i.item, area: i.value * scale}))

  const out: Placed<T>[] = []
  let r = {...rect}
  let row: typeof areas = []
  let i = 0
  while (i < areas.length) {
    const side = Math.min(r.w, r.h)
    const candidate = [...row, areas[i]]
    if (row.length === 0 || worst(candidate, side) <= worst(row, side)) {
      row = candidate
      i++
    } else {
      r = layoutRow(row, r, out)
      row = []
    }
  }
  if (row.length) layoutRow(row, r, out)
  return out
}

function worst(row: {area: number}[], side: number): number {
  let sum = 0
  let max = 0
  let min = Infinity
  for (const r of row) {
    sum += r.area
    max = Math.max(max, r.area)
    min = Math.min(min, r.area)
  }
  const s2 = sum * sum
  const side2 = side * side
  return Math.max((side2 * max) / s2, s2 / (side2 * min))
}

function layoutRow<T>(row: {item: T; area: number}[], r: Rect, out: Placed<T>[]): Rect {
  const sum = row.reduce((s, x) => s + x.area, 0)
  if (r.w >= r.h) {
    // Colonne le long du bord gauche.
    const colW = sum / r.h
    let y = r.y
    for (const x of row) {
      const h = x.area / colW
      out.push({x: r.x, y, w: colW, h, item: x.item})
      y += h
    }
    return {x: r.x + colW, y: r.y, w: r.w - colW, h: r.h}
  }
  // Rangée le long du bord haut.
  const rowH = sum / r.w
  let x0 = r.x
  for (const x of row) {
    const w = x.area / rowH
    out.push({x: x0, y: r.y, w, h: rowH, item: x.item})
    x0 += w
  }
  return {x: r.x, y: r.y + rowH, w: r.w, h: r.h - rowH}
}
