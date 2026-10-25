<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { api, type AdminView } from '../api'
import { originLabel, statusLabel } from '../labels'

const info = ref<AdminView | null>(null)
const error = ref('')
let timer = 0

async function load() {
  try {
    info.value = await api.admin()
    error.value = ''
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'administration indisponible'
  }
}

async function forget(id: string, name: string) {
  if (!window.confirm(`Oublier ${name} ? Un Node encore allumé se réinscrit tout seul.`)) return
  await api.forgetAgent(id)
  await load()
}

function clock(value?: string) {
  if (!value) return 'jamais'
  return new Intl.DateTimeFormat('fr-FR', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}

onMounted(() => {
  void load()
  timer = window.setInterval(() => { void load() }, 5000)
})
onBeforeUnmount(() => window.clearInterval(timer))
</script>

<template>
  <section class="admin">
    <p class="eyebrow">Core Admin</p>
    <h2>Administration</h2>
    <p v-if="error" class="error">{{ error }}</p>
    <div v-if="info" class="admin-grid">
      <article>
        <h3>Core</h3>
        <dl>
          <div><dt>Interface</dt><dd>{{ info.ui || '—' }}</dd></div>
          <div><dt>BNCP</dt><dd>{{ info.bncp || '—' }}</dd></div>
          <div><dt>Entités</dt><dd>{{ info.entities }}</dd></div>
          <div><dt>Origine</dt><dd>{{ originLabel[info.data_origin] || info.data_origin }}</dd></div>
        </dl>
      </article>
      <article>
        <h3>Node</h3>
        <p v-if="!info.agents.length" class="empty">Aucun Node inscrit.</p>
        <ul v-else>
          <li v-for="agent in info.agents" :key="agent.id">
            <strong>{{ agent.hostname || 'node' }}</strong>
            <em>{{ statusLabel[agent.status] || agent.status }} · vu {{ clock(agent.last_seen_at) }} · {{ agent.version }}</em>
            <button type="button" class="ghost" @click="forget(agent.id, agent.hostname || 'ce node')">Oublier</button>
          </li>
        </ul>
      </article>
    </div>
  </section>
</template>
