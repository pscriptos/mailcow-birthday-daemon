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

## Kalender erscheint nicht in SOGo

SOGo zeigt neue Kalender manchmal erst nach einem Neuladen der Seite (Strg+Shift+R) oder nach dem nächsten Login an. Der Kalender wird unter dem Namen erstellt, der in `CALENDAR_NAME` konfiguriert ist (Standard: `Birthdays`).
