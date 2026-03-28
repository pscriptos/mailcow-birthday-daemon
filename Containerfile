FROM scratch

LABEL org.opencontainers.image.source https://git.techniverse.net/scriptos/mailcow-birthday-daemon
ENTRYPOINT ["/mailcow-birthday-daemon"]

ENV STATEFILE=/data/state.json
VOLUME [ "/data" ]

COPY mailcow-birthday-daemon /mailcow-birthday-daemon
