# Progression

Relevé du 25 octobre 2026, après la vérification du MVP local. Une ligne « vérifié » correspond à un test réellement exécuté.

## Réalisé

- Documentation de phase 0 : vision, architecture, modèle, sécurité, intégrations, décisions, environnement, feuille de route.
- Dépôt Git initialisé, sans commit. Toolchain Go officiel 1.26.5 dans `~/.local/go`.
- Core, agent, contrat OpenAPI, schéma SQLite, interface Vue/Cytoscape, Compose de développement et workflow CI.

## Fonctionne réellement

Vérifié dans cette session :

- `go test ./...` : succès (`httpserver`, `ingest`, `dockerclient`).
- `npm run typecheck` dans `apps/web` : succès.
- `docker compose -f deploy/compose/docker-compose.dev.yml up -d --build` : Core sain, interface sur `127.0.0.1:5173`, agent en ligne (`SHAYA-DEV`).
- Le Core ne monte que le volume `/data`. Le socket Docker est monté en lecture seule sur l’agent.
- Connexion locale `admin` / `brightnet-dev`. Badge **Observations réelles**. Vues DNS, Flux et Applications signalées vides.
- Graphe Docker réel : 140 nœuds (1 hôte, 1 moteur, 70 conteneurs, 25 réseaux, 32 volumes, 11 services). Les ports publiés sont regroupés (un service par conteneur).
- Recherche du graphe JSON : aucune occurrence de `PASSWORD`, `MYSQL_ROOT`, `SECRET`, `Env` ni `/var/lib/docker`.
- Cycle `brightnet-signal` (`nginx:alpine`, sans port publié) observé dans le navigateur, sans rechargement : apparition, état **en cours**, image `nginx:alpine`, puis **arrêté** après `docker stop`, puis **en cours** après `docker start`. Les autres conteneurs n’ont pas été arrêtés.
- Le conteneur de test a ensuite été supprimé. Le nœud est passé à l’état absent.
- Les événements `exec_*` et `health_status` ne sont plus affichés.

## Starlink — étudié, pas branché

Vérifié le 25 octobre 2026, en lecture seule : l'antenne `192.168.100.1:9200` (`rev4_panda_prod2`) et le routeur `192.168.1.1:9000` (`v5`) répondent. `get_diagnostics`, `get_status` et `get_history` renvoient `grpc-status: 0`. Le routeur annonce `192.168.1.1/24` et des clients Wi-Fi, donc il n'est pas en bypass sur ce LAN. Aucun appel d'écriture. `get_location` n'a pas été appelé. Le Pi n'a pas été contacté. Le collecteur n'est pas dans le binaire. Technitium reste le prochain incrément.

## Problèmes connus

- Le Raspberry Pi n’a pas été contacté. Modèle, RAM, OS et rôles de Pi-hole et Technitium restent inconnus. Le premier résolveur vu par WSL est `192.168.1.3` ; on ne sait pas s’il fait aussi le DHCP.
- La route par défaut de Windows n’a pas pu être lue depuis WSL. Le routeur Starlink sert `192.168.1.1/24`, sans preuve que Windows l’utilise comme passerelle.
- L’adresse IPv4 WAN Starlink, CGNAT ou publique, n’apparaît pas dans les diagnostics lus.
- L’agent Compose tourne dans le netns du conteneur (`observation_scope` agent). Les vues physique (2 nœuds) et logique (4 nœuds) décrivent ce netns, plus le nom d’hôte et le `machine-id` montés depuis l’hôte. Les interfaces WSL complètes ne sont pas vues.
- Pas de statistiques CPU, pas de TLS, pas de scan du LAN. L’écoute publiée reste `127.0.0.1`.
- Le graphe complet est dense. Le cadrage initial tient dans la vue ; un filtre ne recadre pas automatiquement.
- L’inventaire latéral est long : l’élément recherché peut demander un défilement avant le clic.
- Le code d’appairage `brightnet-dev-pair` est réutilisable, uniquement pour ce Compose local.

## BNCP — codé, testé localement

Vérifié par `go test ./...` après l'ajout du protocole : trames trop courtes, mauvaise version et corps CBOR ; deux Node avec le même `machine-id` produisent un hôte et conservent chacun son interface. L'API JSON d'ingestion reste en place. Les ports `1717` et `1718` sont prévus sur `127.0.0.1`. Le Pi n'a pas été contacté. L'image Compose qui embarque l'interface n'a pas été reconstruite dans cette étape.

## Prochains travaux recommandés

1. Reconstruire l'image Compose pour servir l'interface sur `1717`, puis vérifier le Node en BNCP.
2. Phase 2 Technitium, après lecture de la version installée. Starlink ne la précède pas.
3. Audit en lecture seule du Raspberry Pi, seulement après une demande explicite. Il dira si le Pi est seulement résolveur ou aussi DHCP. Le Core n'y est pas copié avant ça.
4. Connecteur Starlink en lecture seule, désactivé par défaut, selon `docs/INTEGRATIONS.md`.
5. Lancer le Node sur l’hôte WSL si les interfaces réelles doivent apparaître.
