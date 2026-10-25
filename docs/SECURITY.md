# Sécurité

BrightNet observe une infrastructure personnelle. Le MVP applique les limites suivantes.

## Périmètre

- Aucune dépendance cloud et aucune télémétrie.
- Aucune découverte active des appareils. Le Node ne balaie pas les adresses : sans `BRIGHTNET_BNCP_ADDR`, il écoute les annonces mDNS des Core (`_brightnet._tcp`) et, s'il n'y en a qu'un, s'y connecte. Plusieurs Core sont proposés au terminal. À défaut d'annonce, il essaie seulement `127.0.0.1:1718`. Un appareil sans Node n'est pas interrogé.
- Aucune action de contrôle : pas de démarrage, d'arrêt, de suppression, de modification DNS ni d'exécution de commande.
- Le connecteur Starlink, lorsqu'il existera, n'ouvrira que les adresses configurées et n'enverra que `get_diagnostics`, `get_status` et `get_history`. Pas de configuration, pas de redémarrage, pas de position GPS. Une lecture manuelle du 25 octobre 2026 a vérifié ces réponses ; elle n'est pas un balayage du LAN.
- Le Core ne monte pas le socket Docker.
- L'agent Docker ne met en œuvre que des requêtes de lecture. Il ne stocke ni variables d'environnement, ni chemins hôtes des montages, ni labels arbitraires.

## Authentification

- Un administrateur local est créé au premier démarrage si `BRIGHTNET_ADMIN_USER` et `BRIGHTNET_ADMIN_PASSWORD` sont définis. Sinon, `POST /api/v1/setup` n'est accepté qu'une fois, tant qu'aucun utilisateur n'existe.
- Le mot de passe est hashé avec Argon2id.
- La session navigateur est un cookie `HttpOnly`, `SameSite=Lax`. En HTTP local, le drapeau `Secure` est absent. Il devra être activé avec TLS.
- Un agent s'inscrit avec un code d'appairage à durée limitée. Le code et le jeton sont stockés sous forme de hash SHA-256. Le jeton en clair n'est renvoyé qu'une fois.
- Les routes d'ingestion refusent toute requête sans jeton d'agent correspondant à l'identifiant du chemin.
- Le code de développement `BRIGHTNET_DEV_PAIRING_CODE` est réutilisable et ne doit pas être défini sur un service exposé au-delà de la machine. Un code créé depuis l'interface expire en dix minutes et ne sert qu'une fois.

## Données

- Les secrets de développement vivent dans `.env`, ignoré par Git. `.env.example` ne contient que des valeurs de boucle locale.
- Les corps JSON sont limités à 2 Mio.
- Le journal d'audit enregistre la connexion, l'appairage, le chargement de fixtures et le changement de nom affiché.
- Les fixtures sont marquées `simulated` et ne sont pas présentées comme des observations.

## Développement Compose

Le conteneur d'agent de développement est root afin de lire le socket Docker Desktop. Ce n'est pas le modèle d'un agent installé sur un hôte : le binaire prévu pour un hôte peut tourner avec un groupe `docker` restreint. Le socket est monté `:ro`.

## Avant tout accès réseau

TLS obligatoire avant d'écouter sur toutes les interfaces, cookie `Secure`, et codes d'appairage à usage unique. En développement, `1717` et `1718` restent sur `127.0.0.1` et ne sont pas annoncés. Sur le Pi, ils sont liés à `192.168.1.3` pour que les Node de la maison puissent joindre le Core ; le chiffrement de ce lien reste à faire. Le code d'appairage réutilisable ne doit pas être défini tant que `1718` est joignable depuis le réseau. Un appareil sans Node n'est pas interrogé.

## Limites connues du MVP

- Pas de verrouillage de compte après des essais de mot de passe, hors limitation simple de l'appairage.
- Pas de rotation de jeton dans l'interface.
- Un processus local sur la même machine peut appeler l'API si un mot de passe de développement faible est utilisé. Le mot de passe d'exemple ne doit pas sortir de WSL.
