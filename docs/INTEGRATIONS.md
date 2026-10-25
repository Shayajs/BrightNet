# Intégrations

## Docker Engine — phase 1

L'agent interroge le socket local, par défaut `/var/run/docker.sock`.

Lectures utilisées :

- `GET /info`
- `GET /containers/json?all=1`
- `GET /networks`
- `GET /volumes`
- `GET /events`

Aucun appel d'écriture n'est implémenté. `inspect` n'est pas appelé, ce qui exclut les variables d'environnement de la collecte. Les labels retenus sont `com.docker.compose.project`, `com.docker.compose.service` et `com.docker.compose.container-number`.

Les événements suivis sont ceux de type conteneur, réseau et volume (`create`, `start`, `die`, `destroy`, `pause`, `unpause` et les équivalents réseau ou volume). Après un événement, l'agent renvoie un instantané pour réconcilier l'état complet.

Plusieurs moteurs restent des entités distinctes, identifiées par le champ `ID` de `/info`.

## Technitium — phase 2, non branché

La documentation officielle consultée est [APIDOCS.md](https://github.com/TechnitiumSoftware/DnsServer/blob/master/APIDOCS.md), branche `master`, au moment de la conception. La version installée sur le Raspberry Pi est inconnue. Le connecteur devra d'abord lire la version réelle et échouer sans rien modifier si elle est incompatible.

API HTTP, port documenté `5380`, à confirmer sur l'hôte. Depuis Technitium 15.0, l'authentification est un jeton Bearer créé par `login` ou `createToken`. Le paramètre `token` en query existe pour compatibilité ; le connecteur utilisera l'en-tête.

Lectures prévues, et seulement celles-là :

| Usage | Chemin | Permission documentée |
| --- | --- | --- |
| Présence du serveur | `/api/status` | aucune |
| Compteurs | `/api/dashboard/stats/get` | Dashboard: View |
| Classements | `/api/dashboard/stats/getTop` (`TopClients`, `TopDomains`, `TopBlockedDomains`) | Dashboard: View |
| Zones | `/api/zones/list` | à confirmer selon la version, lecture seule |
| Enregistrements | `/api/zones/records/get` | lecture seule |
| Fichiers de journaux | `/api/logs/list` | Logs: View |
| Requêtes journalisées | `/api/logs/query` | Logs: View |
| Étendues DHCP | `/api/dhcp/scopes/list` | DhcpServer: View |
| Baux DHCP | `/api/dhcp/leases/list` | DhcpServer: View |

Les chemins de suppression, de création, de blocage, de restauration et de changement de réglages existent dans la documentation et sont interdits au connecteur. Les statistiques agrégées ne sont pas des connexions TCP ou UDP. Une requête DNS deviendra une observation `RESOLVES`.

Le rôle de Pi-hole par rapport à Technitium n'est pas connu. Aucun des deux n'est supposé être le résolveur de tous les appareils. Les baux DHCP ne seront lus que si l'audit montre que Technitium assure ce rôle.

## Starlink — facultatif, non branché

Lecture unique du 25 octobre 2026, détaillée dans `docs/ENVIRONMENT.md`. Le collecteur n'est pas compilé dans l'agent. Il ne bloque ni le MVP Docker, déjà vérifié, ni Technitium.

Référence étudiée : [joshuasing/starlink_exporter](https://github.com/joshuasing/starlink_exporter) v0.9.2, licence MIT, non affilié à Starlink. Il interroge l'antenne en gRPC local et publie des métriques Prometheus. BrightNet ne l'embarquera pas : le binaire vendorise tout le proto SpaceX, y compris des appels d'écriture, et il relève la position GPS. Le futur client BrightNet recodera un sous-ensemble en liste blanche.

Deux API coexistent sur le même service `SpaceX.API.Device.Device/Handle`, en gRPC clair :

| Cible | Adresse habituelle | Appels retenus |
| --- | --- | --- |
| Antenne | `192.168.100.1:9200` | `get_diagnostics` (champ 6000, API publiée), `get_status` (champ 1004), `get_history` (champ 1007) |
| Routeur Wi-Fi | `192.168.1.1:9000` | `get_diagnostics` seulement |

L'API publiée par [SpaceX enterprise-api/device-api](https://github.com/SpaceExplorationTechnologies/enterprise-api/tree/master/device-api) ne documente que `get_diagnostics`, à partir des logiciels routeur 2023.55 et antenne 2023.51.0. Les générations plus anciennes peuvent répondre `UNIMPLEMENTED`. Le proto évolue par champs ajoutés : un firmware inconnu ne doit pas faire échouer le décodage. Sur ce site, l'antenne `rev4_panda_prod2` (logiciel `2026.10.09.mr88722.2`) et le routeur `v5` (logiciel `2026.10.18.mr88737`) répondent aux deux familles d'appels. Le champ numérique 3 de la réponse vaut 43.

Métriques prévues, lues au plus une fois par minute, stockées dans la table `metrics` déjà réservée, pas comme un nœud par échantillon :

- état de liaison, latence vers le point de présence, ratio de perte, débits montant et descendant ;
- obstruction instantanée et secondes obstruées ;
- alertes antenne (chauffe, limitation thermique, Ethernet lent) ;
- comptes de clients du routeur, séparés en Ethernet, 2,4 GHz et 5 GHz.

`get_history` a renvoyé un tampon d'environ 19 Ko. C'est la source de la courbe de performance. Les échantillons agrégés du lien satellite ne deviennent jamais des arêtes `COMMUNICATES_WITH`. Les comptes de clients ne créent pas un nœud par appareil. Une liste de clients, si une lecture ultérieure l'apporte, ne relie un hôte que lorsque cet hôte a déclaré la même interface.

Une interruption est un événement de la chronologie (`starlink.outage`, `starlink.obstruction`, `starlink.unreachable`). La corrélation avec un service est une relation `AFFECTED_BY`, classe `inferred`, confiance basse, limitée au chevauchement de temps. Elle ne prouve pas qu'un flux précis a échoué.

Identité :

- antenne : `starlink-terminal:{id matériel}`
- routeur : `starlink-router:{id matériel}`
- réseau local annoncé : `starlink-lan:{id routeur}:{domaine}`, le préfixe n'est qu'une propriété

Une adresse IPv4 dans `100.64.0.0/10` (CGNAT) n'est pas une identité publique et ne fusionne aucune entité. Un préfixe IPv6, même global, ne fusionne pas non plus. L'appel `get_location` est interdit : la position n'est pas nécessaire au graphe.

Aucun appel d'écriture, de redémarrage, d'arrimage, de test de débit ou de changement de configuration. Le collecteur est absent tant que `BRIGHTNET_STARLINK` n'est pas défini. Une antenne injoignable est un échec de collecteur, comme Docker aujourd'hui : le reste de l'inventaire reste en place.

Fichiers prévus le jour de l'implémentation, et pas avant la phase Technitium :

```text
apps/agent/internal/collectors/starlink/client.go
apps/agent/internal/collectors/starlink/collect.go
apps/agent/internal/collectors/starlink/collect_test.go
```

## Pi-hole, SNMP, Home Assistant, MQTT, OpenTelemetry

Non implémentés. Ils restent des collecteurs futurs, isolés, avec des capacités déclarées. Pi-hole ne sera ajouté que si Technitium ne fournit pas déjà la même information utile.
