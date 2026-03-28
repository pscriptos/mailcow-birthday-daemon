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

> **Wichtig:** Damit die Umbenennung korrekt erkannt wird, muss der Daemon **mindestens einmal** mit dem neuen Code und dem **alten** Kalendernamen gelaufen sein, damit der Name im State-File gespeichert wird. Erst danach `CALENDAR_NAME` ändern und erneut starten. Wird der Name geändert, bevor der State aktualisiert wurde, kann der alte Kalender nicht automatisch entfernt werden und muss manuell gelöscht werden.

## Benachrichtigungen

- Wenn `NOTIFICATION_ENABLED=true` gesetzt ist, erhält jedes Geburtstags-Event einen **VALARM** (iCal-Alarm). Kalender-Clients (SOGo, iOS, Android, Thunderbird) zeigen dann zur konfigurierten Uhrzeit eine Benachrichtigung an. Das Event bleibt weiterhin ein Ganztags-Event.
- Bestehende Events ohne VALARM werden beim nächsten Synchronisationszyklus automatisch neu erstellt – keine manuelle Migration nötig.
- Falls die Benachrichtigungen wieder deaktiviert werden (`NOTIFICATION_ENABLED=false`), werden Events mit VALARM ebenfalls automatisch durch Events ohne VALARM ersetzt.

## Synchronisationsintervall

Der Synchronisationszyklus läuft alle **15 Minuten** automatisch.
