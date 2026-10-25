export const kindLabel: Record<string, string> = {
  host: 'Hôte',
  network_interface: 'Interface',
  address: 'Adresse',
  router: 'Routeur',
  device: 'Appareil',
  network: 'Réseau',
  docker_engine: 'Moteur Docker',
  docker_container: 'Conteneur',
  docker_network: 'Réseau Docker',
  docker_volume: 'Volume',
  service: 'Service',
}

export const statusLabel: Record<string, string> = {
  online: 'en ligne',
  offline: 'hors ligne',
  running: 'en cours',
  exited: 'arrêté',
  absent: 'disparu',
  paused: 'en pause',
  present: 'présent',
  created: 'créé',
  restarting: 'redémarrage',
  dead: 'en échec',
  unknown: 'inconnu',
}

export const evidenceLabel: Record<string, string> = {
  observed: 'observée',
  configured: 'configurée',
  declared: 'déclarée',
  inferred: 'déduite',
}

export const originLabel: Record<string, string> = {
  live: 'Observations réelles',
  mixed: 'Données mixtes',
  fixture: 'Simulation',
}

export const views = [
  { id: 'network', label: 'Simple', fed: true },
  { id: 'complex', label: 'Complet', fed: true },
  { id: 'docker', label: 'Docker', fed: true },
  { id: 'physical', label: 'Physique', fed: true },
  { id: 'logical', label: 'Logique', fed: true },
  { id: 'dns', label: 'DNS', fed: false },
  { id: 'communications', label: 'Flux', fed: false },
  { id: 'application', label: 'Applications', fed: false },
]
