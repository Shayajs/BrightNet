import { createApp } from 'vue'
import '@fontsource/outfit/400.css'
import '@fontsource/outfit/500.css'
import '@fontsource/outfit/600.css'
import '@fontsource/ibm-plex-mono/400.css'
import '@fontsource/ibm-plex-mono/500.css'
import App from './App.vue'
import './styles.css'

const theme = localStorage.getItem('brightnet-theme')
document.documentElement.dataset.theme = theme === 'light' ? 'light' : 'dark'
createApp(App).mount('#app')
