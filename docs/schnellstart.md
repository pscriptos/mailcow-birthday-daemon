# Schnellstart

## Voraussetzungen

- Eine laufende [Mailcow](https://mailcow.email/)-Instanz mit Docker Compose
- Ein API-Key mit **Lese-/Schreibzugriff** (Admin-Panel → Konfiguration → Zugang → Administratordetails bearbeiten → API)

## Installation

Den folgenden Abschnitt in die `docker-compose.override.yml` der Mailcow-Installation einfügen:

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
            - MAILCOW_RESOLVE_HOST=nginx-mailcow
        volumes:
            - birthdaydaemon:/data

volumes:
    birthdaydaemon:
```

> **Wichtig:** `mail.example.com` muss durch den tatsächlichen FQDN der eigenen Mailcow-Instanz ersetzt werden.

> **Hinweis zu `MAILCOW_RESOLVE_HOST`:** Innerhalb eines Docker-Netzes kann der Container die öffentliche Domain (z. B. `mail.example.com`) oft nicht über die externe IP erreichen – ein typisches **Hairpin-NAT-Problem**. Die Variable `MAILCOW_RESOLVE_HOST=nginx-mailcow` sorgt dafür, dass TCP-Verbindungen direkt an den Mailcow-Nginx-Container im selben Docker-Netz aufgebaut werden, anstatt den Umweg über die öffentliche IP zu nehmen. TLS-SNI und die Zertifikatsprüfung verwenden dabei weiterhin den Hostnamen aus `MAILCOW_BASE`, sodass die Verbindung korrekt verschlüsselt bleibt.

> **Tipp:** Statt `:latest` kann auch eine feste Version wie `:1.0.0` verwendet werden. Alle verfügbaren Tags sind in der [Container Registry](https://git.techniverse.net/scriptos/-/packages/container/mailcow-birthday-daemon) einsehbar.

## Container starten

```bash
cd /opt/mailcow-dockerized
docker compose up -d
```

## Umgebungsvariablen

| Variable | Pflicht | Standardwert | Beschreibung |
|---|---|---|---|
| `MAILCOW_BASE` | **Ja** | – | Basis-URL der Mailcow-Instanz (z. B. `https://mailcow.example.com`) |
| `MAILCOW_APIKEY` | **Ja** | – | API-Key mit Lese-/Schreibzugriff aus dem Mailcow-Admin-Panel |
| `MAILCOW_RESOLVE_HOST` | Nein | – | Interner Hostname für TCP-Verbindungen (z. B. `nginx-mailcow`). Löst Hairpin-NAT-Probleme in Docker-Netzen. TLS nutzt weiterhin den Hostnamen aus `MAILCOW_BASE`. |
| `CALENDAR_NAME` | Nein | `Birthdays` | Name des Geburtstagskalenders, der in jeder Mailbox erstellt wird |
| `STATEFILE` | Nein | `state.json` (im Container: `/data/state.json`) | Pfad zur Zustandsdatei, in der App-Passwörter und der aktuelle Kalendername gespeichert werden |

## API-Key erstellen

Den API-Key findet man im Admin-Panel unter Konfiguration → Zugang → Administratordetails bearbeiten → API → Lese-/Schreibzugriff.

> **Warnung:** Da die Mailcow-API derzeit nicht vollständig ist und sich eher im Early-Access-Stadium befindet, wird dringend davon abgeraten, die Option „IP-Prüfung für API überspringen" zu aktivieren.

## Prüfen, ob alles läuft

```bash
docker compose logs -f birthdaydaemon
```

Nach dem Start synchronisiert der Daemon automatisch alle 15 Minuten die Geburtstagskalender für jede Mailbox.

> **Hinweis für bestehende Installationen:** Falls der Daemon die Mailcow-API wegen Hairpin-NAT nicht erreichen kann, muss lediglich `MAILCOW_RESOLVE_HOST=nginx-mailcow` als Umgebungsvariable ergänzt werden. Details siehe [Installationsabschnitt](#installation).

## Funktionsweise

- Über die Mailcow-API wird für jeden aktiven Benutzer ein App-Passwort mit Zugriff auf CardDAV und CalDAV erzeugt.
    - Da jedes App-Passwort in Mailcow eine global hochzählende Nummer erhält, werden die Passwörter auf der Festplatte gespeichert, um das unnötige Ansteigen dieser Nummer zu vermeiden.
- Alle Kontakte aus sämtlichen Adressbüchern werden abgerufen und die Geburtstagsinformationen je Benutzer extrahiert.
- Die daraus resultierenden Kalendereinträge werden im Voraus berechnet.
    - Aktuell fest eingestellt: 1 Jahr in der Vergangenheit, 10 Jahre in der Zukunft.
    - Selbstverständlich pro Mailbox isoliert – ein Benutzer sieht nur die Geburtstage seiner eigenen Kontakte.
- Die berechneten Ereignisse werden in einen Kalender synchronisiert, dessen Name über `CALENDAR_NAME` konfigurierbar ist (Standard: „Birthdays"). Der Anzeigename kann vom Benutzer in SOGo zusätzlich umbenannt werden.
    - Bei Änderung von `CALENDAR_NAME` wird der alte Kalender beim nächsten Start automatisch entfernt und ein neuer mit dem neuen Namen erstellt. Der alte Kalender wird dabei nur gelöscht, wenn er ausschließlich vom Daemon erstellte Einträge enthält – manuell angelegte Kalender mit gleichem Namen bleiben unangetastet.
    - **Wichtig:** Damit die Umbenennung korrekt erkannt wird, muss der Daemon **mindestens einmal** mit dem neuen Code und dem **alten** Kalendernamen gelaufen sein, damit der Name im State-File gespeichert wird. Erst danach `CALENDAR_NAME` ändern und erneut starten. Wird der Name geändert, bevor der State aktualisiert wurde, kann der alte Kalender nicht automatisch entfernt werden und muss manuell gelöscht werden.
- Der Synchronisationszyklus läuft alle **15 Minuten** automatisch.
