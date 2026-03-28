# Mailcow Birthday Daemon 🎂

> **Fork-Hinweis:** Dieses Projekt ist ein Fork von [Marco98/mailcow-birthday-daemon](https://github.com/Marco98/mailcow-birthday-daemon) und wird hier eigenständig weiterentwickelt.

Ein einfacher Daemon, der automatisch einen Geburtstagskalender für jede Mailcow-Mailbox erzeugt und synchronisiert.

Es ist kein Benutzereingriff erforderlich. Alles wird vollautomatisch erledigt.

## Installation

Den folgenden Abschnitt in die `docker-compose.override.yml` einfügen:

```yaml
services:
    birthdaydaemon:
        image: git.techniverse.net/scriptos/mailcow-birthday-daemon:latest
        restart: always
        environment:
        - MAILCOW_BASE=https://mailcow.host
        - MAILCOW_APIKEY=DEIN-APIKEY-HIER
        volumes:
        - birthdaydaemon:/data
volumes:
    birthdaydaemon:
```

> **Tipp:** Statt `:latest` kann auch eine feste Version wie `:1.0.0` verwendet werden.

Den API-Key findet man im Admin-Panel unter Konfiguration > Zugang > Administratordetails bearbeiten > API > Lese-/Schreibzugriff.

Da die Mailcow-API derzeit nicht vollständig ist und sich eher im Early-Access-Stadium befindet, wird dringend davon abgeraten, die Option „IP-Prüfung für API überspringen" zu aktivieren.

## Konfiguration (Umgebungsvariablen)

| Variable | Pflicht | Standardwert | Beschreibung |
|---|---|---|---|
| `MAILCOW_BASE` | **Ja** | – | Basis-URL der Mailcow-Instanz (z. B. `https://mailcow.example.com`) |
| `MAILCOW_APIKEY` | **Ja** | – | API-Key mit Lese-/Schreibzugriff aus dem Mailcow-Admin-Panel |
| `STATEFILE` | Nein | `state.json` (im Container: `/data/state.json`) | Pfad zur Zustandsdatei, in der App-Passwörter gespeichert werden |

> **Hinweis für bestehende Installationen:** Es wurden keine Variablennamen oder Funktionalitäten geändert. Ein Update ist ohne Anpassungen möglich.

## Funktionsweise

- Über die Mailcow-API wird für jeden aktiven Benutzer ein App-Passwort mit Zugriff auf CardDAV und CalDAV erzeugt.
    - Da jedes App-Passwort in Mailcow eine global hochzählende Nummer erhält, werden die Passwörter auf der Festplatte gespeichert, um das unnötige Ansteigen dieser Nummer zu vermeiden.
- Alle Kontakte aus sämtlichen Adressbüchern werden abgerufen und die Geburtstagsinformationen je Benutzer extrahiert.
- Die daraus resultierenden Kalendereinträge werden im Voraus berechnet.
    - Aktuell fest eingestellt: 1 Jahr in der Vergangenheit, 10 Jahre in der Zukunft.
    - Selbstverständlich pro Mailbox isoliert – ein Benutzer sieht nur die Geburtstage seiner eigenen Kontakte.
- Die berechneten Ereignisse werden in einen Kalender namens „Birthdays" in jeder Mailbox synchronisiert (der Anzeigename kann vom Benutzer in SOGo umbenannt werden).
- Der Synchronisationszyklus läuft alle **15 Minuten** automatisch.
