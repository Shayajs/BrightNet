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

- Le Raspberry Pi 3 B+ porte le Core. Technitium écoute sur 53 et 5380 ; CouchDB sur 5984. Aucun Pi-hole. On n'a pas lu s'il fait le DHCP.
- La route par défaut de Windows n’a pas pu être lue depuis WSL. Le routeur Starlink sert `192.168.1.1/24`, sans preuve que Windows l’utilise comme passerelle.
- L’adresse IPv4 WAN Starlink, CGNAT ou publique, n’apparaît pas dans les diagnostics lus.
- L’agent Compose tourne dans le netns du conteneur (`observation_scope` agent). Les vues physique (2 nœuds) et logique (4 nœuds) décrivent ce netns, plus le nom d’hôte et le `machine-id` montés depuis l’hôte. Les interfaces WSL complètes ne sont pas vues.
- Pas de statistiques CPU, pas de TLS. Sur le Pi, `1717` est en HTTP et lié à `192.168.1.3` ; `1718` reste sur `127.0.0.1`. Pas de scan du LAN.
- Le graphe complet est dense. Le cadrage initial tient dans la vue ; un filtre ne recadre pas automatiquement.
- L’inventaire latéral est long : l’élément recherché peut demander un défilement avant le clic.
- Le code d’appairage `brightnet-dev-pair` est réutilisable, uniquement pour ce Compose local.

## Pi — installé le 25 octobre 2026

`shayajs@192.168.1.3` a refusé la clé. Le compte `shaya` a permis l'audit puis l'installation. Technitium et CouchDB n'ont pas été arrêtés.

Vérifié après démarrage : Core sain, interface française sur `http://192.168.1.3:1717`, Node enrollé en BNCP vers `192.168.1.3:1718`. La vue Simple montre le réseau, la passerelle et les appareils vus par mDNS, par la table des voisins, ou par un bail DHCP. dBNCP relie deux Node connectés au Core. Le mot de passe n'est pas celui de développement ; il est seulement dans `~/brightnet/.env` sur le Pi.

L'interface est en HTTP, liée à `192.168.1.3`. BNCP écoute aussi `192.168.1.3:1718` et le Core s'annonce en mDNS. Le client Node ne parle pas encore TLS. Le code d'appairage réutilisable a été retiré du Pi au moment de cette ouverture. L'image Compose locale de WSL n'a pas été reconstruite.

## Prochains travaux recommandés

1. Reconstruire l'image Compose locale, puis vérifier le Node WSL en BNCP.
2. Phase 2 Technitium, en lecture seule d'abord. Starlink ne la précède pas.
3. TLS sur `1717` avant d'élargir l'écoute au-delà de `192.168.1.3`.
4. Connecteur Starlink en lecture seule, désactivé par défaut.
5. Lancer le Node sur l’hôte WSL si les interfaces réelles de ce poste doivent apparaître.
