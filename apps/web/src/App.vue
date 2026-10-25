<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from './api'
import GraphCanvas from './components/GraphCanvas.vue'
import Admin from './components/Admin.vue'
import Home from './components/Home.vue'
import type { Download } from './api'
import { evidenceLabel, kindLabel, originLabel, statusLabel, views } from './labels'
import type { Activity, Graph, GraphNode, Position, Summary } from './types'

type Mode = 'loading' | 'setup' | 'login' | 'app'
const mode = ref<Mode>('loading')
const error = ref('')
const username = ref('')
const password = ref('')
const sessionUser = ref('')
const view = ref('network')
const focus = ref('')
const graph = ref<Graph | null>(null)
const summary = ref<Summary | null>(null)
const events = ref<Activity[]>([])
const positions = ref<Record<string, Position>>({})
const query = ref('')
const selectedId = ref('')
const draft = ref('')
const animations = ref(localStorage.getItem('brightnet-animations') !== 'off')
const theme = ref(document.documentElement.dataset.theme === 'light' ? 'light' : 'dark')
const kinds = ref<Record<string, boolean>>({})
const downloads = ref<Download[]>([])
const admin = ref(false)
const phone = ref<'map' | 'side' | 'events' | 'detail'>('map')
const canvas = ref<{ focus: (id: string) => void, pulse: (from: string, to: string) => void } | null>(null)
let source: EventSource | null = null
let saveTimer = 0

const selected = computed(() => graph.value?.nodes.find((node) => node.id === selectedId.value) ?? null)
const relations = computed(() => graph.value?.edges.filter((edge) => edge.source === selectedId.value || edge.target === selectedId.value) ?? [])
const inventory = computed(() => {
  const groups = new Map<string, GraphNode[]>()
  for (const node of graph.value?.nodes ?? []) {
    if (query.value && !node.label.toLowerCase().includes(query.value.toLowerCase())) continue
    const list = groups.get(node.kind) ?? []
    list.push(node)
    groups.set(node.kind, list)
  }
  return [...groups.entries()]
})

watch(selected, (node) => { draft.value = node?.label ?? '' })
watch(animations, (value) => localStorage.setItem('brightnet-animations', value ? 'on' : 'off'))
watch(graph, (value) => {
  if (!value) return
  for (const node of value.nodes) if (!(node.kind in kinds.value)) kinds.value[node.kind] = true
}, { deep: true })

watch(selectedId, (id) => {
  if (id && window.matchMedia('(max-width: 760px)').matches) phone.value = 'detail'
})

onMounted(async () => {
  window.brightnetBack = brightnetBack
  downloads.value = (await api.downloads().catch(() => ({ downloads: [] }))).downloads
  try {
    const status = await api.setupStatus()
    if (status.needs_setup) {
      mode.value = 'setup'
      return
    }
    const me = await api.me()
    sessionUser.value = me.username
    mode.value = 'app'
    await refresh()
    connect()
  } catch {
    mode.value = 'login'
  }
})

onBeforeUnmount(() => {
  source?.close()
  delete window.brightnetBack
})

async function submitAuth(kind: 'login' | 'setup') {
  error.value = ''
  try {
    if (kind === 'setup') await api.setup(username.value, password.value)
    await api.login(username.value, password.value)
    sessionUser.value = username.value
    mode.value = 'app'
    await refresh()
    connect()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'accès refusé'
  }
}

async function refresh() {
  const [nextGraph, nextSummary, nextEvents, nextLayout] = await Promise.all([
    api.graph(view.value, focus.value),
    api.summary(),
    api.events(),
    api.layout(view.value).catch(() => ({})),
  ])
  graph.value = nextGraph
  summary.value = nextSummary
  events.value = nextEvents.events
  positions.value = nextLayout
}

async function changeView(next: string) {
  view.value = next
  focus.value = ''
  selectedId.value = ''
  await refresh()
}

function connect() {
  source?.close()
  source = new EventSource('/api/v1/events/stream')
  let refreshTimer = 0
  source.onmessage = (message) => {
    const payload = JSON.parse(message.data) as { type: string, id?: string, label?: string, from?: string, to?: string, event?: Activity }
    if (payload.type === 'activity' && payload.event) events.value = [payload.event, ...events.value].slice(0, 80)
    if (payload.type === 'current' && payload.from && payload.to) canvas.value?.pulse(payload.from, payload.to)
    if (payload.type === 'entity.updated' && payload.id && graph.value) {
      const node = graph.value.nodes.find((item) => item.id === payload.id)
      if (node && payload.label) node.label = payload.label
    }
    if (payload.type === 'graph.changed') {
      window.clearTimeout(refreshTimer)
      refreshTimer = window.setTimeout(() => { void api.graph(view.value, focus.value).then((value) => { graph.value = value }) }, 120)
    }
  }
}

function onMove(next: Record<string, Position>) {
  positions.value = { ...next, __layout: { x: 2, y: 0 } }
  window.clearTimeout(saveTimer)
  saveTimer = window.setTimeout(() => { void api.saveLayout(view.value, positions.value).catch(() => undefined) }, 400)
}

async function rename() {
  if (!selected.value) return
  await api.rename(selected.value.id, draft.value)
  await refresh()
}

async function loadFixture() {
  await api.loadFixture()
  await refresh()
}

async function logout() {
  source?.close()
  await api.logout()
  mode.value = 'login'
}

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  document.documentElement.dataset.theme = theme.value
  localStorage.setItem('brightnet-theme', theme.value)
}

function choose(id: string) {
  selectedId.value = id
  const node = graph.value?.nodes.find((item) => item.id === id)
  if (view.value === 'network' && node && (node.kind === 'host' || node.kind === 'device' || node.kind === 'router') && focus.value !== id) {
    focus.value = id
    void refresh()
  }
  canvas.value?.focus(id)
}

function clearFocus() {
  focus.value = ''
  selectedId.value = ''
  void refresh()
}

function androidBridge() {
  return window.BrightNetAndroid
}

function brightnetBack() {
  if (mode.value !== 'app') return false
  if (window.matchMedia('(max-width: 760px)').matches && phone.value !== 'map') {
    phone.value = 'map'
    return true
  }
  if (admin.value) {
    admin.value = false
    return true
  }
  if (focus.value) {
    clearFocus()
    return true
  }
  return false
}

function propertyEntries(node: GraphNode) {
  return Object.entries(node.properties).filter(([, value]) => value !== null && value !== '')
}

function formatValue(value: unknown) {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return String(value)
  return JSON.stringify(value)
}

function otherEnd(edge: { source: string, target: string }) {
  const id = edge.source === selectedId.value ? edge.target : edge.source
  return graph.value?.nodes.find((node) => node.id === id)?.label ?? id
}

function clock(value?: string) {
  if (!value) return ''
  return new Intl.DateTimeFormat('fr-FR', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}

function eventLabel(kind: string) {
  const labels: Record<string, string> = { inventory: 'Inventaire', appear: 'Apparition', status: 'État', 'docker.event': 'Docker' }
  return labels[kind] || kind
}
</script>

<template>
  <main v-if="mode === 'loading'" class="gate">BrightNet</main>
  <Home v-else-if="mode === 'setup' || mode === 'login'" v-model:username="username" v-model:password="password" :mode="mode" :error="error" :downloads="downloads" @submit="submitAuth(mode === 'setup' ? 'setup' : 'login')" @theme="toggleTheme" />
  <div v-else class="shell" :class="{ 'admin-on': admin }" :data-phone="phone">
    <header class="top">
      <div class="brand">BrightNet <span>{{ sessionUser }}</span></div>
      <p class="origin" :data-origin="graph?.data_origin || summary?.data_origin">{{ originLabel[graph?.data_origin || summary?.data_origin || 'live'] }}</p>
      <p class="agents">
        <template v-if="summary?.agents.length">
          <span v-for="agent in summary.agents" :key="agent.id" :data-status="agent.status">{{ agent.hostname || 'agent' }} · {{ statusLabel[agent.status] || agent.status }}</span>
        </template>
        <span v-else>Aucun agent appairé</span>
      </p>
      <div class="top-actions">
        <button type="button" class="ghost" :class="{ on: admin }" @click="admin = !admin">Administration</button>
        <button type="button" class="ghost" @click="animations = !animations">{{ animations ? 'Animations' : 'Animations coupées' }}</button>
        <button type="button" class="ghost" @click="toggleTheme">{{ theme === 'dark' ? 'Clair' : 'Sombre' }}</button>
        <button v-if="androidBridge()" type="button" class="ghost" @click="androidBridge()?.changeCore()">Core</button>
        <button type="button" class="ghost" @click="logout">Quitter</button>
      </div>
    </header>
    <aside v-if="!admin" class="side">
      <label class="search">Recherche <input v-model="query" placeholder="nom, type" /></label>
      <div class="views">
        <button v-for="item in views" :key="item.id" type="button" :class="{ on: view === item.id }" @click="changeView(item.id)">
          {{ item.label }}<small v-if="!item.fed">vide</small>
        </button>
      </div>
      <p v-if="view === 'network'" class="hint">Le réseau d’abord. Cliquez un appareil pour l’ouvrir. Un nœud déplacé écarte les autres.</p>
      <button v-if="focus" type="button" class="ghost" @click="clearFocus">Revenir au réseau</button>
      <div class="filters">
        <h2>Types</h2>
        <label v-for="(enabled, kind) in kinds" :key="kind" class="slide">
          <input v-model="kinds[kind]" type="checkbox" />
          <i aria-hidden="true"></i>
          <span>{{ kindLabel[kind] || kind }}</span>
        </label>
      </div>
      <div class="inventory">
        <section v-for="[kind, nodes] in inventory" :key="kind">
          <h2>{{ kindLabel[kind] || kind }}</h2>
          <button v-for="node in nodes" :key="node.id" type="button" :class="{ on: node.id === selectedId, simulated: node.simulated }" @click="choose(node.id)">
            <i :data-status="node.status" />{{ node.label }}
          </button>
        </section>
      </div>
      <div class="tools">
        <button type="button" @click="loadFixture">Charger une démonstration</button>
      </div>
    </aside>
    <section class="stage">
      <Admin v-if="admin" />
      <GraphCanvas v-else-if="graph" ref="canvas" :graph="graph" :positions="positions" :query="query" :kinds="kinds" :animations="animations" :selected-id="selectedId" @select="selectedId = $event" @move="onMove" />
      <p v-if="!admin && graph?.notice" class="notice">{{ graph.notice }}</p>
    </section>
    <aside v-if="!admin" class="detail">
      <template v-if="selected">
        <p class="eyebrow">{{ kindLabel[selected.kind] || selected.kind }}</p>
        <h2>{{ selected.label }}</h2>
        <p class="state" :data-status="selected.status">{{ statusLabel[selected.status] || selected.status }}</p>
        <p v-if="selected.simulated" class="banner">Entité simulée. Elle ne décrit pas un équipement observé.</p>
        <form @submit.prevent="rename">
          <label>Nom affiché <input v-model="draft" maxlength="80" /></label>
          <button type="submit">Enregistrer</button>
        </form>
        <dl>
          <div v-for="[key, value] in propertyEntries(selected)" :key="key"><dt>{{ key }}</dt><dd>{{ formatValue(value) }}</dd></div>
        </dl>
        <h3>Relations</h3>
        <ul>
          <li v-for="edge in relations" :key="edge.id">
            <strong>{{ edge.kind }}</strong> {{ otherEnd(edge) }}
            <em>{{ evidenceLabel[edge.evidence] || edge.evidence }}</em>
          </li>
        </ul>
        <p class="key">{{ selected.technical_key }}</p>
      </template>
      <p v-else class="empty">Sélectionnez un nœud pour voir son état, ses preuves et ses relations.</p>
    </aside>
    <footer class="events">
      <h2>Événements</h2>
      <ol>
        <li v-for="event in events" :key="event.id" :data-kind="event.kind" :class="{ simulated: event.simulated }">
          <time>{{ clock(event.observed_at) }}</time>
          <span>{{ eventLabel(event.kind) }}</span>
          <p>{{ event.message }}</p>
        </li>
      </ol>
    </footer>
    <nav v-if="!admin" class="phone-nav" aria-label="Sections">
      <button type="button" :class="{ on: phone === 'map' }" @click="phone = 'map'">Carte</button>
      <button type="button" :class="{ on: phone === 'side' }" @click="phone = 'side'">Réseau</button>
      <button type="button" :class="{ on: phone === 'events' }" @click="phone = 'events'">Événements</button>
      <button type="button" :class="{ on: phone === 'detail' }" @click="phone = 'detail'">Fiche</button>
    </nav>
  </div>
</template>
