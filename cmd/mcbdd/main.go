package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"git.techniverse.net/scriptos/mailcow-birthday-daemon/pkg/mailcow"
	"github.com/emersion/go-webdav"
)

const (
	ConstUsertokenName = "Birthday Daemon"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type Daemon struct {
	httpClient          *http.Client
	baseURL             string
	mailcowClient       mailcow.Client
	userTokens          map[string]string
	userTokensLock      *sync.RWMutex
	stateFilepath       string
	stateUnsaved        bool
	calendarName        string
	oldCalendarName     string
	notificationEnabled bool
	notificationTrigger string
	eventYears          int
	syncInterval        time.Duration
	excludeMailboxes    map[string]bool
	health              *healthState
}

// initLogLevel liest die Umgebungsvariable LOG_LEVEL und konfiguriert den
// globalen slog-Logger entsprechend. Gültige Werte: debug, info, warn, error.
// Standard: info.
func initLogLevel() {
	var level slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(newBracketHandler(os.Stderr, level)))
}

func main() {
	initLogLevel()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "cleanup":
			if err := runCleanup(); err != nil {
				slog.Error("cleanup failed", "err", err)
				os.Exit(1)
			}
			return
		case "healthcheck":
			if err := runHealthcheck(); err != nil {
				slog.Error("healthcheck failed", "err", err)
				os.Exit(1)
			}
			return
		}
	}
	if err := run(); err != nil {
		slog.Error("fatal error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting mcbdd", "version", version, "commit", commit, "date", date)
	slog.Debug("log level configured", "LOG_LEVEL", os.Getenv("LOG_LEVEL"))

	// Signal-Handling für Graceful Shutdown (SIGTERM, SIGINT).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Kurze Wartezeit beim Start, damit abhängige Dienste (z. B. nginx)
	// vollständig hochgefahren sind, bevor Verbindungen aufgebaut werden.
	const startupDelay = 15 * time.Second
	slog.Info("waiting for dependent services to become ready", "delay", startupDelay)
	select {
	case <-time.After(startupDelay):
		slog.Debug("startup delay completed, proceeding with initialization")
	case <-ctx.Done():
		slog.Warn("shutdown signal received during startup, exiting")
		return nil
	}

	mailcowBase := os.Getenv("MAILCOW_BASE")
	if mailcowBase == "" {
		return fmt.Errorf("MAILCOW_BASE environment variable is not set")
	}
	mailcowAPIKey := os.Getenv("MAILCOW_APIKEY")
	if mailcowAPIKey == "" {
		return fmt.Errorf("MAILCOW_APIKEY environment variable is not set")
	}
	calendarName := os.Getenv("CALENDAR_NAME")
	if calendarName == "" {
		calendarName = "Birthdays"
	}
	notificationEnabled := strings.EqualFold(os.Getenv("NOTIFICATION_ENABLED"), "true")
	eventYears, err := parseEventYears()
	if err != nil {
		return err
	}
	slog.Info("event horizon configured", "years", eventYears)
	syncInterval, err := parseSyncInterval()
	if err != nil {
		return err
	}
	slog.Info("sync interval configured", "interval", syncInterval)
	slog.Debug("configuration summary",
		"MAILCOW_BASE", mailcowBase,
		"CALENDAR_NAME", calendarName,
		"NOTIFICATION_ENABLED", notificationEnabled,
		"EVENT_YEARS", eventYears,
		"MAILCOW_RESOLVE_HOST", os.Getenv("MAILCOW_RESOLVE_HOST"),
		"STATEFILE", os.Getenv("STATEFILE"),
		"MAILBOX_EXCLUDE", os.Getenv("MAILBOX_EXCLUDE"),
	)

	excludeMailboxes := parseMailboxExclude()
	if len(excludeMailboxes) > 0 {
		slog.Info("mailbox exclusion configured", "count", len(excludeMailboxes))
		for addr := range excludeMailboxes {
			slog.Debug("excluding mailbox", "address", addr)
		}
	}

	notificationTrigger := "PT8H"
	if notificationEnabled {
		notificationTime := os.Getenv("NOTIFICATION_TIME")
		if notificationTime == "" {
			notificationTime = "08:00"
		}
		trigger, err := parseNotificationTrigger(notificationTime)
		if err != nil {
			return fmt.Errorf("invalid NOTIFICATION_TIME: %w", err)
		}
		notificationTrigger = trigger
		slog.Info("birthday notifications enabled", "time", notificationTime, "trigger", notificationTrigger)
	} else {
		slog.Debug("birthday notifications disabled")
	}
	d := &Daemon{
		userTokens:          make(map[string]string),
		userTokensLock:      &sync.RWMutex{},
		baseURL:             mailcowBase,
		stateFilepath:       os.Getenv("STATEFILE"),
		httpClient:          &http.Client{Transport: buildTransport()},
		calendarName:        calendarName,
		notificationEnabled: notificationEnabled,
		notificationTrigger: notificationTrigger,
		eventYears:          eventYears,
		syncInterval:        syncInterval,
		excludeMailboxes:    excludeMailboxes,
	}
	if len(d.stateFilepath) == 0 {
		d.stateFilepath = "state.json"
	}
	slog.Debug("state file path", "path", d.stateFilepath)
	d.health = newHealthState(d.stateFilepath)
	d.mailcowClient = mailcow.New(
		d.httpClient,
		mailcowBase,
		mailcowAPIKey,
	)
	if err := d.loadState(); err != nil {
		return err
	}
	slog.Info("initialization complete, entering daemon loop")
	d.daemonLoop(ctx)
	return nil
}

func (d *Daemon) daemonLoop(ctx context.Context) {
	for {
		// Vor jedem Sync prüfen, ob ein Shutdown angefordert wurde.
		select {
		case <-ctx.Done():
			slog.Info("shutdown signal received, exiting")
			return
		default:
		}

		slog.Debug("starting sync cycle")
		start := time.Now()
		err := d.daemonRun()
		d.health.update(err)
		if err != nil {
			slog.Error("error while syncing birthdays", "err", err)
		} else {
			slog.Info("sync cycle completed", "duration", time.Since(start).Round(time.Millisecond))
		}

		slog.Debug("waiting for next sync cycle", "interval", d.syncInterval)
		// Auf nächsten Zyklus oder Shutdown-Signal warten.
		timer := time.NewTimer(d.syncInterval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			slog.Info("shutdown signal received, saving state and exiting")
			if d.stateUnsaved {
				if err := d.saveState(); err != nil {
					slog.Error("error saving state during shutdown", "err", err)
				}
			}
			return
		}
	}
}

func (d *Daemon) daemonRun() error {
	slog.Debug("fetching mailboxes from mailcow API")
	mb, err := d.mailcowClient.GetMailboxes(context.Background())
	if err != nil {
		return fmt.Errorf("error fetching mailboxes: %w", err)
	}
	slog.Info("fetched mailboxes", "count", len(mb))
	eg := sync.WaitGroup{}
	for _, m := range mb {
		eg.Go(func() {
			ctx := context.Background()
			if err := d.processUser(ctx, m); err != nil {
				slog.ErrorContext(ctx, "error processing user", "err", err, "user", m.Username)
			}
		})
	}
	eg.Wait()
	d.oldCalendarName = ""
	if d.stateUnsaved {
		slog.Info("saving tokens to disk", "count", len(d.userTokens))
		if err := d.saveState(); err != nil {
			return err
		}
		d.stateUnsaved = false
	}
	return nil
}

// isMailboxExcluded prüft, ob eine Mailbox über MAILBOX_EXCLUDE
// von der Synchronisation ausgeschlossen ist.
func (d *Daemon) isMailboxExcluded(username string) bool {
	if len(d.excludeMailboxes) == 0 {
		return false
	}
	return d.excludeMailboxes[strings.ToLower(username)]
}

func (d *Daemon) processUser(ctx context.Context, m mailcow.Mailbox) error {
	if !m.IsActive() {
		slog.DebugContext(ctx, "skipping inactive mailbox", "user", m.Username)
		return nil
	}
	if d.isMailboxExcluded(m.Username) {
		slog.DebugContext(ctx, "skipping excluded mailbox", "user", m.Username)
		return nil
	}
	slog.DebugContext(ctx, "processing user", "user", m.Username)
	pass, err := d.getUserPass(ctx, m.Username)
	if err != nil {
		return fmt.Errorf("error getting userpass: %w", err)
	}
	davclient := webdav.HTTPClientWithBasicAuth(d.httpClient, m.Username, pass)
	if d.oldCalendarName != "" {
		slog.DebugContext(ctx, "cleaning up old calendar", "user", m.Username, "oldName", d.oldCalendarName)
		if err := d.cleanupOldCalendar(ctx, davclient, m.Username, d.oldCalendarName); err != nil {
			slog.WarnContext(ctx, "error cleaning up old calendar", "err", err, "user", m.Username)
		}
	}
	bb, err := d.getBirthdays(ctx, davclient, m.Username)
	if err != nil {
		if strings.HasPrefix(err.Error(), "401 Unauthorized: ") {
			slog.WarnContext(ctx, "user password seems to be invalid and will be discarded", "user", m.Username)
			d.userTokensLock.Lock()
			delete(d.userTokens, m.Username)
			d.stateUnsaved = true
			d.userTokensLock.Unlock()
		}
		return fmt.Errorf("error getting birthdays from carddav: %w", err)
	}
	slog.DebugContext(ctx, "found birthday contacts", "user", m.Username, "count", len(bb))
	if err := d.ensureBirthdayCal(ctx, davclient, m.Username); err != nil {
		return fmt.Errorf("error creating birthday calendar in caldav: %w", err)
	}
	if err := d.syncBirthdaysToCal(ctx, davclient, m.Username, bb); err != nil {
		return fmt.Errorf("error syncing birthday events to caldav: %w", err)
	}
	slog.DebugContext(ctx, "user processing complete", "user", m.Username)
	return nil
}

func runCleanup() error {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Verwendung: %s cleanup <alter-kalendername>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nEntfernt einen automatisch erstellten Geburtstagskalender aus allen Mailboxen.\n")
		fmt.Fprintf(os.Stderr, "Nur Kalender, deren Einträge ausschließlich vom Daemon erstellt wurden, werden gelöscht.\n")
		os.Exit(1)
	}
	oldCalendarName := os.Args[2]

	slog.Info("starting calendar cleanup", "calendarName", oldCalendarName)
	slog.Debug("loading environment configuration for cleanup")

	mailcowBase := os.Getenv("MAILCOW_BASE")
	if mailcowBase == "" {
		return fmt.Errorf("MAILCOW_BASE environment variable is not set")
	}
	mailcowAPIKey := os.Getenv("MAILCOW_APIKEY")
	if mailcowAPIKey == "" {
		return fmt.Errorf("MAILCOW_APIKEY environment variable is not set")
	}
	calendarName := os.Getenv("CALENDAR_NAME")
	if calendarName == "" {
		calendarName = "Birthdays"
	}

	d := &Daemon{
		userTokens:     make(map[string]string),
		userTokensLock: &sync.RWMutex{},
		baseURL:        mailcowBase,
		stateFilepath:   os.Getenv("STATEFILE"),
		httpClient:     &http.Client{Transport: buildTransport()},
		calendarName:   calendarName,
	}
	if len(d.stateFilepath) == 0 {
		d.stateFilepath = "state.json"
	}
	d.mailcowClient = mailcow.New(d.httpClient, mailcowBase, mailcowAPIKey)

	if err := d.loadState(); err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	mb, err := d.mailcowClient.GetMailboxes(context.Background())
	if err != nil {
		return fmt.Errorf("error fetching mailboxes: %w", err)
	}

	processed, skipped := 0, 0
	slog.Debug("starting cleanup for all mailboxes", "totalMailboxes", len(mb))
	for _, m := range mb {
		if !m.IsActive() {
			slog.Debug("skipping inactive mailbox during cleanup", "user", m.Username)
			continue
		}
		d.userTokensLock.RLock()
		pass, ok := d.userTokens[m.Username]
		d.userTokensLock.RUnlock()
		if !ok {
			slog.Warn("no stored password for user, skipping", "user", m.Username)
			skipped++
			continue
		}
		ctx := context.Background()
		slog.Debug("cleaning up calendar for user", "user", m.Username, "calendar", oldCalendarName)
		davclient := webdav.HTTPClientWithBasicAuth(d.httpClient, m.Username, pass)
		if err := d.cleanupOldCalendar(ctx, davclient, m.Username, oldCalendarName); err != nil {
			slog.Error("error cleaning up calendar", "user", m.Username, "err", err)
		} else {
			slog.Debug("calendar cleanup successful for user", "user", m.Username)
		}
		processed++
	}

	slog.Info("cleanup finished", "processed", processed, "skipped", skipped)
	return nil
}

// parseMailboxExclude liest MAILBOX_EXCLUDE aus der Umgebung und gibt eine
// Map mit den ausgeschlossenen Mailadressen (lowercase) zurück.
// Mehrere Adressen werden durch Komma getrennt.
func parseMailboxExclude() map[string]bool {
	raw := os.Getenv("MAILBOX_EXCLUDE")
	if raw == "" {
		return nil
	}
	exclude := make(map[string]bool)
	for _, addr := range strings.Split(raw, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			exclude[strings.ToLower(addr)] = true
		}
	}
	if len(exclude) == 0 {
		return nil
	}
	return exclude
}

// parseSyncInterval liest SYNC_INTERVAL aus der Umgebung und gibt die
// geparste Dauer zurück. Standard: 15m.
func parseSyncInterval() (time.Duration, error) {
	raw := os.Getenv("SYNC_INTERVAL")
	if raw == "" {
		return 15 * time.Minute, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid SYNC_INTERVAL %q: %w", raw, err)
	}
	if d < 1*time.Minute {
		return 0, fmt.Errorf("SYNC_INTERVAL must be at least 1m, got %s", d)
	}
	return d, nil
}

// parseEventYears liest EVENT_YEARS aus der Umgebung und gibt die Anzahl
// der Jahre zurück, für die Geburtstags-Events im Voraus erzeugt werden.
// Standard: 10. Gültige Werte: 1–30.
func parseEventYears() (int, error) {
	raw := os.Getenv("EVENT_YEARS")
	if raw == "" {
		return 10, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid EVENT_YEARS %q: %w", raw, err)
	}
	if n < 1 || n > 30 {
		return 0, fmt.Errorf("EVENT_YEARS must be between 1 and 30, got %d", n)
	}
	return n, nil
}

// buildTransport erstellt einen http.Transport.
// Wenn MAILCOW_RESOLVE_HOST gesetzt ist (z. B. "nginx-mailcow"), wird der
// tatsächliche TCP-Connect auf diesen Host umgeleitet, während TLS-SNI und
// Zertifikatsprüfung den Original-Hostnamen aus der URL verwenden.
// Damit wird das Hairpin-NAT-Problem in Docker-Netzen umgangen.
func buildTransport() *http.Transport {
	resolveHost := os.Getenv("MAILCOW_RESOLVE_HOST")
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if resolveHost != "" {
		slog.Info("using internal resolve host for connections", "resolveHost", resolveHost)
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			addr = net.JoinHostPort(resolveHost, port)
			return dialer.DialContext(ctx, network, addr)
		}
	}
	return t
}
