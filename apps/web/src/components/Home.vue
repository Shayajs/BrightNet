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
  { id: 'bnnp-windows', os: 'windows', arch: 'amd64', title: 'BNNP', file: 'brightnet-bnnp-windows-amd64.exe', note: '', available: false },
  { id: 'bnca-windows', os: 'windows', arch: 'amd64', title: 'BNCA', file: 'brightnet-bnca-windows-amd64.exe', note: '', available: false },
  { id: 'linux-amd64', os: 'linux', arch: 'amd64', title: 'Linux', file: 'brightnet-node-linux-amd64', note: '', available: false },
  { id: 'linux-arm64', os: 'linux', arch: 'arm64', title: 'Linux', file: 'brightnet-node-linux-arm64', note: '', available: false },
  { id: 'android', os: 'android', arch: 'universal', title: 'Android', file: 'brightnet-android.apk', note: '', available: false },
]

const listed = computed(() => (props.downloads.length ? props.downloads : fallback))
const windows = computed(() => listed.value.filter((item) => item.os === 'windows'))
const linux = computed(() => listed.value.filter((item) => item.os === 'linux'))
const android = computed(() => listed.value.filter((item) => item.os === 'android'))
const androidApp = typeof window.BrightNetAndroid?.changeCore === 'function'

function changeCore() {
  window.BrightNetAndroid?.changeCore()
}
</script>

<template>
  <main class="home">
    <header class="home-top">
      <p class="eyebrow">Réseau local</p>
      <span class="home-actions">
        <button v-if="androidApp" type="button" class="ghost" @click="changeCore">Core</button>
        <button type="button" class="ghost" @click="emit('theme')">Thème</button>
      </span>
    </header>
    <h1>BrightNet</h1>
    <p class="promise">Le Node envoie son inventaire au Core. S’il n’y en a qu’un sur le réseau, il le prend. S’il y en a plusieurs, il les propose.</p>
    <section class="clients">
      <article class="client">
        <p class="os">WINDOWS</p>
        <h2>Deux applications</h2>
        <p>BNNP règle le Node de cette machine. BNCA gère le Core.</p>
        <div class="grow">
          <a v-for="item in windows" :key="item.id" :href="item.available ? `/api/v1/downloads/${item.file}` : undefined" :aria-disabled="!item.available">
            <span class="dl">
              <svg v-if="item.id.startsWith('bnnp')" class="hero" viewBox="0 0 22 22" aria-hidden="true">
                <rect x="1" y="1" width="20" height="20" rx="6" fill="#17200a" stroke="#d6f25c" />
                <circle cx="7" cy="11" r="2" fill="#d6f25c" />
                <circle cx="15" cy="7.5" r="2" fill="#d6f25c" />
                <circle cx="15" cy="14.5" r="2" fill="#d6f25c" />
                <path d="M9 11h4M9 10.2 13.2 8.2M9 11.8 13.2 13.8" stroke="#d6f25c" stroke-width="1.2" fill="none" />
              </svg>
              <svg v-else class="hero" viewBox="0 0 22 22" aria-hidden="true">
                <rect x="1" y="1" width="20" height="20" rx="6" fill="#17200a" stroke="#d6f25c" />
                <rect x="7" y="7" width="8" height="8" rx="2" fill="#d6f25c" />
                <circle cx="11" cy="11" r="6.5" fill="none" stroke="#d6f25c" stroke-opacity="0.55" />
              </svg>
              {{ item.title }}
            </span>
            <span>{{ item.available ? 'Télécharger' : 'Absent' }}</span>
          </a>
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
      <article class="client">
        <p class="os">ANDROID</p>
        <h2>L’application</h2>
        <p>La même carte, en plein écran. L’APK se met à jour en le réinstallant.</p>
        <div class="grow">
          <a v-for="item in android" :key="item.id" :href="item.available ? `/api/v1/downloads/${item.file}` : undefined" :aria-disabled="!item.available">
            <span class="dl">
              <svg class="hero" viewBox="0 0 22 22" aria-hidden="true">
                <rect x="1" y="1" width="20" height="20" rx="6" fill="#17200a" stroke="#d6f25c" />
                <rect x="7.2" y="3.6" width="7.6" height="14.8" rx="1.6" fill="none" stroke="#d6f25c" />
                <rect x="8.6" y="5.4" width="4.8" height="9.2" rx="0.6" fill="#d6f25c" />
              </svg>
              {{ item.title }}
            </span>
            <span>{{ item.available ? 'Télécharger' : 'Absent' }}</span>
          </a>
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
