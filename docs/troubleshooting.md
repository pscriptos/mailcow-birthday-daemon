# Troubleshooting

## Container startet, aber keine Kalender werden erstellt

1. Logs prüfen: `docker compose logs -f birthdaydaemon`
2. Sicherstellen, dass der API-Key **Lese- und Schreibzugriff** hat (nicht nur Lesezugriff).
3. Prüfen, ob die Mailbox aktiv ist – inaktive Mailboxen werden übersprungen.

## Verbindungsfehler / Timeout

- Typisches Hairpin-NAT-Problem: `MAILCOW_RESOLVE_HOST=nginx-mailcow` als Umgebungsvariable hinzufügen.
- Sicherstellen, dass der Container im Netzwerk `mailcow-network` ist.

## `401 Unauthorized` in den Logs

Das gespeicherte App-Passwort für den betroffenen Benutzer ist ungültig (z. B. manuell in Mailcow gelöscht). Der Daemon verwirft das alte Passwort automatisch und erstellt beim nächsten Zyklus ein neues.

## Doppelte Kalender nach Umbenennung von `CALENDAR_NAME`

Wurde `CALENDAR_NAME` geändert, bevor der alte Name im State-File gespeichert war (z. B. bei einem Update von vor v0.2.0), existieren pro Mailbox zwei Geburtstagskalender. Der integrierte Cleanup-Befehl entfernt den alten Kalender aus allen Mailboxen – aber nur, wenn alle enthaltenen Einträge vom Daemon erstellt wurden. Manuell angelegte Kalender mit gleichem Namen bleiben unangetastet.

```bash
cd /opt/mailcow-dockerized
docker compose exec birthdaydaemon /mailcow-birthday-daemon cleanup <alter-kalendername>
```

**Beispiel:** Der alte Kalender hieß `Birthdays` und wurde auf `Geburtstage` umgestellt:

```bash
docker compose exec birthdaydaemon /mailcow-birthday-daemon cleanup Birthdays
```

**Beispielausgabe:**

```
2026/03/28 23:46:22 INFO starting calendar cleanup calendarName=Birthdays
2026/03/28 23:46:22 INFO using internal resolve host for connections resolveHost=nginx-mailcow
2026/03/28 23:46:22 INFO removed old birthday calendar user=user1@example.com calendar=Birthdays
2026/03/28 23:46:23 INFO removed old birthday calendar user=user2@example.com calendar=Birthdays
2026/03/28 23:46:23 INFO removed old birthday calendar user=user3@example.com calendar=Birthdays
2026/03/28 23:46:24 INFO cleanup finished processed=5 skipped=0
```

> **Hinweis:** Der Daemon muss vorher mindestens einmal gelaufen sein, damit App-Passwörter im State-File vorhanden sind. Benutzer ohne gespeichertes Passwort werden übersprungen.

## Kalender erscheint nicht in SOGo

SOGo zeigt neue Kalender manchmal erst nach einem Neuladen der Seite (Strg+Shift+R) oder nach dem nächsten Login an. Der Kalender wird unter dem Namen erstellt, der in `CALENDAR_NAME` konfiguriert ist (Standard: `Birthdays`).

## Healthcheck meldet `unhealthy`

1. Logs prüfen: `docker compose logs -f birthdaydaemon`
2. Status manuell abfragen: `docker compose exec birthdaydaemon /mailcow-birthday-daemon healthcheck`
3. Der Healthcheck meldet `unhealthy`, wenn der letzte Sync-Lauf fehlgeschlagen ist oder länger als 20 Minuten zurückliegt.
4. Falls der Container gerade erst gestartet wurde, kann es bis zu 2 Minuten dauern, bis der erste Sync abgeschlossen und der Status `healthy` ist.
