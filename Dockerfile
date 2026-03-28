FROM alpine:latest AS certs
RUN apk add --no-cache ca-certificates

FROM scratch

LABEL org.opencontainers.image.source https://git.techniverse.net/scriptos/mailcow-birthday-daemon
ENTRYPOINT ["/mailcow-birthday-daemon"]

ENV STATEFILE=/data/state.json
VOLUME [ "/data" ]

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY mailcow-birthday-daemon /mailcow-birthday-daemon
