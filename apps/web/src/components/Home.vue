<script setup lang="ts">
import { computed } from 'vue'
import type { Download } from '../api'

const props = defineProps<{
  mode: 'setup' | 'login'
  error: string
  downloads: Download[]
}>()

const username = defineModel<string>('username', { required: true })
const password = defineModel<string>('password', { required: true })
const emit = defineEmits<{ submit: [], theme: [] }>()

const fallback: Download[] = [
  { id: 'windows-amd64', os: 'windows', arch: 'amd64', title: 'Windows', file: 'brightnet-node-windows-amd64.exe', note: '', available: false },
  { id: 'linux-amd64', os: 'linux', arch: 'amd64', title: 'Linux', file: 'brightnet-node-linux-amd64', note: '', available: false },
  { id: 'linux-arm64', os: 'linux', arch: 'arm64', title: 'Linux', file: 'brightnet-node-linux-arm64', note: '', available: false },
]

const windows = computed(() => (props.downloads.length ? props.downloads : fallback).filter((item) => item.os === 'windows'))
const linux = computed(() => (props.downloads.length ? props.downloads : fallback).filter((item) => item.os === 'linux'))
</script>

<template>
  <main class="home">
    <header class="home-top">
      <p class="eyebrow">Réseau local</p>
      <button type="button" class="ghost" @click="emit('theme')">Thème</button>
    </header>
    <h1>BrightNet</h1>
    <p class="promise">Le Node envoie son inventaire au Core. S’il n’y en a qu’un sur le réseau, il le prend. S’il y en a plusieurs, il les propose.</p>
    <section class="clients">
      <article class="client">
        <p class="os">WINDOWS</p>
        <h2>Exécutable</h2>
        <p>Ligne de commande, compilée en Go. Une fenêtre pourra s’y ajouter, ou le programme rester en CLI.</p>
        <div class="grow">
          <a v-for="item in windows" :key="item.id" :href="item.available ? `/api/v1/downloads/${item.file}` : undefined" :aria-disabled="!item.available">{{ item.arch }} <span>{{ item.available ? 'Télécharger' : 'Absent' }}</span></a>
        </div>
      </article>
      <article class="client">
        <p class="os">LINUX</p>
        <h2>Exécutable</h2>
        <p>Le même Node, pour un PC ou une machine ARM.</p>
        <div class="grow">
          <a v-for="item in linux" :key="item.id" :href="item.available ? `/api/v1/downloads/${item.file}` : undefined" :aria-disabled="!item.available">{{ item.arch }} <span>{{ item.available ? 'Télécharger' : 'Absent' }}</span></a>
        </div>
      </article>
      <article class="client web">
        <p class="os">WEB</p>
        <h2>Cette page</h2>
        <p>Le navigateur ne parle pas BNCP. Il montre ce que les Node ont déjà envoyé, et ne collecte pas cette machine.</p>
        <form class="grow" @submit.prevent="emit('submit')">
          <label>Identifiant <input v-model="username" autocomplete="username" required /></label>
          <label>Mot de passe <input v-model="password" type="password" autocomplete="current-password" required /></label>
          <p v-if="error" class="error">{{ error }}</p>
          <button type="submit">{{ mode === 'setup' ? 'Créer l’accès' : 'Entrer' }}</button>
        </form>
      </article>
    </section>
  </main>
</template>
