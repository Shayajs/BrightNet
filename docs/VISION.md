# Vision

BrightNet est le jumeau numérique local d'une infrastructure domestique. Ouvert dans un navigateur, il présente les machines, les systèmes, les interfaces, Docker et, plus tard, le DNS et les objets connectés comme un graphe hiérarchique que l'on explore.

Un nœud est une entité réelle ou logique. Un lien est une relation dont on connaît le type, la date et la provenance. Une animation correspond à un événement observé. Une donnée simulée est toujours signalée comme telle.

Le produit final vise la maison entière. Le premier logiciel utilisable se limite à ce qu'un agent Linux peut voir sur sa propre machine : l'hôte et le moteur Docker local. Le Raspberry Pi, Technitium, Pi-hole, le scan du réseau et les actions de contrôle viendront dans des phases séparées, après validation. L'accès Internet est Starlink : son antenne et son routeur seront un connecteur facultatif, en lecture seule, après Technitium. Le débit du lien satellite n'est pas le trafic des appareils.

BrightNet fonctionne sans cloud et sans compte externe. Il est auto-hébergé. Le Core ne reçoit jamais un accès administratif direct aux moteurs Docker : chaque agent observe localement et n'envoie que l'inventaire autorisé.
