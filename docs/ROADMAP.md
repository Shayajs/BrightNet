# Feuille de route

Chaque phase est terminée seulement lorsque ses critères ont été vérifiés. Une case non cochée dans `docs/PROGRESS.md` n'est pas une fonctionnalité livrée.

## Phase 0 — Audit et conception

Documentation initiale, décisions, modèle, contrat et constat d'environnement. Le Raspberry Pi n'est pas contacté.

Critères :

- Les documents listés dans le README existent et séparent les faits, les inconnues et les choix.
- Aucune action n'a été menée sur `192.168.1.3`.

## Phase 1 — MVP local

Core, SQLite, interface, graphe, agent Linux, Docker en lecture seule, authentification, SSE, fixtures signalées, Compose et tests.

Critères :

- Compose démarre Core, interface et agent sans modifier les conteneurs déjà présents.
- L'interface exige une session. L'ingestion exige un agent appairé.
- Le graphe Docker montre le moteur réel, ses conteneurs, réseaux et volumes, sans variables d'environnement.
- Le conteneur jetable `brightnet-signal` apparaît, passe à l'état arrêté, puis redémarre dans l'interface.
- Les fixtures sont visuellement distinctes des observations.
- `go test ./...` et le contrôle de types de l'interface passent.
- Le service Core n'a pas le socket Docker.

## Phase 2 — Technitium et découverte

Connecteur en lecture seule, après identification de la version installée et du rôle de Pi-hole. Statistiques, clients, journaux et baux DHCP si ce rôle est confirmé. Corrélation avec une machine seulement si une interface de cette machine a porté l'adresse dans la fenêtre observée. Aucune écriture DNS.

Cette phase n'attend pas Starlink.

## Connecteur Starlink — facultatif, après la phase 2

Antenne et routeur en gRPC local, lecture seule, désactivé par défaut. Courbes dans `metrics`, événements dans la chronologie, nœuds antenne et routeur dans le graphe. Pas de position GPS. Pas de graphe de communications tant que les API ne fournissent que des compteurs agrégés. Le constat du 25 octobre 2026 est dans `docs/ENVIRONMENT.md` ; le code n'est pas encore écrit.

## Phase 3 — Multiplateforme

Agent Windows, particularités WSL2 et Docker Desktop, agent macOS documenté selon les permissions Apple, découverte passive des téléphones et objets dans les plages explicitement autorisées. La découverte sans agent reste distincte de l'installation d'un agent.

## Phase 4 — Observation avancée

Connexions réellement visibles par l'agent, association processus lorsque le système l'autorise, métriques ciblées, dépendances, alertes et navigation temporelle. eBPF, ETW, NetFlow ou SNMP seulement si l'équipement et les droits le permettent. Un agent sur un PC ne représente pas le trafic de la maison.

## Phase 5 — Domotique

Lecture Home Assistant et MQTT. Les commandes attendent un modèle d'autorisation propre.

## Phase 6 — Centre de contrôle

Actions sur les services, application de bureau, notifications et assistant local. Ces travaux ne commencent pas avant la stabilisation des phases précédentes.

## Déploiement

Un plan séparé, précédé de l'audit en lecture seule décrit dans `docs/ENVIRONMENT.md`, est obligatoire avant toute installation sur le Raspberry Pi. Si la marge mémoire est insuffisante, le Pi ne reçoit pas le Core.
