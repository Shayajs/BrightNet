# Environnement

Les faits ci-dessous ont été relevés le 25 octobre 2026. Ce qui n'a pas été mesuré est indiqué comme inconnu.

## Développement local — vérifié

| Élément | Constat |
| --- | --- |
| Système | WSL2, Ubuntu 24.04.4 LTS, noyau `6.18.33.2-microsoft-standard-WSL2`, x86_64 |
| Hyperviseur | Microsoft. Le processeur hôte annoncé dans WSL est un AMD Ryzen 7 7800X3D |
| Ressources WSL | 4 vCPU, 7,8 Go de RAM (environ 3,2 Go disponibles au moment de l'audit), 925 Go libres |
| Git | 2.43.0. Le répertoire du projet était vide et n'était pas un dépôt |
| Docker | Docker Desktop 4.84.0, Engine 29.6.2, Compose v5.3.1, architecture amd64, socket accessible |
| Node | 20.19.0 via nvm, npm 10.8.2. pnpm absent |
| Go | Absent au moment de l'audit. Le toolchain officiel Go 1.26.5 est installé ensuite dans `~/.local/go` |
| Autres | Python 3.12.3, GNU Make 4.3, gcc 13.3, curl, jq. Client `sqlite3` absent |

Le moteur Docker héberge déjà d'autres projets (Lekairos, MediaMTX, bases MySQL, Redis, Nginx Proxy Manager, entre autres). BrightNet ne doit ni les arrêter, ni les modifier, ni publier les ports déjà pris : `82`, `6011`, `8081`, `8189`, `8190`, `8554`, `8890`, `8891`, `8989`, `9997`, `11936`.

Le test de cycle de vie utilise uniquement un conteneur jetable nommé `brightnet-signal`.

L'agent lancé dans WSL voit le moteur Linux de Docker Desktop. Il ne voit pas Windows comme une machine distincte, ni le trafic du réseau domestique.

## Raspberry Pi — lu le 25 octobre 2026

`shayajs@192.168.1.3` refuse la clé présente sur ce poste (`Permission denied (publickey,password)`). Le compte `shaya` accepte `~/.ssh/id_ed25519`. Aucun mot de passe n'a été utilisé. Le compte `shayajs` n'existe pas sur la machine.

| Élément | Constat |
| --- | --- |
| Modèle | Raspberry Pi 3 Model B Plus Rev 1.3 |
| Système | Ubuntu 24.04.3 LTS, noyau `6.8.0-1065-raspi`, aarch64, 4 cœurs |
| Mémoire | 899 Mio, environ 456 Mio disponibles, swap 2 Gio dont 175 Mio utilisés |
| Disque | 29 Go, 16 Go libres sur `/` |
| Route | `default via 192.168.1.1 dev eth0`, adresse `192.168.1.3/24` |
| Docker | Engine 29.1.3 arm64, Compose v5.0.1. `shaya` est dans le groupe `docker` |
| Conteneurs | `technitium-dns` (`technitium/dns-server:latest`, ports 53 et 5380) et `obsidian-couchdb` (`couchdb:latest`, port 5984), tous deux démarrés depuis 2 jours. Un `hello-world` est arrêté. Aucun Pi-hole |
| Ports hôtes | 22, 53, 5380, 5984, 5201 |

Technitium et CouchDB n'ont pas été arrêtés, ni inspectés au-delà du nom, de l'image et des ports. Le Pi tient le Core et un Node si les images sont construites ailleurs : compiler sur place épuiserait la mémoire.

## Accès Internet Starlink — lecture du 25 octobre 2026

Une seule série de lectures, sans écriture et sans parcours du réseau. Le Raspberry Pi a été audité plus tard ; voir la section ci-dessus. Au moment de ces lectures Starlink, il n'avait pas encore été contacté.

| Fait | Constat |
| --- | --- |
| Route par défaut de WSL | `172.30.0.1` sur `eth0`. C'est le NAT Hyper-V, pas la passerelle de la maison |
| Résolveurs vus par WSL | `192.168.1.3`, puis `192.168.1.1` |
| Passerelle Windows | Non lue : l'appel à `route.exe` depuis WSL n'a rien renvoyé |
| Antenne | TCP puis gRPC clairs vers `192.168.100.1:9200` réussis |
| Routeur | TCP vers `192.168.1.1:9000` et `:80` réussi. Le port `443` refuse la connexion |
| Antenne, matériel | `rev4_panda_prod2`, logiciel `2026.10.09.mr88722.2`, pays `FR` |
| Antenne, état | `get_diagnostics`, `get_status` et `get_history` répondent `grpc-status: 0`. Autotest au champ 7 : valeur `PASSED`. Bloc d'alertes vide. L'historique pèse environ 19 Ko |
| Routeur | Matériel `v5`, logiciel `2026.10.18.mr88737`. Un réseau `lan` en `192.168.1.1/24` |
| Clients annoncés | 2 Ethernet, 2 en 2,4 GHz, 5 en 5 GHz. Ce sont des comptes, pas des identités |
| IPv6 | Trois préfixes sur le LAN : un global `/60`, un ULA `/60`, un lien-local. Les adresses ne sont pas recopiées |
| CGNAT | Aucune adresse `100.64.0.0/10` dans ces réponses. Le cas reste prévu, il n'est pas confirmé ici |
| Identifiants | L'antenne cite deux routeurs. Un seul répond sur `192.168.1.1`. Les identifiants ne sont pas recopiés |
| Position | `get_location` n'a pas été appelé |

Conséquence : le routeur Starlink n'est pas en mode bypass pour ce LAN. Il annonce `192.168.1.1/24` et porte le Wi-Fi. Il est donc le routeur du réseau observé. On ne peut pas encore affirmer qu'il est la route par défaut de Windows, ni que le Pi ne fait pas le DNS ou le DHCP : `192.168.1.3` est le premier résolveur de WSL, et ses services n'ont pas été interrogés.

Les API lues ne montrent pas les flux des appareils. La vue des communications ne sera pas dessinée à partir de ces compteurs.

## Contraintes WSL2 et Docker

- Le socket Docker ne doit être monté que dans l'agent, en lecture seule.
- Dans Compose, le Core publie `127.0.0.1:8787` et l'interface `127.0.0.1:5173`.
- L'identité d'hôte vient de l'identifiant persistant de l'agent et, s'il est monté, du `machine-id` de l'hôte. Le nom d'hôte d'un conteneur d'agent n'est pas l'identité de la machine.
- Docker Desktop peut rendre le socket accessible avec des droits élevés. Le conteneur d'agent de développement tourne en root pour pouvoir le lire. Ce choix est limité au développement et documenté dans `docs/SECURITY.md`.
