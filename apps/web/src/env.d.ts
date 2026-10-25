/// <reference types="vite/client" />

interface BrightNetAndroidBridge {
  changeCore: () => void
}

interface Window {
  BrightNetAndroid?: BrightNetAndroidBridge
  brightnetBack?: () => boolean
}

declare module 'cytoscape-fcose' {
  import type cytoscape from 'cytoscape'
  const extension: cytoscape.Ext
  export default extension
}
