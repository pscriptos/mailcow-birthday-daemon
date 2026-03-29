# Funktionsweise

## Übersicht

Der Mailcow Birthday Daemon synchronisiert automatisch Geburtstagskalender für jede aktive Mailbox. Der gesamte Prozess läuft ohne Benutzereingriff ab.

## App-Passwörter

- Über die Mailcow-API wird für jeden aktiven Benutzer ein App-Passwort mit Zugriff auf CardDAV und CalDAV erzeugt.
- Da jedes App-Passwort in Mailcow eine global hochzählende Nummer erhält, werden die Passwörter auf der Festplatte gespeichert, um das unnötige Ansteigen dieser Nummer zu vermeiden.

## Kontakte und Geburtstage

- Alle Kontakte aus sämtlichen Adressbüchern werden abgerufen und die Geburtstagsinformationen je Benutzer extrahiert.
- Zusätzlich zu Geburtstagen werden auch Jahrestage (`ANNIVERSARY`-Feld nach vCard 4.0 / RFC 6350) ausgelesen. Dieses Feld wird von allen gängigen Clients unterstützt (Android, iOS, Thunderbird).
- Geburtstags-Events erhalten das Präfix 🎂, Jahrestags-Events das Präfix 💍 – so sind beide Typen im Kalender sofort unterscheidbar.
- Die daraus resultierenden Kalendereinträge werden im Voraus berechnet.

> **Hinweis zu Jahrestagen:** Die SOGo-Weboberfläche bietet kein Feld zum Anzeigen oder Bearbeiten von Jahrestagen. Das `ANNIVERSARY`-Feld muss über einen externen Client (z. B. Thunderbird, Android- oder iOS-Kontakte-App) gepflegt und per CardDAV synchronisiert werden. Das SOGo-CardDAV-Backend speichert und liefert das Feld korrekt – es fehlt lediglich die Unterstützung in der Web-UI.
    - Aktuell standardmäßig: 1 Jahr in der Vergangenheit, 10 Jahre in der Zukunft (konfigurierbar über `EVENT_YEARS`).
    - Selbstverständlich pro Mailbox isoliert – ein Benutzer sieht nur die Geburtstage seiner eigenen Kontakte.

## Mailbox-Filter

Standardmäßig erhalten alle aktiven Mailboxen einen Geburtstagskalender. Über die Umgebungsvariable `MAILBOX_EXCLUDE` können einzelne Mailboxen von der Synchronisation ausgeschlossen werden (z. B. Service-Accounts oder Shared Mailboxen). Ausgeschlossene Mailboxen werden beim Sync-Zyklus übersprungen. Bereits vorhandene Kalender in diesen Mailboxen werden dabei nicht automatisch entfernt – dafür kann der `cleanup`-Befehl verwendet werden (siehe [Troubleshooting](troubleshooting.md#doppelte-kalender-nach-umbenennung-von-calendar_name)).

Die Variable kann jederzeit – auch bei bestehenden Installationen – per Update hinzugefügt oder geändert werden.

## Kalendersynchronisation

- Die berechneten Ereignisse werden in einen Kalender synchronisiert, dessen Name über `CALENDAR_NAME` konfigurierbar ist (Standard: „Birthdays"). Der Anzeigename kann vom Benutzer in SOGo zusätzlich umbenannt werden.
- Bei Änderung von `CALENDAR_NAME` wird der alte Kalender beim nächsten Start automatisch entfernt und ein neuer mit dem neuen Namen erstellt. Der alte Kalender wird dabei nur gelöscht, wenn er ausschließlich vom Daemon erstellte Einträge enthält – manuell angelegte Kalender mit gleichem Namen bleiben unangetastet.

> **Wichtig (betrifft nur Updates von vor v0.2.0):** Damit die Umbenennung korrekt erkannt wird, muss der Daemon **mindestens einmal** mit dem neuen Code (ab v0.2.0) und dem **alten** Kalendernamen gelaufen sein, damit der Name im State-File gespeichert wird. Erst danach `CALENDAR_NAME` ändern und erneut starten. Wird der Name geändert, bevor der State aktualisiert wurde, kann der alte Kalender nicht automatisch entfernt werden. In diesem Fall kann der integrierte Cleanup-Befehl verwendet werden (siehe [Troubleshooting](troubleshooting.md#doppelte-kalender-nach-umbenennung-von-calendar_name)).

## Kalenderfarbe

- Der Daemon setzt automatisch die CalDAV-Property `calendar-color` (Apple-Namespace) auf dem Geburtstagskalender. Dadurch hebt sich der Kalender im Client farblich sofort ab.
- Die Farbe ist über `CALENDAR_COLOR` konfigurierbar (Standard: `#D01818`). Das Format ist `#RRGGBB`.
- Hat ein Benutzer die Farbe seines Kalenders manuell geändert (z. B. in SOGo oder einem anderen Client), wird diese Anpassung respektiert und nicht überschrieben.
- Bei einer Änderung von `CALENDAR_COLOR` werden alle Kalender aktualisiert, deren Farbe nicht manuell angepasst wurde.

## Benachrichtigungen

- Wenn `NOTIFICATION_ENABLED=true` gesetzt ist, erhält jedes Geburtstags-Event einen **VALARM** (iCal-Alarm). Kalender-Clients (SOGo, iOS, Android, Thunderbird) zeigen dann zur konfigurierten Uhrzeit eine Benachrichtigung an. Das Event bleibt weiterhin ein Ganztags-Event.
- Bestehende Events ohne VALARM werden beim nächsten Synchronisationszyklus automatisch neu erstellt – keine manuelle Migration nötig.
- Falls die Benachrichtigungen wieder deaktiviert werden (`NOTIFICATION_ENABLED=false`), werden Events mit VALARM ebenfalls automatisch durch Events ohne VALARM ersetzt.

## Synchronisationsintervall

Der Synchronisationszyklus läuft standardmäßig alle **15 Minuten** automatisch. Das Intervall kann über die Umgebungsvariable `SYNC_INTERVAL` angepasst werden (z. B. `SYNC_INTERVAL=30m`). Details zu den möglichen Werten finden sich in der [Umgebungsvariablen-Tabelle](schnellstart.md#umgebungsvariablen).

## Graceful Shutdown

Der Daemon reagiert auf `SIGTERM` und `SIGINT` (z. B. durch `docker stop`) und beendet sich sauber:

1. Der aktuelle Synchronisationszyklus wird noch vollständig abgeschlossen.
2. Ungespeicherte App-Passwörter werden in die Zustandsdatei geschrieben.
3. Erst danach beendet sich der Prozess.

Dadurch wird sichergestellt, dass das State-File konsistent bleibt und keine Daten verloren gehen. Auch die Startverzögerung (15 Sekunden) wird bei einem Shutdown-Signal sofort abgebrochen.

## Healthcheck

- Das Dockerfile enthält eine `HEALTHCHECK`-Anweisung, die den eingebauten Subcommand `healthcheck` nutzt – es werden keine externen Tools wie `curl` oder `wget` benötigt und kein Port wird geöffnet.
- Nach jedem Sync-Lauf schreibt der Daemon eine kleine Statusdatei (`health.json`) neben das State-File. Der `healthcheck`-Subcommand liest diese Datei und prüft, ob der letzte Sync aktuell und fehlerfrei war.
- Docker zeigt den Status in `docker ps` als `(healthy)` oder `(unhealthy)` an.
- Der Healthcheck meldet **unhealthy**, wenn der letzte Sync-Lauf fehlgeschlagen ist oder länger als das konfigurierte Sync-Intervall plus 5 Minuten Toleranz zurückliegt. Während der Startphase (bevor der erste Sync abgeschlossen ist) gilt der Daemon als healthy.
