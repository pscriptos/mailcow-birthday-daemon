# Mailcow Birthday Daemon 🎂

> **Fork-Hinweis:** Dieses Projekt ist ein Fork von [Marco98/mailcow-birthday-daemon](https://github.com/Marco98/mailcow-birthday-daemon) und wird hier eigenständig weiterentwickelt.

Ein einfacher Daemon, der automatisch einen Geburtstagskalender für jede Mailcow-Mailbox erzeugt und synchronisiert. Es ist kein Benutzereingriff erforderlich – alles läuft vollautomatisch.

![Kalenderansicht](assets/img/kalenderansicht.png)

## Kurzübersicht

- Liest Geburtstage aus allen CardDAV-Adressbüchern jeder Mailbox
- Erstellt und synchronisiert automatisch einen Geburtstagskalender pro Benutzer
- Synchronisation alle **15 Minuten**
- Läuft als Docker-Container direkt im Mailcow-Stack

## Schnellstart

```yaml
services:
    birthdaydaemon:
        image: git.techniverse.net/scriptos/mailcow-birthday-daemon:latest
        restart: always
        depends_on:
            - nginx-mailcow
        networks:
            - mailcow-network
        environment:
            - MAILCOW_BASE=https://mail.example.com
            - MAILCOW_APIKEY=DEIN-APIKEY-HIER
        volumes:
            - birthdaydaemon:/data

volumes:
    birthdaydaemon:
```

> Alle verfügbaren Image-Tags sind in der [Container Registry](https://git.techniverse.net/scriptos/-/packages/container/mailcow-birthday-daemon) einsehbar.

## Dokumentation

Die vollständige Dokumentation befindet sich im Ordner [`docs/`](docs/README.md).

<p align="center">
  <img src="https://assets.techniverse.net/f1/git/graphics/gray0-catonline.svg" alt="">
</p>

<p align="center">
<img src="https://assets.techniverse.net/f1/logos/small/license.png" alt="License" width="15" height="15"> <a href="./LICENSE">License</a> | <img src="https://assets.techniverse.net/f1/logos/small/matrix2.svg" alt="Matrix" width="15" height="15"> <a href="https://matrix.to/#/#community:techniverse.net">Matrix</a> | <img src="https://assets.techniverse.net/f1/logos/small/mastodon2.svg" alt="Mastodon" width="15" height="15"> <a href="https://social.techniverse.net/@donnerwolke">Mastodon</a>
</p>
