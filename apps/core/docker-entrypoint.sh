#!/bin/sh
set -eu
mkdir -p /data
chown brightnet:brightnet /data
exec setpriv --reuid=brightnet --regid=brightnet --init-groups --inh-caps=-all /usr/local/bin/brightnet-core
