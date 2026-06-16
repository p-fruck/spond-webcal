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

	events := []api.Event{
		{
			Heading:        "Older past event",
			StartTimestamp: time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
		{
			Heading:        "Recent past event",
			StartTimestamp: time.Date(2026, 6, 15, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
		{
			Heading:        "Later upcoming",
			StartTimestamp: time.Date(2026, 6, 22, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
		{
			Heading:        "Soon upcoming",
			StartTimestamp: time.Date(2026, 6, 17, 18, 0, 0, 0, time.UTC),
			Responses: &api.EventResponse{
				AcceptedIds: &accepted,
			},
		},
	}

	upcoming, past := buildEventViewData(events, []string{"U1"}, now)
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
