# Dokumentation

Willkommen in der Dokumentation des **Mailcow Birthday Daemon** 🎂

## Inhaltsverzeichnis

- [Schnellstart](schnellstart.md) – Installation und erste Einrichtung
- [Update](update.md) – Bestehende Installation aktualisieren
- [Funktionsweise](funktionsweise.md) – Technische Details zum Synchronisationsprozess
- [Troubleshooting](troubleshooting.md) – Häufige Probleme und Lösungen

## Kurzübersicht

- Automatischer Geburtstagskalender für jede Mailcow-Mailbox
- Liest Geburtstage und Jahrestage aus allen CardDAV-Adressbüchern
- Optionale Benachrichtigungen (VALARM) zur konfigurierbaren Uhrzeit
- Konfigurierbares Sync-Intervall, Event-Horizont und Kalenderfarbe
- Mailboxen einzeln ausschließbar, strukturiertes Logging mit Log-Level
- Docker-Healthcheck, Graceful Shutdown und Startup-Connectivity-Check
- Läuft als Docker-Container im Mailcow-Stack via `docker-compose.override.yml`
