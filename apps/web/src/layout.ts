export interface Box {
  id: string
  kind: string
  label: string
  w: number
  h: number
}

export interface Point {
  x: number
  y: number
}

export interface Marble {
  id: string
  x: number
  y: number
  r: number
}

const rank: Record<string, number> = {
  router: 0,
  network: 1,
  host: 2,
  device: 3,
  network_interface: 4,
  address: 5,
  docker_engine: 6,
  docker_network: 7,
  docker_container: 8,
  docker_volume: 9,
  service: 10,
}

function minor(label: string) {
  return label.startsWith('veth') || label.startsWith('br-') || label === 'docker0'
}

function sorted(boxes: Box[]) {
  return [...boxes].sort((a, b) => {
    const small = Number(minor(a.label)) - Number(minor(b.label))
    if (small) return small
    const order = (rank[a.kind] ?? 30) - (rank[b.kind] ?? 30)
    if (order) return order
    return a.label.localeCompare(b.label, 'fr')
  })
}

export function lanes(boxes: Box[]) {
  const kinds = new Set(boxes.map((box) => box.kind))
  const order = sorted(boxes)
  let groups = [['']]
  if (kinds.has('router') || (kinds.has('host') && kinds.has('device'))) groups = [['router'], ['host', 'network'], ['device']]
  else if (kinds.has('network_interface') || kinds.has('docker_engine')) groups = [['network_interface'], ['docker_engine']]
  else if (kinds.has('docker_container') || kinds.has('docker_network') || kinds.has('docker_volume')) groups = [['docker_network'], ['docker_container'], ['docker_volume', 'service']]
  const used = new Set<string>()
  const rows: Box[][] = []
  for (const group of groups) {
    const row = order.filter((box) => group[0] === '' || group.includes(box.kind))
    row.forEach((box) => used.add(box.id))
    if (row.length) rows.push(row)
  }
  const rest = order.filter((box) => !used.has(box.id))
  if (rest.length) rows.push(rest)
  return rows
}

export function pack(rows: Box[][], gap = 36) {
  const points = new Map<string, Point>()
  let y = 0
  for (const row of rows) {
    const width = Math.max(1, Math.ceil(Math.sqrt(row.length)))
    const lines: Box[][] = []
    for (let index = 0; index < row.length; index += width) lines.push(row.slice(index, index + width))
    if (row.length <= 4) lines.splice(0, lines.length, row)
    for (const line of lines) {
      let x = 0
      let rowH = 0
      for (const box of line) {
        points.set(box.id, { x: x + box.w / 2, y: y + box.h / 2 })
        x += box.w + gap
        rowH = Math.max(rowH, box.h)
      }
      y += rowH + gap
    }
  }
  return points
}

export function separate(items: Marble[], fixed: Set<string>, steps: number) {
  const next = items.map((item) => ({ ...item }))
  for (let step = 0; step < steps; step += 1) {
    const shifts = next.map(() => ({ x: 0, y: 0 }))
    for (let i = 0; i < next.length; i += 1) {
      if (fixed.has(next[i].id)) continue
      for (let j = 0; j < next.length; j += 1) {
        if (i === j) continue
        let vx = next[i].x - next[j].x
        let vy = next[i].y - next[j].y
        let dist = Math.hypot(vx, vy)
        if (dist < 0.5) {
          vx = ((i * 17 + j) % 7) - 3
          vy = ((i * 13 + j) % 7) - 3
          dist = Math.hypot(vx, vy) || 1
        }
        const min = next[i].r + next[j].r
        if (dist >= min) continue
        const push = ((min - dist) / dist) * 0.55
        shifts[i].x += vx * push
        shifts[i].y += vy * push
      }
    }
    for (let i = 0; i < next.length; i += 1) {
      if (fixed.has(next[i].id)) continue
      const length = Math.hypot(shifts[i].x, shifts[i].y)
      const cap = length > 90 ? 90 / length : 1
      next[i].x += shifts[i].x * cap
      next[i].y += shifts[i].y * cap
    }
  }
  return next
}
