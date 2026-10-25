FROM scratch AS core
COPY brightnet-core /brightnet-core
COPY web /usr/local/share/brightnet/web
COPY downloads /var/lib/brightnet/downloads
EXPOSE 8787 1717 1718
ENTRYPOINT ["/brightnet-core"]

FROM scratch AS node
COPY brightnet-agent /brightnet-agent
ENTRYPOINT ["/brightnet-agent"]
