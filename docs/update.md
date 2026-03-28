# Update

## Image aktualisieren

Um den Mailcow Birthday Daemon auf die neueste Version zu aktualisieren, genügen folgende Schritte im Mailcow-Verzeichnis:

```bash
cd /opt/mailcow-dockerized
docker compose pull birthdaydaemon
docker compose up -d birthdaydaemon
```

## Auf eine bestimmte Version wechseln

1. Die gewünschte Version in der [Container Registry](https://git.techniverse.net/scriptos/-/packages/container/mailcow-birthday-daemon) auswählen.
2. Den Image-Tag in der `docker-compose.override.yml` anpassen:

```yaml
image: git.techniverse.net/scriptos/mailcow-birthday-daemon:1.0.0
```

3. Container neu starten:

```bash
docker compose up -d birthdaydaemon
```

## Hinweise

- Die Zustandsdatei (`/data/state.json`) im Volume `birthdaydaemon` bleibt bei Updates erhalten. Gespeicherte App-Passwörter werden weiterverwendet.
- Ein Neustart des Containers löst sofort einen Synchronisationszyklus aus.
- Falls sich der Standard-Kalendername (`CALENDAR_NAME`) mit einem Update ändert, siehe den Abschnitt zur Kalender-Umbenennung in der [Schnellstart-Dokumentation](schnellstart.md#funktionsweise).
