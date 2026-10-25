# Modèle de données

SQLite, migrations dans `internal/store/migrations`. Les horodatages sont des chaînes RFC3339 UTC.

## Entités

`entities` représente l'inventaire canonique.

| Colonne | Rôle |
| --- | --- |
| `id` | ULID stable, distinct du nom affiché |
| `kind` | `host`, `network_interface`, `address`, `router`, `docker_engine`, `docker_container`, `docker_network`, `docker_volume`, `service` |
| `display_name` | nom modifiable |
| `name_locked` | si vrai, un instantané ne réécrit pas le nom |
| `technical_key` | clé unique de réconciliation |
| `parent_id` | parent visuel principal. Un conteneur n'a qu'un parent, le moteur |
| `status` | `online`, `running`, `exited`, `offline`, `absent` ou l'état Docker |
| `simulated` | donnée de fixture |
| `properties` | JSON public, sans secrets |
| `source_agent_id` | agent qui a produit l'observation |
| `first_seen_at`, `last_seen_at` | durée de vie observée |

Clés :

- hôte avec `machine-id` : `host:machine:{machineID}`, partagé par les Node de cette machine
- hôte sans `machine-id` : `host:agent:{agentID}`
- routeur : `router:{machineID}:{passerelle}`, ou `router:{agentID}:{passerelle}` sans machine-id. Seule la route par défaut lue dans la table de routage du Node est enregistrée. La relation est `ROUTES_VIA`, preuve `observed`
- interface : `iface:{agentID}:{nom}`
- adresse : `addr:{agentID}:{interface}:{ip}`
- moteur : `docker-engine:{engineID}`
- conteneur Compose : `container:{engineID}:{projet}:{service}`
- autre conteneur : `container:{engineID}:name:{nom}`
- réseau : `docker-network:{engineID}:{networkID}`
- volume : `docker-volume:{engineID}:{nom}`
- service publié : `service:{cleConteneur}`, un seul nœud pour l'ensemble des ports publiés du conteneur

L'identifiant de conteneur Docker est une propriété. Une recréation Compose conserve la même clé. Une image identique ne suffit pas.

Une même adresse IP sur deux agents produit deux lignes `address`. Un `machine-id` déjà vu sur un autre agent est journalisé et ne fusionne pas les hôtes.

## Relations

`relations` relie deux entités.

| Colonne | Rôle |
| --- | --- |
| `kind` | `RUNS`, `HOSTS`, `BELONGS_TO`, `MEMBER_OF`, `MOUNTS`, `EXPOSES` |
| `evidence` | `declared`, `configured`, `observed`, `inferred` |
| `confidence` | de 0 à 1 |
| `status` | `active` ou `absent` |
| `properties` | précisions, par exemple le port |
| `first_seen_at`, `last_seen_at` | observations |
| `source_agent_id` | provenance |

Contrainte unique `(source_id, target_id, kind)`.

Le MVP n'émet pas de relation `inferred`. Les labels Compose sont de classe `configured` pour l'identité du conteneur ; la présence dans le moteur est `observed`. Les résolutions DNS futures seront `RESOLVES` et ne seront jamais enregistrées comme `COMMUNICATES_WITH`.

Réservé pour Starlink, non écrit par les collecteurs actuels :

- `starlink_terminal`, clé `starlink-terminal:{id}`
- `starlink_router`, clé `starlink-router:{id}`
- réseau local du routeur, clé `starlink-lan:{id routeur}:{domaine}` ; le préfixe IPv4 ou IPv6 est une propriété
- relations `UPLINKS` (routeur vers antenne, `observed`) et `SERVES` (routeur vers son LAN, `observed`)
- relation `AFFECTED_BY` (`inferred`) entre un service et un événement d'interruption, jamais `COMMUNICATES_WITH`

Une adresse dans `100.64.0.0/10` ou un préfixe IPv6 ne sont pas des clés. Les échantillons de latence et de débit iront dans `metrics`. Les interruptions iront dans `events`.

## Autres tables

- `observations` : charge utile normalisée de chaque instantané, rattachée à une entité.
- `agents` : identité, hash du jeton, dernier battement, version.
- `pairing_codes` : hash du code, expiration, date d'usage.
- `collector_runs` : succès ou échec isolé de chaque collecteur.
- `events` : changements d'état et faits Docker reçus.
- `graph_layouts` : positions JSON par utilisateur et par vue.
- `users`, `sessions` : accès local.
- `audit_log` : actions sensibles.
- `metrics`, `alerts` : réservées, non alimentées par les collecteurs du MVP.
- `schema_migrations` : version appliquée.

## Instantané

Le corps `brightnet.ingest.v1` contient l'hôte, l'éventuel inventaire Docker, le résultat de chaque collecteur et le drapeau `simulated`. Le Core ignore tout champ d'environnement s'il apparaissait malgré le filtre de l'agent.

Les observations brutes sont conservées 48 heures. Les événements le sont 7 jours. Ces durées sont fixes dans le MVP et pourront devenir des réglages. Les entités et les relations restent, avec le statut `absent` lorsqu'elles ne sont plus vues.
