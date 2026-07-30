package web

import (
	"strings"
	"testing"
	"time"
)

func TestBuildICSCalendarIncludesCoreFields(t *testing.T) {
	events := []eventViewData{
		{
			EventID:   "EVT1",
			Heading:   "Training",
			StartAt:   time.Date(2099, 5, 26, 18, 0, 0, 0, time.UTC),
			EndAt:     time.Date(2099, 5, 26, 19, 0, 0, 0, time.UTC),
			GroupName: "Team A",
			Status:    "Accepted",
		},
	}

	ics := buildICSCalendar(events, time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC))

	if !strings.Contains(ics, "BEGIN:VCALENDAR") || !strings.Contains(ics, "END:VCALENDAR") {
		t.Fatalf("expected VCALENDAR envelope, got %q", ics)
	}

	if !strings.Contains(ics, "UID:EVT1") {
		t.Fatalf("expected event UID EVT1, got %q", ics)
	}

	if !strings.Contains(ics, "DTSTART:20990526T180000Z") {
		t.Fatalf("expected DTSTART, got %q", ics)
	}

	if !strings.Contains(ics, "DTEND:20990526T190000Z") {
		t.Fatalf("expected DTEND, got %q", ics)
	}

	if !strings.Contains(ics, "SUMMARY:Training") {
		t.Fatalf("expected SUMMARY, got %q", ics)
	}

	if !strings.Contains(ics, "CATEGORIES:Team A") {
		t.Fatalf("expected CATEGORIES with group name, got %q", ics)
	}

	if !strings.Contains(ics, "DESCRIPTION:Group: Team A\\nStatus: Accepted") {
		t.Fatalf("expected DESCRIPTION with group and status, got %q", ics)
	}
}
