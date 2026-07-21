package web

import (
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"code.p-fruck.eu/spond-webcal/internal/api"
)

func TestResponseStatusForProfile(t *testing.T) {
	accepted := []string{"U1"}
	declined := []string{"U2"}
	waiting := []string{"U3"}
	unconfirmed := []string{"U4"}
	unanswered := []string{"U5"}

	tests := []struct {
		name      string
		responses *api.EventResponse
		actorIDs  []string
		want      string
		wantCSS   string
	}{
		{
			name: "accepted",
			responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
			actorIDs: []string{"U1"},
			want:     "Accepted",
			wantCSS:  "accepted",
		},
		{
			name: "declined",
			responses: &api.EventResponse{
				DeclinedIds: &declined,
			},
			actorIDs: []string{"U2"},
			want:     "Declined",
			wantCSS:  "declined",
		},
		{
			name: "waiting list",
			responses: &api.EventResponse{
				WaitinglistIds: &waiting,
			},
			actorIDs: []string{"U3"},
			want:     "Waiting list",
			wantCSS:  "waiting",
		},
		{
			name: "unconfirmed",
			responses: &api.EventResponse{
				UnconfirmedIds: &unconfirmed,
			},
			actorIDs: []string{"U4"},
			want:     "Unconfirmed",
			wantCSS:  "unconfirmed",
		},
		{
			name: "unanswered",
			responses: &api.EventResponse{
				UnansweredIds: &unanswered,
			},
			actorIDs: []string{"U5"},
			want:     "Unanswered",
			wantCSS:  "unanswered",
		},
		{
			name:      "unknown when missing",
			responses: nil,
			actorIDs:  []string{"U1"},
			want:      "Unknown",
			wantCSS:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotCSS := responseStatusForActors(tt.responses, tt.actorIDs)
			if got != tt.want || gotCSS != tt.wantCSS {
				t.Fatalf("expected (%q, %q), got (%q, %q)", tt.want, tt.wantCSS, got, gotCSS)
			}
		})
	}
}

func TestBuildEventViewDataSplitsAndSorts(t *testing.T) {
	accepted := []string{"U1"}
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	groupNames := map[string]string{"G1": "Team A"}
	subGroupParent := map[string]string{}
	groupID := "G1"

	events := []api.Event{
		{
			Heading:        "Older past event",
			GroupId:        &groupID,
			StartTimestamp: time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
		{
			Heading:        "Recent past event",
			GroupId:        &groupID,
			StartTimestamp: time.Date(2026, 6, 15, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
		{
			Heading:        "Later upcoming",
			GroupId:        &groupID,
			StartTimestamp: time.Date(2026, 6, 22, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
		{
			Heading:        "Soon upcoming",
			GroupId:        &groupID,
			StartTimestamp: time.Date(2026, 6, 17, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
	}

	upcoming, past := buildEventViewData(events, []string{"U1"}, groupNames, subGroupParent, now)
	if len(upcoming) != 2 {
		t.Fatalf("expected 2 upcoming items, got %d", len(upcoming))
	}

	if len(past) != 2 {
		t.Fatalf("expected 2 past items, got %d", len(past))
	}

	if upcoming[0].Heading != "Soon upcoming" || upcoming[1].Heading != "Later upcoming" {
		t.Fatalf("expected upcoming items sorted ascending by start, got %q then %q", upcoming[0].Heading, upcoming[1].Heading)
	}

	if past[0].Heading != "Older past event" || past[1].Heading != "Recent past event" {
		t.Fatalf("expected past items sorted ascending by start, got %q then %q", past[0].Heading, past[1].Heading)
	}

	if upcoming[0].Status != "Accepted" || upcoming[0].StatusCSS != "accepted" {
		t.Fatalf("expected accepted status, got (%q, %q)", upcoming[0].Status, upcoming[0].StatusCSS)
	}

	if upcoming[0].StartTime == "" {
		t.Fatal("expected formatted start time")
	}

	if upcoming[0].StartISO == "" {
		t.Fatal("expected start ISO timestamp")
	}

	if upcoming[0].GroupName != "Team A" {
		t.Fatalf("expected group name Team A, got %q", upcoming[0].GroupName)
	}
}

func TestBuildEventViewDataResolvesSubGroupToParentGroup(t *testing.T) {
	accepted := []string{"U1"}
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	groupNames := map[string]string{"G1": "Team A"}
	subGroupParent := map[string]string{"SG1": "G1"}
	subGroupID := "SG1"

	events := []api.Event{
		{
			Heading:        "Subgroup event",
			SubGroupId:     &subGroupID,
			StartTimestamp: time.Date(2026, 6, 17, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
	}

	upcoming, _ := buildEventViewData(events, []string{"U1"}, groupNames, subGroupParent, now)
	if len(upcoming) != 1 {
		t.Fatalf("expected 1 upcoming item, got %d", len(upcoming))
	}

	if upcoming[0].GroupID != "G1" {
		t.Fatalf("expected parent group ID G1, got %q", upcoming[0].GroupID)
	}

	if upcoming[0].GroupName != "Team A" {
		t.Fatalf("expected parent group name Team A, got %q", upcoming[0].GroupName)
	}
}

func TestFilterEventViewData(t *testing.T) {
	items := []eventViewData{
		{Heading: "A", GroupID: "G1", StatusKey: "accepted"},
		{Heading: "B", GroupID: "G2", StatusKey: "declined"},
		{Heading: "C", GroupID: "G1", StatusKey: "unanswered"},
	}

	groups := map[string]struct{}{"G1": {}}
	statuses := map[string]struct{}{"accepted": {}, "unanswered": {}}

	filtered := filterEventViewData(items, groups, statuses)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered events, got %d", len(filtered))
	}

	if filtered[0].Heading != "A" || filtered[1].Heading != "C" {
		t.Fatalf("unexpected filtered order/content: %+v", filtered)
	}
}

func TestNormalizeStatusFilterValues(t *testing.T) {
	selected := normalizeStatusFilterValues([]string{"0", "2", "declined", "other", "unknown-value"})

	if len(selected) != 4 {
		t.Fatalf("expected 4 known status values, got %d", len(selected))
	}

	if _, ok := selected["accepted"]; !ok {
		t.Fatalf("expected accepted to be selected, got %+v", selected)
	}

	if _, ok := selected["declined"]; !ok {
		t.Fatalf("expected declined to be selected, got %+v", selected)
	}

	if _, ok := selected["unanswered"]; !ok {
		t.Fatalf("expected unanswered to be selected, got %+v", selected)
	}

	if _, ok := selected["other"]; !ok {
		t.Fatalf("expected other to be selected, got %+v", selected)
	}
}

func TestResponseActorIDsUsesGroupMembership(t *testing.T) {
	email := "ada@example.com"
	profile := api.Profile{Id: "P1", FirstName: "Ada", LastName: "Lovelace"}
	memberEmail := openapi_types.Email(email)
	groups := []api.Group{
		{
			Id:   "G1",
			Name: "Team",
			Members: []api.Member{
				{
					Id:        "M1",
					FirstName: "Ada",
					LastName:  "Lovelace",
					Email:     &memberEmail,
				},
			},
		},
	}

	actorIDs := responseActorIDs(profile, email, groups)
	if !containsString(actorIDs, "P1") {
		t.Fatalf("expected actor IDs to include profile id P1, got %+v", actorIDs)
	}

	if !containsString(actorIDs, "M1") {
		t.Fatalf("expected actor IDs to include member id M1, got %+v", actorIDs)
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}

	return false
}
