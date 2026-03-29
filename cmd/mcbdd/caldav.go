package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/google/uuid"
)

const (
	ConstProductID = "-//scriptos//MailcowBirthdayDaemon//EN"
)

// cleanupOldCalendar prüft, ob ein alter Daemon-Kalender existiert, und löscht
// ihn nur, wenn alle enthaltenen Events die Daemon-PRODID tragen. Enthält der
// Kalender fremde Events, wird er nicht gelöscht und eine Warnung geloggt.
func (d *Daemon) cleanupOldCalendar(ctx context.Context, httpClient webdav.HTTPClient, user, oldName string) error {
	slog.DebugContext(ctx, "checking for old calendar to clean up", "user", user, "oldName", oldName)
	endpoint, err := url.JoinPath(d.baseURL, "SOGo/dav", user, "Calendar/")
	if err != nil {
		return err
	}
	cl, err := caldav.NewClient(httpClient, endpoint)
	if err != nil {
		return err
	}
	cc, err := cl.FindCalendars(ctx, "")
	if err != nil {
		return err
	}
	slog.DebugContext(ctx, "found calendars", "user", user, "count", len(cc))
	found := false
	for _, c := range cc {
		if strings.HasSuffix(c.Path, fmt.Sprintf("/%s", oldName)) {
			found = true
			break
		}
	}
	if !found {
		slog.DebugContext(ctx, "old calendar not found, nothing to clean up", "user", user, "oldName", oldName)
		return nil
	}
	slog.DebugContext(ctx, "old calendar found, checking events", "user", user, "oldName", oldName)
	calendarPath := fmt.Sprintf("/SOGo/dav/%s/Calendar/%s", user, oldName)
	events, err := cl.QueryCalendar(ctx, calendarPath, &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name: "VCALENDAR",
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{
				Name: "VEVENT",
			}},
		},
	})
	if err != nil {
		return err
	}
	for _, ev := range events {
		prodID := ev.Data.Props.Get(ical.PropProductID)
		if prodID == nil || prodID.Value != ConstProductID {
			slog.WarnContext(ctx, "old calendar contains foreign events, skipping cleanup",
				"user", user, "calendar", oldName)
			return nil
		}
	}
	if err := cl.RemoveAll(ctx, calendarPath); err != nil {
		return fmt.Errorf("error removing old calendar: %w", err)
	}
	slog.InfoContext(ctx, "removed old birthday calendar", "user", user, "calendar", oldName)
	return nil
}

func (d *Daemon) ensureBirthdayCal(ctx context.Context, httpClient webdav.HTTPClient, user string) error {
	slog.DebugContext(ctx, "ensuring birthday calendar exists", "user", user, "calendar", d.calendarName)
	endpoint, err := url.JoinPath(d.baseURL, "SOGo/dav", user, "Calendar/")
	if err != nil {
		return err
	}
	cl, err := caldav.NewClient(httpClient, endpoint)
	if err != nil {
		return err
	}
	cc, err := cl.FindCalendars(ctx, "")
	if err != nil {
		return err
	}
	for _, c := range cc {
		if strings.HasSuffix(c.Path, fmt.Sprintf("/%s", d.calendarName)) {
			slog.DebugContext(ctx, "birthday calendar already exists", "user", user)
			return nil
		}
	}
	if err := cl.Mkdir(ctx, d.calendarName); err != nil {
		return err
	}
	slog.InfoContext(ctx, "created birthday calendar", "user", user, "calendar", d.calendarName)
	return nil
}

func (d *Daemon) syncBirthdaysToCal(ctx context.Context, httpClient webdav.HTTPClient, user string, birthdays []BirthdayContact) error {
	endpoint, err := url.JoinPath(d.baseURL, "SOGo/dav", user, "Calendar/")
	if err != nil {
		return err
	}
	cl, err := caldav.NewClient(httpClient, endpoint)
	if err != nil {
		return err
	}
	calendarPath := fmt.Sprintf("/SOGo/dav/%s/Calendar/%s", user, d.calendarName)
	events, err := cl.QueryCalendar(ctx, calendarPath, &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name: "VCALENDAR",
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{
				Name: "VEVENT",
			}},
			Start: time.Now().Add(time.Hour * 24 * 365 * -1).UTC(),
			End:   time.Now().Add(time.Hour * 24 * 365 * 100).UTC(),
		},
	})
	if err != nil {
		return err
	}
	slog.DebugContext(ctx, "queried existing calendar events", "user", user, "count", len(events))
	bevs := generateBirthdayEvents(birthdays, d.eventYears)
	slog.DebugContext(ctx, "generated birthday events", "user", user, "count", len(bevs))
	bevsInSync := make([]int, 0)
	driftedEvents := make([]string, 0)
	for _, ev := range events {
		matchedBev := false
		for _, v := range ev.Data.Children {
			for i, bev := range bevs {
				if icalMatchesBev(v, bev, d.notificationEnabled) {
					bevsInSync = append(bevsInSync, i)
					matchedBev = true
				}
			}
		}
		if !matchedBev {
			driftedEvents = append(driftedEvents, ev.Path)
		}
	}
	counterDelete, counterAdded := 0, 0
	for _, v := range driftedEvents {
		slog.DebugContext(ctx, "removing drifted calendar event", "user", user, "path", v)
		if err := cl.RemoveAll(ctx, v); err != nil {
			return err
		}
		counterDelete++
	}
	for i, v := range bevs {
		if slices.Contains(bevsInSync, i) {
			continue
		}
		slog.DebugContext(ctx, "adding calendar event", "user", user, "summary", v.Summary, "start", v.DateTimeStart)
		p, ic := v.generateICAL(calendarPath, d.notificationEnabled, d.notificationTrigger)
		_, err := cl.PutCalendarObject(ctx, p, ic)
		if err != nil {
			return err
		}
		counterAdded++
	}
	if (counterAdded + counterDelete) > 0 {
		slog.InfoContext(ctx, "synchronized birthday events", "user", user, "added", counterAdded, "removed", counterDelete)
	} else {
		slog.DebugContext(ctx, "birthday events already in sync", "user", user, "total", len(bevs))
	}
	return nil
}

type birthdayEvent struct {
	Summary       string
	DateTimeStart string
	DateTimeEnd   string
}

func generateBirthdayEvents(birthdays []BirthdayContact, eventYears int) []birthdayEvent {
	cyear := time.Now().Year()
	bb := make([]birthdayEvent, 0)
	for _, v := range birthdays {
		for year := cyear; year <= eventYears+cyear; year++ {
			yearshift := year - v.Date.Year()
			prefix := "\U0001F382 " // 🎂
			if v.Type == ContactTypeAnniversary {
				prefix = "\U0001F48D " // 💍
			}
			ev := birthdayEvent{
				Summary:       fmt.Sprintf("%s%s %s", prefix, v.GivenName, v.FamilyName),
				DateTimeStart: v.Date.AddDate(yearshift, 0, 0).Format("20060102"),
				DateTimeEnd:   v.Date.AddDate(yearshift, 0, 1).Format("20060102"),
			}
			if v.YearKnown {
				ev.Summary = fmt.Sprintf("%s (%d)", ev.Summary, yearshift)
			}
			bb = append(bb, ev)
		}
	}
	return bb
}

func icalMatchesBev(ic *ical.Component, bev birthdayEvent, notificationEnabled bool) bool {
	if ic.Props.Get(ical.PropSummary) == nil || ic.Props.Get(ical.PropSummary).Value != bev.Summary {
		return false
	}
	dtStart := ic.Props.Get(ical.PropDateTimeStart)
	if dtStart == nil || dtStart.Value != bev.DateTimeStart {
		return false
	}
	if dtStart.Params.Get(ical.ParamValue) != string(ical.ValueDate) {
		return false
	}
	dtEnd := ic.Props.Get(ical.PropDateTimeEnd)
	if dtEnd == nil || dtEnd.Value != bev.DateTimeEnd {
		return false
	}
	if dtEnd.Params.Get(ical.ParamValue) != string(ical.ValueDate) {
		return false
	}
	hasAlarm := false
	for _, child := range ic.Children {
		if child.Name == ical.CompAlarm {
			hasAlarm = true
			break
		}
	}
	if notificationEnabled != hasAlarm {
		return false
	}
	return true
}

func (bev birthdayEvent) generateICAL(calendar string, notificationEnabled bool, notificationTrigger string) (string, *ical.Calendar) {
	id := uuid.New().String()
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, ConstProductID)
	cal.Props.SetText(ical.PropVersion, "2.0")
	event := ical.NewComponent(ical.CompEvent)
	event.Props.SetText(ical.PropUID, id)
	event.Props.SetText(ical.PropSummary, bev.Summary)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Now())
	start := ical.NewProp(ical.PropDateTimeStart)
	start.SetValueType(ical.ValueDate)
	start.Value = bev.DateTimeStart
	end := ical.NewProp(ical.PropDateTimeEnd)
	end.SetValueType(ical.ValueDate)
	end.Value = bev.DateTimeEnd
	event.Props.Set(start)
	event.Props.Set(end)
	if notificationEnabled {
		alarm := ical.NewComponent(ical.CompAlarm)
		alarm.Props.SetText(ical.PropAction, "DISPLAY")
		alarm.Props.SetText(ical.PropDescription, bev.Summary)
		trigger := ical.NewProp(ical.PropTrigger)
		trigger.Params.Set(ical.ParamValue, string(ical.ValueDuration))
		trigger.Value = notificationTrigger
		alarm.Props.Set(trigger)
		event.Children = append(event.Children, alarm)
	}
	cal.Children = append(cal.Children, event)
	return fmt.Sprintf("%s/%s.ics", calendar, id), cal
}
