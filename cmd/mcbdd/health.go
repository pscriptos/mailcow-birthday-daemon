package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxSyncAge berechnet die maximale Dauer seit dem letzten Sync-Lauf, bevor
// der Healthcheck den Daemon als unhealthy meldet. Die Toleranz beträgt
// 5 Minuten über dem konfigurierten Sync-Intervall.
func maxSyncAge() time.Duration {
	syncInterval, err := parseSyncInterval()
	if err != nil {
		// Fallback: 20 Minuten (15m Standard-Intervall + 5m Toleranz).
		return 20 * time.Minute
	}
	return syncInterval + 5*time.Minute
}

// healthFile ist der Dateiname der Healthcheck-Statusdatei, die neben dem
// State-File abgelegt wird.
const healthFile = "health.json"

// healthStatus wird als JSON in die Healthcheck-Datei geschrieben.
type healthStatus struct {
	LastSync  time.Time `json:"last_sync"`
	LastError string    `json:"last_error,omitempty"`
}

// healthState hält den aktuellen Health-Status im Speicher und schreibt
// ihn nach jedem Sync-Lauf in eine Datei.
type healthState struct {
	mu        sync.Mutex
	filePath  string
}

func newHealthState(stateFilepath string) *healthState {
	dir := filepath.Dir(stateFilepath)
	return &healthState{
		filePath: filepath.Join(dir, healthFile),
	}
}

// update wird nach jedem Sync-Lauf aufgerufen und schreibt den Status
// in die Health-Datei.
func (h *healthState) update(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := healthStatus{
		LastSync: time.Now(),
	}
	if err != nil {
		s.LastError = err.Error()
	}
	data, _ := json.Marshal(s)
	if writeErr := os.WriteFile(h.filePath, data, 0644); writeErr != nil {
		slog.Error("failed to write health file", "path", h.filePath, "err", writeErr)
	} else {
		slog.Debug("health status updated", "path", h.filePath, "lastError", s.LastError != "")
	}
}

// runHealthcheck liest die Health-Datei und prüft, ob der Daemon healthy ist.
// Exit-Code 0 = healthy, 1 = unhealthy. Wird von Docker HEALTHCHECK aufgerufen.
func runHealthcheck() error {
	stateFilepath := os.Getenv("STATEFILE")
	if stateFilepath == "" {
		stateFilepath = "state.json"
	}
	healthPath := filepath.Join(filepath.Dir(stateFilepath), healthFile)
	slog.Debug("running healthcheck", "healthPath", healthPath)

	data, err := os.ReadFile(healthPath)
	if err != nil {
		return fmt.Errorf("health file not found: %w (daemon may still be starting)", err)
	}
	var s healthStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("invalid health file: %w", err)
	}
	if s.LastError != "" {
		slog.Warn("healthcheck: last sync had an error", "lastError", s.LastError)
		return fmt.Errorf("last sync failed: %s", s.LastError)
	}
	if time.Since(s.LastSync) > maxSyncAge() {
		slog.Warn("healthcheck: last sync too old", "age", time.Since(s.LastSync).Round(time.Second))
		return fmt.Errorf("last sync too old: %s ago", time.Since(s.LastSync).Round(time.Second))
	}
	slog.Debug("healthcheck passed", "lastSync", s.LastSync.Format("2006/01/02 15:04:05"))
	return nil
}
