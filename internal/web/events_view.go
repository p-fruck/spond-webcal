package web

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"code.p-fruck.eu/spond-webcal/internal/api"
)

type accountPageData struct {
	Name           string
	Email          string
	UpcomingEvents []eventViewData
	PastEvents     []eventViewData
}

type profilePageData struct {
	Name       string
	Email      string
	ProfileID  string
	GroupCount int
	ActorIDs   []string
}

type eventViewData struct {
	Heading   string
	StartISO  string
	StartTime string
	StartAt   time.Time
	Status    string
	StatusCSS string
}

func buildEventViewData(events []api.Event, actorIDs []string, now time.Time) ([]eventViewData, []eventViewData) {
	upcoming := make([]eventViewData, 0, len(events))
	past := make([]eventViewData, 0, len(events))

	for _, event := range events {
		status, statusCSS := responseStatusForActors(event.Responses, actorIDs)
		item := eventViewData{
			Heading:   event.Heading,
			StartISO:  event.StartTimestamp.UTC().Format(time.RFC3339),
			StartTime: event.StartTimestamp.UTC().Format("2006-01-02 15:04 UTC"),
			StartAt:   event.StartTimestamp,
			Status:    status,
			StatusCSS: statusCSS,
		}

		if event.StartTimestamp.Before(now) {
			past = append(past, item)
			continue
		}

		upcoming = append(upcoming, item)
	}

	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].StartAt.Before(upcoming[j].StartAt)
	})

	sort.Slice(past, func(i, j int) bool {
		return past[i].StartAt.Before(past[j].StartAt)
	})

	return upcoming, past
}

func responseStatusForActors(responses *api.EventResponse, actorIDs []string) (string, string) {
	if responses == nil || len(actorIDs) == 0 {
		return "Unknown", "unknown"
	}

	if includesAnyID(responses.AcceptedIds, actorIDs) {
		return "Accepted", "accepted"
	}

	if includesAnyID(responses.DeclinedIds, actorIDs) {
		return "Declined", "declined"
	}

	if includesAnyID(responses.WaitinglistIds, actorIDs) {
		return "Waiting list", "waiting"
	}

	if includesAnyID(responses.UnconfirmedIds, actorIDs) {
		return "Unconfirmed", "unconfirmed"
	}

	if includesAnyID(responses.UnansweredIds, actorIDs) {
		return "Unanswered", "unanswered"
	}

	return "Unknown", "unknown"
}

func includesAnyID(ids *[]string, actorIDs []string) bool {
	if ids == nil {
		return false
	}

	actorSet := make(map[string]struct{}, len(actorIDs))
	for _, id := range actorIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		actorSet[trimmed] = struct{}{}
	}

	for _, id := range *ids {
		if _, ok := actorSet[id]; ok {
			return true
		}
	}

	return false
}

func responseActorIDs(profile api.Profile, email string, groups []api.Group) []string {
	actorSet := map[string]struct{}{}
	addActorID(actorSet, profile.Id)

	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	for _, group := range groups {
		for _, member := range group.Members {
			memberEmail := ""
			if member.Email != nil {
				memberEmail = strings.TrimSpace(strings.ToLower(string(*member.Email)))
			}

			profileIDMatches := false
			if member.Profile != nil && member.Profile.Id != nil && *member.Profile.Id == profile.Id {
				profileIDMatches = true
			}

			if memberEmail == normalizedEmail || profileIDMatches || member.Id == profile.Id {
				addActorID(actorSet, member.Id)
				if member.Profile != nil && member.Profile.Id != nil {
					addActorID(actorSet, *member.Profile.Id)
				}
			}
		}
	}

	ids := make([]string, 0, len(actorSet))
	for id := range actorSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids
}

func addActorID(actorSet map[string]struct{}, id string) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return
	}

	actorSet[trimmed] = struct{}{}
}

func fullNameFromProfile(firstName, lastName, fallbackEmail string) string {
	fullName := strings.TrimSpace(fmt.Sprintf("%s %s", firstName, lastName))
	if fullName == "" {
		return fallbackEmail
	}

	return fullName
}
