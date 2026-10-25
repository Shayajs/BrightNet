<script setup lang="ts">
import cytoscape, { type Core, type ElementDefinition, type NodeSingular } from 'cytoscape'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { lanes, pack, separate, type Marble } from '../layout'
import type { Graph, Position } from '../types'

const props = defineProps<{
  graph: Graph
  positions: Record<string, Position>
  query: string
  kinds: Record<string, boolean>
  animations: boolean
  selectedId: string
}>()

const emit = defineEmits<{
  select: [id: string]
  move: [positions: Record<string, Position>]
}>()

const host = ref<HTMLElement | null>(null)
let cy: Core | null = null
let arranged = false
let arrangedMode = ''
let arrangedIds: string[] = []
let dragFrame = 0
let flowFrame = 0
let flowing = false
const surges = new Map<string, number>()
const memory = new Map<string, Position>()
const collapsed = new Set<string>()
const previousStatus = new Map<string, string>()

const stylesheet = [
  { selector: 'node', style: { label: 'data(label)', color: '#e7ecdf', 'font-family': 'IBM Plex Mono, ui-monospace, monospace', 'font-size': 11, 'text-wrap': 'ellipsis', 'text-max-width': '140px', 'text-valign': 'bottom', 'text-margin-y': 8, 'background-color': '#243024', 'border-width': 1.5, 'border-color': '#d6f25c', width: 22, height: 22, 'overlay-padding': 6 } },
  { selector: 'node.host', style: { shape: 'round-rectangle', width: 58, height: 58, 'background-color': '#1d3148', 'border-color': '#8eb6ff', 'font-size': 13 } },
  { selector: 'node.router', style: { shape: 'diamond', width: 74, height: 74, 'background-color': '#243044', 'border-color': '#9eb7e8', 'border-width': 3, 'font-size': 13 } },
  { selector: 'node.device', style: { shape: 'ellipse', width: 40, height: 40, 'background-color': '#243028', 'border-color': '#d6f25c' } },
  { selector: 'node.network', style: { shape: 'round-rectangle', 'background-color': '#182018', 'border-color': '#6d8a55' } },
  { selector: 'node.docker_engine', style: { shape: 'hexagon', width: 36, height: 36, 'background-color': '#163c3c', 'border-color': '#7ed0d0' } },
  { selector: 'node.docker_container', style: { shape: 'ellipse', width: 18, height: 18, 'background-color': '#1d3324', 'border-color': '#8ed38a' } },
  { selector: 'node.docker_network', style: { shape: 'diamond', width: 20, height: 20, 'background-color': '#2c2440', 'border-color': '#c9a6f5' } },
  { selector: 'node.docker_volume', style: { shape: 'round-rectangle', width: 18, height: 14, 'background-color': '#3a321c', 'border-color': '#e6c07b' } },
  { selector: 'node.service', style: { shape: 'cut-rectangle', width: 16, height: 12, 'background-color': '#3a281c', 'border-color': '#f0a36b' } },
  { selector: 'node.network_interface', style: { shape: 'rectangle', width: 30, height: 18, 'background-color': '#1c3330', 'border-color': '#7ec8a3' } },
  { selector: 'node.primary-iface', style: { width: 42, height: 26, 'border-width': 2 } },
  { selector: 'node.minor', style: { width: 12, height: 12, opacity: 0.55 } },
  { selector: 'node.address', style: { shape: 'ellipse', width: 14, height: 14, 'background-color': '#22282a', 'border-color': '#b7c4c0' } },
  { selector: 'node.exited, node.paused, node.created', style: { 'border-color': '#e08a72', 'background-color': '#3a2420' } },
  { selector: 'node.absent, node.offline, node.dead', style: { opacity: 0.4, 'border-style': 'dashed' } },
  { selector: 'node.simulated', style: { 'border-style': 'dashed' } },
  { selector: 'node.just-changed', style: { 'border-width': 5, 'border-color': '#d6f25c' } },
  { selector: ':parent', style: { 'background-color': '#d6f25c', 'background-opacity': 0.06, 'border-color': '#3c4a34', 'border-width': 1, shape: 'round-rectangle', padding: '28px', 'text-valign': 'top', 'text-margin-y': -6, color: '#c5d2b4' } },
  { selector: 'edge', style: { width: 1.2, 'line-color': '#44523c', 'curve-style': 'bezier', 'target-arrow-shape': 'triangle', 'target-arrow-color': '#44523c', 'arrow-scale': 0.7, opacity: 0.85 } },
  { selector: 'edge.dbncp', style: { width: 2.2, 'line-color': '#d6f25c', 'line-style': 'dashed', 'line-dash-pattern': [3, 9], 'target-arrow-shape': 'none', label: 'data(label)', color: '#d6f25c', 'font-size': 11, 'font-family': 'IBM Plex Mono, monospace', 'text-background-color': '#121611', 'text-background-opacity': 0.9, 'text-background-padding': '3px' } },
  { selector: 'edge.dbncp.surge', style: { width: 4.5, 'line-color': '#f6ffc2', 'line-dash-pattern': [2, 7], 'target-arrow-color': '#f6ffc2', color: '#f6ffc2' } },
  { selector: '.dim', style: { opacity: 0.12 } },
] as unknown as cytoscape.StylesheetJson

function visibleNodes() {
  const query = props.query.trim().toLowerCase()
  const nodes = props.graph.nodes.filter((node) => {
    if (props.kinds[node.kind] === false) return false
    if (query && !node.label.toLowerCase().includes(query) && !node.kind.includes(query)) return false
    return true
  })
  return nodes.filter((node) => {
    let parent = node.parent_id
    while (parent) {
      if (collapsed.has(parent)) return false
      parent = props.graph.nodes.find((item) => item.id === parent)?.parent_id
    }
    return true
  })
}

function classesOf(node: Graph['nodes'][number], flash: boolean) {
  const classes = [node.kind, node.status, node.simulated ? 'simulated' : 'live', flash ? 'just-changed' : '']
  if (node.kind === 'network_interface') {
    classes.push(node.label.startsWith('veth') || node.label.startsWith('br-') || node.label === 'docker0' ? 'minor' : 'primary-iface')
  }
  return classes.filter(Boolean).join(' ')
}

function parentOf(node: Graph['nodes'][number], ids: Set<string>) {
  if (!node.parent_id || !ids.has(node.parent_id) || collapsed.has(node.parent_id)) return undefined
  return node.parent_id
}

function ordered(nodes: Graph['nodes'][number][], ids: Set<string>) {
  const pending = [...nodes]
  const done = new Set<string>()
  const result: Graph['nodes'][number][] = []
  let guard = 0
  while (pending.length && guard < 10) {
    guard += 1
    for (let index = pending.length - 1; index >= 0; index -= 1) {
      const node = pending[index]
      const parent = parentOf(node, ids)
      if (!parent || done.has(parent)) {
        result.push(node)
        done.add(node.id)
        pending.splice(index, 1)
      }
    }
  }
  return result.concat(pending)
}

function savedMode() {
  return props.positions.__layout?.x === 2
}

function place(node: Graph['nodes'][number]): Position {
  return memory.get(node.id) || (savedMode() ? props.positions[node.id] : undefined) || { x: 0, y: 0 }
}

function blockOf(node: NodeSingular) {
  const box = node.boundingBox({ includeLabels: true })
  const w = Math.max(box.w, 28)
  const h = Math.max(box.h, 28)
  return { id: node.id(), kind: String(node.data('kind') || ''), label: String(node.data('label') || ''), w: w + 20, h: h + 16 }
}

function moveBy(node: NodeSingular, dx: number, dy: number) {
  if (!dx && !dy) return
  const leaves = node.isParent() ? node.descendants().filter((item) => !item.isParent()) : cy!.collection().union(node)
  leaves.forEach((leaf) => {
    const position = leaf.position()
    leaf.position({ x: position.x + dx, y: position.y + dy })
  })
}

function moveTo(node: NodeSingular, x: number, y: number) {
  const box = node.boundingBox({ includeLabels: true })
  moveBy(node, x - (box.x1 + box.w / 2), y - (box.y1 + box.h / 2))
}

function siblingGroups() {
  const groups = new Map<string, NodeSingular[]>()
  cy?.nodes().forEach((node) => {
    const key = String(node.data('parent') || '')
    const list = groups.get(key) || []
    list.push(node)
    groups.set(key, list)
  })
  return [...groups.entries()].sort((a, b) => depth(b[0]) - depth(a[0]))
}

function depth(id: string) {
  let level = 0
  let current = id
  while (current && cy) {
    const node = cy.getElementById(current)
    if (node.empty()) break
    current = String(node.data('parent') || '')
    level += 1
  }
  return level
}

function packAll() {
  if (!cy) return
  for (const [, siblings] of siblingGroups()) {
    const points = pack(lanes(siblings.map((node) => blockOf(node))))
    siblings.forEach((node) => {
      const point = points.get(node.id())
      if (point) moveTo(node, point.x, point.y)
    })
  }
}

function restoreSaved() {
  cy?.nodes().forEach((node) => {
    if (node.isParent()) return
    const point = props.positions[node.id()] || memory.get(node.id())
    if (point) node.position(point)
  })
}

function marbles(nodes: NodeSingular[]): Marble[] {
  return nodes.map((node) => {
    const box = node.boundingBox({ includeLabels: true })
    return { id: node.id(), x: box.x1 + box.w / 2, y: box.y1 + box.h / 2, r: Math.max(box.w, box.h) / 2 + 18 }
  })
}

function relax(nodes: NodeSingular[], fixed: Set<string>, steps: number) {
  if (nodes.length < 2) return
  const start = new Map(marbles(nodes).map((item) => [item.id, item]))
  const done = separate([...start.values()], fixed, steps)
  for (const item of done) {
    const from = start.get(item.id)
    const node = cy?.getElementById(item.id)
    if (!from || !node || node.empty()) continue
    moveBy(node, item.x - from.x, item.y - from.y)
  }
}

function relaxAll(steps: number, fixed = new Set<string>()) {
  for (const [, siblings] of siblingGroups()) relax(siblings, fixed, steps)
}

function remember() {
  cy?.nodes().forEach((node) => {
    if (node.isParent()) return
    const position = node.position()
    memory.set(node.id(), { x: position.x, y: position.y })
  })
}

function overlap(next: string[]) {
  if (!arrangedIds.length || !next.length) return 0
  const known = new Set(arrangedIds)
  const shared = next.filter((id) => known.has(id)).length
  return shared / Math.max(arrangedIds.length, next.length)
}

function arrange(ids: string[]) {
  if (!cy || ids.length === 0) return
  const mode = savedMode() ? 'saved' : 'auto'
  const viewChanged = !arranged || overlap(ids) < 0.5
  const fresh = viewChanged || arrangedMode !== mode
  if (fresh) {
    if (mode === 'saved') restoreSaved()
    else packAll()
    relaxAll(mode === 'saved' ? 16 : 28)
    if (viewChanged) cy.fit(undefined, 56)
    arranged = true
    arrangedMode = mode
    arrangedIds = ids
    remember()
    return
  }
  const newcomers = cy.nodes().filter((node) => !node.isParent() && !memory.has(node.id()))
  if (newcomers.empty()) return
  newcomers.forEach((node, index) => {
    node.position({ x: 80 + index * 48, y: -120 })
  })
  relaxAll(14)
  remember()
}

function pulse(from: string, to: string) {
  const edge = cy?.edges().filter((item) => {
    if (item.data('kind') !== 'DIRECT') return false
    const source = String(item.data('source'))
    const target = String(item.data('target'))
    return (source === from && target === to) || (source === to && target === from)
  })
  const found = edge && edge.length ? edge[0] : undefined
  if (!found) return
  found.addClass('surge')
  surges.set(found.id(), performance.now() + 1400)
}

function flow() {
  if (!flowing || !cy) return
  flowFrame = (flowFrame + 1) % 64
  cy.edges('.dbncp').forEach((edge) => {
    const hot = (surges.get(edge.id()) ?? 0) > performance.now()
    if (!props.animations) {
      edge.removeClass('surge')
      edge.removeStyle('line-dash-offset')
      return
    }
    edge.style('line-dash-offset', hot ? -flowFrame * 2 : -flowFrame * 0.45)
    if (hot) edge.addClass('surge')
    else edge.removeClass('surge')
  })
  requestAnimationFrame(flow)
}

function pushAround(node: NodeSingular) {
  const fixed = new Set<string>([node.id()])
  let current: NodeSingular | undefined = node
  while (current && !current.empty()) {
    const parent = String(current.data('parent') || '')
    const siblings = cy!.nodes().filter((item) => String(item.data('parent') || '') === parent)
    relax([...siblings], fixed, 3)
    if (!parent) break
    fixed.add(parent)
    const container = cy!.getElementById(parent)
    current = container.empty() ? undefined : container
  }
  remember()
}

function sync() {
  if (!cy) return
  const nodes = visibleNodes()
  const ids = new Set(nodes.map((node) => node.id))
  const edges = props.graph.edges.filter((edge) => ids.has(edge.source) && ids.has(edge.target))
  cy.batch(() => {
    cy?.nodes().forEach((element) => {
      if (!ids.has(element.id())) element.remove()
    })
    cy?.edges().forEach((element) => {
      if (!edges.some((edge) => edge.id === element.id())) element.remove()
    })
    for (const node of ordered(nodes, ids)) {
      const parent = parentOf(node, ids)
      const previous = previousStatus.get(node.id)
      const flash = props.animations && previous !== undefined && previous !== node.status
      previousStatus.set(node.id, node.status)
      const data: ElementDefinition['data'] = { id: node.id, label: node.label, kind: node.kind, status: node.status, simulated: node.simulated ? 'yes' : 'no' }
      if (parent) data.parent = parent
      const existing = cy?.getElementById(node.id)
      if (!existing || existing.empty()) {
        cy?.add({ group: 'nodes', data, classes: classesOf(node, flash), position: place(node) })
      } else {
        if (existing.data('parent') !== (parent ?? undefined)) {
          const position = existing.position()
          existing.remove()
          cy?.add({ group: 'nodes', data, classes: classesOf(node, flash), position })
        } else {
          existing.data(data)
          existing.classes(classesOf(node, flash))
        }
      }
      if (flash) {
        const id = node.id
        window.setTimeout(() => cy?.getElementById(id).removeClass('just-changed'), 1200)
      }
    }
    for (const edge of edges) {
      const data = { id: edge.id, source: edge.source, target: edge.target, kind: edge.kind, label: edge.kind === 'DIRECT' ? 'dBNCP' : '' }
      const existing = cy?.getElementById(edge.id)
      const classes = edge.kind === 'DIRECT' ? 'dbncp' : ''
      if (!existing || existing.empty()) cy?.add({ group: 'edges', data, classes })
      else {
        existing.data(data)
        existing.classes(classes)
      }
    }
  })
  arrange(props.graph.nodes.map((node) => node.id).sort())
  highlight()
}

function readPositions() {
  const positions: Record<string, Position> = {}
  cy?.nodes().forEach((node) => {
    if (node.isParent() && node.children().length) return
    const position = node.position()
    positions[node.id()] = { x: position.x, y: position.y }
  })
  return positions
}

function highlight() {
  if (!cy) return
  cy.elements().removeClass('dim near path')
  const selected = props.selectedId ? cy.getElementById(props.selectedId) : cy.collection()
  if (selected.nonempty()) {
    cy.elements().addClass('dim')
    selected.closedNeighborhood().removeClass('dim').addClass('near')
    selected.removeClass('dim')
  }
  const picked = cy.nodes(':selected')
  if (picked.length === 2) {
    const path = cy.elements().dijkstra({ root: picked[0], directed: false }).pathTo(picked[1])
    path.removeClass('dim').addClass('path')
  }
}

function focus(id: string) {
  const element = cy?.getElementById(id)
  if (!element || element.empty()) return
  cy?.animate({ center: { eles: element }, zoom: Math.max(cy.zoom(), 1) }, { duration: props.animations ? 280 : 0 })
  element.select()
  emit('select', id)
}

onMounted(() => {
  if (!host.value) return
  cy = cytoscape({
    container: host.value,
    elements: [],
    style: stylesheet,
    wheelSensitivity: 1,
    minZoom: 0.08,
    maxZoom: 6,
    boxSelectionEnabled: true,
  })
  cy.on('tap', 'node', (event) => emit('select', event.target.id()))
  cy.on('tap', (event) => {
    if (event.target === cy) emit('select', '')
  })
  host.value.addEventListener('wheel', zoomWheel, { capture: true, passive: false })
  cy.on('dbltap', 'node', (event) => {
    const id = event.target.id() as string
    if (collapsed.has(id)) collapsed.delete(id)
    else collapsed.add(id)
    sync()
  })
  let draggedId = ''
  cy.on('drag', 'node', (event) => {
    draggedId = event.target.id() as string
    if (dragFrame) return
    dragFrame = requestAnimationFrame(() => {
      dragFrame = 0
      const node = cy?.getElementById(draggedId)
      if (node && !node.empty()) pushAround(node)
    })
  })
  cy.on('dragfree', 'node', () => {
    const node = draggedId ? cy?.getElementById(draggedId) : undefined
    if (node && !node.empty()) {
      pushAround(node)
      relaxAll(8, new Set([draggedId]))
    }
    remember()
    emit('move', readPositions())
  })
  sync()
  flowing = true
  requestAnimationFrame(flow)
  window.addEventListener('resize', resize)
})

function zoomWheel(event: WheelEvent) {
  if (!cy || !host.value) return
  event.preventDefault()
  event.stopPropagation()
  const raw = event.deltaMode === 1 ? event.deltaY * 100 : event.deltaY
  const notches = raw / 100
  const rect = host.value.getBoundingClientRect()
  const level = Math.min(cy.maxZoom(), Math.max(cy.minZoom(), cy.zoom() * Math.pow(1.35, -notches)))
  cy.zoom({ level, renderedPosition: { x: event.clientX - rect.left, y: event.clientY - rect.top } })
}

function resize() {
  cy?.resize()
}

watch(() => [props.graph, props.query, props.kinds, props.positions, props.animations], sync, { deep: true })
watch(() => props.selectedId, () => {
  if (!cy || !props.selectedId) {
    highlight()
    return
  }
  const element = cy.getElementById(props.selectedId)
  if (element.nonempty() && !element.selected()) element.select()
  highlight()
})

onBeforeUnmount(() => {
  flowing = false
  host.value?.removeEventListener('wheel', zoomWheel, { capture: true })
  window.removeEventListener('resize', resize)
  cy?.destroy()
  cy = null
})

defineExpose({ focus, pulse })
</script>

<template>
  <div ref="host" class="canvas" />
</template>
