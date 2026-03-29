# Funktionsweise

## Übersicht

Der Mailcow Birthday Daemon synchronisiert automatisch Geburtstagskalender für jede aktive Mailbox. Der gesamte Prozess läuft ohne Benutzereingriff ab.

## App-Passwörter

- Über die Mailcow-API wird für jeden aktiven Benutzer ein App-Passwort mit Zugriff auf CardDAV und CalDAV erzeugt.
- Da jedes App-Passwort in Mailcow eine global hochzählende Nummer erhält, werden die Passwörter auf der Festplatte gespeichert, um das unnötige Ansteigen dieser Nummer zu vermeiden.

## Kontakte und Geburtstage

- Alle Kontakte aus sämtlichen Adressbüchern werden abgerufen und die Geburtstagsinformationen je Benutzer extrahiert.
- Die daraus resultierenden Kalendereinträge werden im Voraus berechnet.
    - Aktuell fest eingestellt: 1 Jahr in der Vergangenheit, 10 Jahre in der Zukunft.
    - Selbstverständlich pro Mailbox isoliert – ein Benutzer sieht nur die Geburtstage seiner eigenen Kontakte.

## Kalendersynchronisation

- Die berechneten Ereignisse werden in einen Kalender synchronisiert, dessen Name über `CALENDAR_NAME` konfigurierbar ist (Standard: „Birthdays"). Der Anzeigename kann vom Benutzer in SOGo zusätzlich umbenannt werden.
- Bei Änderung von `CALENDAR_NAME` wird der alte Kalender beim nächsten Start automatisch entfernt und ein neuer mit dem neuen Namen erstellt. Der alte Kalender wird dabei nur gelöscht, wenn er ausschließlich vom Daemon erstellte Einträge enthält – manuell angelegte Kalender mit gleichem Namen bleiben unangetastet.

> **Wichtig (betrifft nur Updates von vor v0.2.0):** Damit die Umbenennung korrekt erkannt wird, muss der Daemon **mindestens einmal** mit dem neuen Code (ab v0.2.0) und dem **alten** Kalendernamen gelaufen sein, damit der Name im State-File gespeichert wird. Erst danach `CALENDAR_NAME` ändern und erneut starten. Wird der Name geändert, bevor der State aktualisiert wurde, kann der alte Kalender nicht automatisch entfernt werden. In diesem Fall kann der integrierte Cleanup-Befehl verwendet werden (siehe [Troubleshooting](troubleshooting.md#doppelte-kalender-nach-umbenennung-von-calendar_name)).

## Benachrichtigungen

- Wenn `NOTIFICATION_ENABLED=true` gesetzt ist, erhält jedes Geburtstags-Event einen **VALARM** (iCal-Alarm). Kalender-Clients (SOGo, iOS, Android, Thunderbird) zeigen dann zur konfigurierten Uhrzeit eine Benachrichtigung an. Das Event bleibt weiterhin ein Ganztags-Event.
- Bestehende Events ohne VALARM werden beim nächsten Synchronisationszyklus automatisch neu erstellt – keine manuelle Migration nötig.
- Falls die Benachrichtigungen wieder deaktiviert werden (`NOTIFICATION_ENABLED=false`), werden Events mit VALARM ebenfalls automatisch durch Events ohne VALARM ersetzt.

## Synchronisationsintervall

Der Synchronisationszyklus läuft alle **15 Minuten** automatisch.

## Healthcheck

- Das Dockerfile enthält eine `HEALTHCHECK`-Anweisung, die den eingebauten Subcommand `healthcheck` nutzt – es werden keine externen Tools wie `curl` oder `wget` benötigt und kein Port wird geöffnet.
- Nach jedem Sync-Lauf schreibt der Daemon eine kleine Statusdatei (`health.json`) neben das State-File. Der `healthcheck`-Subcommand liest diese Datei und prüft, ob der letzte Sync aktuell und fehlerfrei war.
- Docker zeigt den Status in `docker ps` als `(healthy)` oder `(unhealthy)` an.
- Der Healthcheck meldet **unhealthy**, wenn der letzte Sync-Lauf fehlgeschlagen ist oder länger als 20 Minuten zurückliegt. Während der Startphase (bevor der erste Sync abgeschlossen ist) gilt der Daemon als healthy.
