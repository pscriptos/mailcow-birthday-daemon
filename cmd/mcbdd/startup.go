package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"git.techniverse.net/scriptos/mailcow-birthday-daemon/pkg/mailcow"
)

const (
	// startupBackoffInit ist das anfängliche Warteintervall zwischen
	// Erreichbarkeitsprüfungen (2 Sekunden).
	startupBackoffInit = 2 * time.Second

	// startupBackoffMax begrenzt das maximale Warteintervall auf 30 Sekunden.
	startupBackoffMax = 30 * time.Second

	// startupRequestTimeout begrenzt einzelne HTTP-Anfragen während der
	// Startphase auf 10 Sekunden.
	startupRequestTimeout = 10 * time.Second
)

// waitForServices prüft in einer Schleife, ob die Mailcow-API und SOGo
// erreichbar sind. Erst wenn beide Dienste antworten, kehrt die Funktion
// zurück. Bei einem Shutdown-Signal (ctx.Done) wird sofort nil
// zurückgegeben.
func waitForServices(ctx context.Context, httpClient *http.Client, baseURL, apiKey string) error {
	slog.Info("checking connectivity to mailcow API and SOGo")

	backoff := startupBackoffInit

	for {
		apiOK := checkMailcowAPI(ctx, httpClient, baseURL, apiKey)
		sogoOK := checkSOGo(ctx, httpClient, baseURL)

		if apiOK && sogoOK {
			slog.Info("all services reachable, proceeding with initialization")
			return nil
		}

		// Klare Meldung, welcher Dienst noch nicht bereit ist.
		if !apiOK && !sogoOK {
			slog.Warn("mailcow API and SOGo not reachable yet, retrying", "backoff", backoff)
		} else if !apiOK {
			slog.Warn("mailcow API not reachable yet, retrying", "backoff", backoff)
		} else {
			slog.Warn("SOGo not reachable yet, retrying", "backoff", backoff)
		}

		// Auf nächsten Versuch oder Shutdown-Signal warten.
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			slog.Warn("shutdown signal received during startup connectivity check, exiting")
			return nil
		}

		// Exponentielles Backoff verdoppeln, aber nicht über das Maximum.
		backoff *= 2
		if backoff > startupBackoffMax {
			backoff = startupBackoffMax
		}
	}
}

// checkMailcowAPI führt einen einfachen API-Aufruf (GetMailboxes) durch,
// um zu prüfen, ob die Mailcow-API erreichbar ist und der API-Key gültig
// ist. Bei Erfolg wird true zurückgegeben.
func checkMailcowAPI(ctx context.Context, httpClient *http.Client, baseURL, apiKey string) bool {
	reqCtx, cancel := context.WithTimeout(ctx, startupRequestTimeout)
	defer cancel()

	mc := mailcow.New(httpClient, baseURL, apiKey)
	_, err := mc.GetMailboxes(reqCtx)
	if err != nil {
		slog.Debug("mailcow API check failed", "err", err)
		return false
	}
	slog.Debug("mailcow API is reachable")
	return true
}

// checkSOGo prüft die Erreichbarkeit von SOGo über einen HTTP-GET auf den
// SOGo-Basispfad. Jeder HTTP-Statuscode (auch Redirects oder 401) gilt als
// "erreichbar", da SOGo in diesem Fall antwortet. Nur Netzwerkfehler
// (connection refused, timeout) gelten als "nicht erreichbar".
func checkSOGo(ctx context.Context, httpClient *http.Client, baseURL string) bool {
	reqCtx, cancel := context.WithTimeout(ctx, startupRequestTimeout)
	defer cancel()

	sogoURL, err := url.JoinPath(baseURL, "SOGo/")
	if err != nil {
		slog.Debug("SOGo URL construction failed", "err", err)
		return false
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, sogoURL, nil)
	if err != nil {
		slog.Debug("SOGo request creation failed", "err", err)
		return false
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Debug("SOGo check failed", "err", err)
		return false
	}
	defer resp.Body.Close()

	// SOGo ist erreichbar – der konkrete Statuscode ist irrelevant,
	// solange eine HTTP-Antwort zurückkommt.
	slog.Debug("SOGo is reachable", "status", resp.StatusCode)
	return true
}


