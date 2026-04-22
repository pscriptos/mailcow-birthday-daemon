<p align="center">
  <a href="https://techniverse.net">
    <img src="https://assets.techniverse.net/f1/git/graphics/repo-techniverse-logo.png" alt="Techniverse Community" height="70" />
  </a>
</p>

<h1 align="center"> Mailcow Birthday Daemon 🎂</h1>

<h4 align="center">
  Ein einfacher Daemon, der automatisch einen Geburtstagskalender für jede Mailcow-Mailbox erzeugt und synchronisiert.
</h4>

<h6 align="center">
  <a href="https://www.cleveradmin.de">🏰 Website</a>
  ·
  <a href="https://techniverse.net">📰 Community</a>
  ·
  <a href="https://social.techniverse.net/@donnerwolke">🐘 Mastodon</a>
  ·
  <a href="https://matrix.to/#/#support:techniverse.net">💬 Support</a>
</h6>
<br><br>

![Kalenderansicht](assets/img/kalenderansicht.png)

## ✨ Highlights

🌟 **Automatische Geburtstagskalender:**
  - Liest Geburtstage & Jahrestage aus allen CardDAV-Adressbüchern jeder Mailbox
  - Erstellt & synchronisiert für jeden Benutzer einen eigenen Geburtstagskalender

🎨 **Individuelle Anpassung:**
  - Kalendername & Kalenderfarbe frei wählbar
  - Sync-Intervall & Event-Horizont flexibel einstellbar

⏰ **Benachrichtigungen:**
  - Optionale Erinnerungen (VALARM) zur gewünschten Uhrzeit

🚫 **Mailbox-Management:**
  - Einzelne Mailboxen gezielt ausschließen
  - Automatische Verwaltung von App-Passwörtern (persistenter State)

🧹 **Cleanup & Wartung:**
  - Integrierter Cleanup-Befehl für doppelte Kalender nach Umbenennung
  - Startup-Connectivity-Check, Graceful Shutdown & Docker-Healthcheck

🌐 **Docker-Optimierung:**
  - Hairpin-NAT-Lösung für Docker-Netzwerke
  - Strukturiertes Logging mit konfigurierbarem Log-Level
  - Läuft als Docker-Container direkt im Mailcow-Stack


## 🚀 Schnellstart

Den folgenden Abschnitt in eine `docker-compose.override.yml` im Mailcow-Verzeichnis (z. B. `/opt/mailcow-dockerized`) einfügen:

> **Wichtig:** Da `mailcow-dockerized` die eigene `docker-compose.yml` bei Updates überschreibt, müssen eigene Anpassungen immer in der `docker-compose.override.yml` erfolgen. Docker Compose lädt diese Datei automatisch und mergt sie mit der Hauptkonfiguration – eigene Änderungen gehen dadurch bei Mailcow-Updates nicht verloren.

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

> **Hinweis:** Das obige Beispiel zeigt nur die minimal nötigen Umgebungsvariablen. Eine vollständige Übersicht aller verfügbaren Umgebungsvariablen findest du im [Schnellstart](docs/schnellstart.md).

Anschließend starten:

```bash
cd /opt/mailcow-dockerized
docker compose up -d
```

> Alle verfügbaren Image-Tags sind in der [Container Registry](https://git.techniverse.net/scriptos/-/packages/container/mailcow-birthday-daemon) einsehbar.

## 🔗 Repository-Spiegel

| Rolle | URL |
|-------|-----|
| **Master** | https://git.techniverse.net/scriptos/mailcow-birthday-daemon.git |
| **Spiegel** | https://github.com/pscriptos/mailcow-birthday-daemon.git |

> **Hinweis:** Die Entwicklung findet im Master-Repository statt. Der GitHub-Spiegel wird automatisch synchronisiert. Issues und Feature-Requests können sowohl auf [Gitea](https://git.techniverse.net/scriptos/mailcow-birthday-daemon/issues) als auch auf [GitHub](https://github.com/pscriptos/mailcow-birthday-daemon/issues) eingereicht werden.

## 📚 Dokumentation

Die vollständige Dokumentation befindet sich im Ordner [`docs/`](docs/README.md).

<br><br>
<p align="center">
  <img src="https://assets.techniverse.net/f1/git/graphics/gray0-catonline.svg" alt="">
</p>

<p align="center">
  <sub>
     © Patrick Asmus · Techniverse Network · <a href="./LICENSE">Lizenz</a>
  </sub>
</p>