# Décisions

## ADR 001 — Go pour le Core et l'agent

Le Core doit pouvoir tenir sur une petite machine ARM, et l'agent doit être un binaire unique à faible empreinte. Go 1.26 permet de partager le contrat, de compiler sans CGO et de produire des binaires statiques pour `linux/amd64` et, plus tard, `linux/arm64`.

Le paquet Ubuntu `golang-go` (1.21) n'est pas utilisé. Le toolchain officiel 1.26.5 est la référence, identique à la version majeure employée par Docker Engine 29.6.2.

## ADR 002 — SQLite plutôt qu'une base serveur

Un usage personnel produit des centaines ou quelques milliers d'entités, pas un flux métrique illimité. SQLite en mode WAL, via `modernc.org/sqlite`, évite un conteneur supplémentaire et reste compilable sans CGO. Postgres, Neo4j, Prometheus et un broker de messages sont hors du MVP.

Les métriques fines et la rétention longue pourront migrer plus tard si le volume l'exige. Le schéma versionné ne dépend pas d'un moteur graphe : le graphe est une projection.

## ADR 003 — Client Docker minimal en lecture seule

L'agent parle à l'API Engine par le socket Unix et n'implémente que des lectures : informations, conteneurs, réseaux, volumes, événements. Il n'appelle pas `inspect` et ne lit donc pas les variables d'environnement. Les labels conservés sont limités à l'identité Compose. Les chemins de montage de l'hôte et les points de montage des volumes ne sont pas stockés.

Ce client étroit remplace le SDK Moby complet : le binaire ne contient pas les endpoints de création, d'arrêt ou de suppression.

## ADR 004 — Cytoscape.js

Cytoscape.js, avec la disposition `fcose`, gère les nœuds composés, les positions imposées, la sélection et plusieurs centaines d'éléments. Sigma.js est plus adapté à un très grand graphe WebGL, mais moins aux groupes imbriqués demandés. Vue Flow ressemble davantage à un éditeur de nœuds qu'à une carte d'infrastructure.

Les positions enregistrées priment. La disposition automatique ne s'applique qu'aux nœuds qui n'ont pas encore de position, pour éviter les sauts à chaque événement.

## ADR 005 — HTTP JSON et SSE

L'interface reçoit le graphe par `GET /api/v1/graph` et les invalidations par SSE. Le navigateur ne déduit pas la topologie. WebSocket sera envisagé si un flux bidirectionnel continu devient nécessaire.

L'agent envoie un instantané, des événements et des battements de cœur. La réponse du Core ne contient que l'intervalle et la liste des collecteurs autorisés. Aucune commande shell ni aucun ordre Docker ne transite.

## ADR 006 — Boucle locale sans TLS, appairage dès le premier agent

En développement, le Core n'écoute que sur `127.0.0.1` (ou sur l'interface interne de Compose, publiée uniquement vers `127.0.0.1`). TLS n'est pas terminé dans le MVP tant que rien n'est exposé sur le réseau. L'appairage par code à usage court et le jeton d'agent hashé sont déjà en place. Tout agent qui quittera la machine devra passer par TLS. Cette étape est un prérequis de la phase de déploiement, pas une option.

## ADR 007 — Pas de fusion sur une IP ou une image

Deux observations partagent une entité seulement si leur clé technique coïncide :

- hôte : identifiant d'agent ;
- moteur : identifiant Docker Info ;
- conteneur Compose : moteur, projet et service ;
- conteneur sans Compose : moteur et nom ;
- réseau et volume : moteur et identifiant ou nom.

Une adresse IP, une adresse MAC non déclarée par l'hôte, un nom d'hôte ou une image Docker ne fusionnent pas. Deux agents qui présenteraient le même `machine-id` sont signalés dans l'audit et restent distincts.

## ADR 008 — Les statistiques ne sont pas pollées en continu

Interroger `stats` pour chaque conteneur du moteur local chargerait Docker Desktop, déjà occupé par d'autres projets. Le MVP reflète l'état (`running`, `exited`, `absent`) via la liste et les événements. Les compteurs CPU et mémoire attendent un canal où l'interface désigne explicitement un conteneur, sans ordre d'exécution arbitraire.

## ADR 009 — Interface en français

L'interface, la documentation et les messages d'erreur visibles sont en français. Les identifiants techniques, les types de relation et les chemins d'API restent en anglais.

## ADR 010 — Starlink en lecture seule, après Technitium

L'accès Internet de la maison est Starlink. Le connecteur est facultatif et n'entre pas dans le MVP Docker. Technitium reste l'incrément suivant.

Le client ne reprend pas `starlink_exporter` comme dépendance. Ce projet est utile comme catalogue de métriques, mais il embarque le proto complet et la position GPS. BrightNet n'enverra que `get_diagnostics`, `get_status` et `get_history`, vers les adresses explicitement configurées. Par défaut : antenne `192.168.100.1:9200`, routeur `192.168.1.1:9000`.

Les compteurs du lien satellite décrivent l'antenne. Ils ne sont pas le trafic des appareils. La vue `communications` reste vide tant qu'aucune source ne fournit des flux réels. Une adresse CGNAT ou un préfixe IPv6 ne fusionne pas d'entité, conformément à l'ADR 007.

## ADR 011 — BNCP entre les Node et le Core

Le canal des Node n'utilise plus HTTP. BNCP version 1 est une enveloppe de 12 octets (`BNCP`, version, type, drapeaux, longueur) suivie d'un corps CBOR qui reprend les noms du contrat JSON. Les types sont l'appairage, le battement, l'instantané, les événements, et leurs réponses. Une réponse ne contient que l'intervalle et les collecteurs autorisés.

Le navigateur reste en JSON et SSE. L'ingestion JSON est conservée pour la transition. Le développement écoute `127.0.0.1:1717` pour l'interface et `127.0.0.1:1718` pour BNCP. TLS, par certificat et clé fournis au Core, est obligatoire avant d'exposer ces ports sur le réseau de la maison. Le Raspberry Pi n'est pas contacté tant que son audit en lecture seule n'est pas fait.

## ADR 012 — Plusieurs Node, un hôte

Un terminal peut lancer plusieurs BrightNet Node. Quand ils annoncent le même `machine-id`, ils écrivent le même hôte `host:machine:{id}` et ne retirent pas les entités d'un autre Node. Sans `machine-id`, chacun garde l'hôte `host:agent:{id}`. Une adresse IP ne fusionne toujours pas deux entités.
