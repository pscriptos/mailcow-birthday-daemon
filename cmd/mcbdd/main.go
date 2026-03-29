package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
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
	syncInterval        time.Duration
	health              *healthState
}

func main() {
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

	// Kurze Wartezeit beim Start, damit abhängige Dienste (z. B. nginx)
	// vollständig hochgefahren sind, bevor Verbindungen aufgebaut werden.
	const startupDelay = 15 * time.Second
	slog.Info("waiting for dependent services to become ready", "delay", startupDelay)
	time.Sleep(startupDelay)

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
	syncInterval, err := parseSyncInterval()
	if err != nil {
		return err
	}
	slog.Info("sync interval configured", "interval", syncInterval)

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
		syncInterval:        syncInterval,
	}
	if len(d.stateFilepath) == 0 {
		d.stateFilepath = "state.json"
	}
	d.health = newHealthState(d.stateFilepath)
	d.mailcowClient = mailcow.New(
		d.httpClient,
		mailcowBase,
		mailcowAPIKey,
	)
	if err := d.loadState(); err != nil {
		return err
	}
	d.daemonLoop()
	return nil
}

func (d *Daemon) daemonLoop() {
	for {
		err := d.daemonRun()
		d.health.update(err)
		if err != nil {
			slog.Error("error while syncing birthdays", "err", err)
		}
		time.Sleep(d.syncInterval)
	}
}

func (d *Daemon) daemonRun() error {
	mb, err := d.mailcowClient.GetMailboxes(context.Background())
	if err != nil {
		return fmt.Errorf("error fetching mailboxes: %w", err)
	}
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

func (d *Daemon) processUser(ctx context.Context, m mailcow.Mailbox) error {
	if !m.IsActive() {
		return nil
	}
	pass, err := d.getUserPass(ctx, m.Username)
	if err != nil {
		return fmt.Errorf("error getting userpass: %w", err)
	}
	davclient := webdav.HTTPClientWithBasicAuth(d.httpClient, m.Username, pass)
	if d.oldCalendarName != "" {
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
	if err := d.ensureBirthdayCal(ctx, davclient, m.Username); err != nil {
		return fmt.Errorf("error creating birthday calendar in caldav: %w", err)
	}
	if err := d.syncBirthdaysToCal(ctx, davclient, m.Username, bb); err != nil {
		return fmt.Errorf("error syncing birthday events to caldav: %w", err)
	}
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
	for _, m := range mb {
		if !m.IsActive() {
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
		davclient := webdav.HTTPClientWithBasicAuth(d.httpClient, m.Username, pass)
		if err := d.cleanupOldCalendar(ctx, davclient, m.Username, oldCalendarName); err != nil {
			slog.Error("error cleaning up calendar", "user", m.Username, "err", err)
		}
		processed++
	}

	slog.Info("cleanup finished", "processed", processed, "skipped", skipped)
	return nil
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
