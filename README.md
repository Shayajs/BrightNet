# BrightNet

BrightNet représente une infrastructure domestique comme un graphe vivant. Cette version est le MVP local : un agent Linux envoie l'hôte et le moteur Docker au Core, et l'interface les affiche.

La vision, les limites et l'état réel du travail sont dans `docs/`.

## Démarrage

Prérequis : Docker Compose, Node 20, Go 1.26 si vous lancez les binaires hors Docker. Le toolchain Go de développement est `~/.local/go` lorsqu'il a été installé par la procédure du dépôt.

```bash
cp .env.example .env
export PATH="$HOME/.local/go/bin:$PATH"
docker compose -f deploy/compose/docker-compose.dev.yml up --build
```

Ouvrir `http://127.0.0.1:5173` pendant le développement, ou `http://127.0.0.1:1717` une fois l'interface construite dans l'image du Core. Les Node parlent BNCP sur le port `1718`. L'API JSON reste sur `8787`.

Compte de développement, défini dans `.env.example` : utilisateur `admin`, mot de passe `brightnet-dev`. Ces valeurs ne servent que sur la boucle locale.

Le BrightNet Node de Compose s'appaire en BNCP et lit `/var/run/docker.sock` en lecture seule. Le Core n'a pas accès à ce socket. Plusieurs Node qui présentent le même `machine-id` alimentent un seul hôte.

Pour suivre un cycle de vie sans toucher aux autres conteneurs :

```bash
docker run -d --name brightnet-signal --label com.docker.compose.project=brightnet-signal --label com.docker.compose.service=signal nginx:alpine
docker stop brightnet-signal
docker start brightnet-signal
docker rm -f brightnet-signal
```

## Hors Docker

```bash
export PATH="$HOME/.local/go/bin:$PATH"
set -a && source .env && set +a
mkdir -p data
go run ./apps/core/cmd/brightnet-core
go run ./apps/agent/cmd/brightnet-agent
cd apps/web && npm install && npm run dev
```

## Tests

```bash
export PATH="$HOME/.local/go/bin:$PATH"
go test ./...
cd apps/web && npm run typecheck
```

## État

Le Raspberry Pi n'est pas déployé. Technitium n'est pas connecté. Starlink est documenté et a été lu une fois ; le collecteur n'est pas branché. Le détail de ce qui a été vérifié est dans `docs/PROGRESS.md`.
