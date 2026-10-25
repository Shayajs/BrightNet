<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from './api'
import GraphCanvas from './components/GraphCanvas.vue'
import { evidenceLabel, kindLabel, originLabel, statusLabel, views } from './labels'
import type { Activity, Graph, GraphNode, Position, Summary } from './types'

type Mode = 'loading' | 'setup' | 'login' | 'app'
const mode = ref<Mode>('loading')
const error = ref('')
const username = ref('')
const password = ref('')
const sessionUser = ref('')
const view = ref('docker')
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
const pairing = ref('')
const canvas = ref<{ focus: (id: string) => void } | null>(null)
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

onMounted(async () => {
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

onBeforeUnmount(() => source?.close())

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
    api.graph(view.value),
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
  selectedId.value = ''
  await refresh()
}

function connect() {
  source?.close()
  source = new EventSource('/api/v1/events/stream')
  source.onmessage = (message) => {
    const payload = JSON.parse(message.data) as { type: string, event?: Activity }
    if (payload.type === 'activity' && payload.event) events.value = [payload.event, ...events.value].slice(0, 80)
    if (payload.type === 'graph.changed') void api.graph(view.value).then((value) => { graph.value = value })
  }
}

function onMove(next: Record<string, Position>) {
  positions.value = { ...positions.value, ...next }
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

async function createPairing() {
  const created = await api.pairingCode()
  pairing.value = created.pairing_code
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
  canvas.value?.focus(id)
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
</script>

<template>
  <main v-if="mode === 'loading'" class="gate">BrightNet</main>
  <main v-else-if="mode === 'setup' || mode === 'login'" class="gate">
    <form class="card" @submit.prevent="submitAuth(mode === 'setup' ? 'setup' : 'login')">
      <p class="eyebrow">Infrastructure locale</p>
      <h1>BrightNet</h1>
      <p class="lede">{{ mode === 'setup' ? 'Créez le compte administrateur de cette machine.' : 'Le graphe reste sur cette machine.' }}</p>
      <label>Identifiant <input v-model="username" autocomplete="username" required /></label>
      <label>Mot de passe <input v-model="password" type="password" autocomplete="current-password" required /></label>
      <p v-if="error" class="error">{{ error }}</p>
      <button type="submit">{{ mode === 'setup' ? 'Créer l’accès' : 'Entrer' }}</button>
    </form>
  </main>
  <div v-else class="shell">
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
        <button type="button" class="ghost" @click="animations = !animations">{{ animations ? 'Animations' : 'Animations coupées' }}</button>
        <button type="button" class="ghost" @click="toggleTheme">{{ theme === 'dark' ? 'Clair' : 'Sombre' }}</button>
        <button type="button" class="ghost" @click="logout">Quitter</button>
      </div>
    </header>
    <aside class="side">
      <label class="search">Recherche <input v-model="query" placeholder="nom, type" /></label>
      <div class="views">
        <button v-for="item in views" :key="item.id" type="button" :class="{ on: view === item.id }" @click="changeView(item.id)">
          {{ item.label }}<small v-if="!item.fed">vide</small>
        </button>
      </div>
      <div class="filters">
        <label v-for="(enabled, kind) in kinds" :key="kind"><input v-model="kinds[kind]" type="checkbox" /> {{ kindLabel[kind] || kind }}</label>
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
        <button type="button" class="ghost" @click="createPairing">Nouveau code d’appairage</button>
        <p v-if="pairing" class="code">{{ pairing }}</p>
      </div>
    </aside>
    <section class="stage">
      <GraphCanvas v-if="graph" ref="canvas" :graph="graph" :positions="positions" :query="query" :kinds="kinds" :animations="animations" :selected-id="selectedId" @select="selectedId = $event" @move="onMove" />
      <p v-if="graph?.notice" class="notice">{{ graph.notice }}</p>
    </section>
    <aside class="detail">
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
        <li v-for="event in events" :key="event.id" :class="{ simulated: event.simulated }">
          <time>{{ clock(event.observed_at) }}</time>{{ event.message }}
        </li>
      </ol>
    </footer>
  </div>
</template>
