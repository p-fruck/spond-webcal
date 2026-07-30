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
	SyncEnabled    bool
	LastSyncISO    string
	LastSyncLabel  string
	NextSyncISO    string
	NextSyncLabel  string
	GroupFilters   []groupFilterViewData
	StatusFilters  []statusFilterViewData
	UpcomingEvents []eventViewData
	PastEvents     []eventViewData
}

type groupFilterViewData struct {
	ID       string
	Name     string
	Selected bool
}

type statusFilterViewData struct {
	Value    string
	Label    string
	Selected bool
}

const (
	statusFilterCodeAccepted   = "0"
	statusFilterCodeDeclined   = "1"
	statusFilterCodeUnanswered = "2"
	statusFilterCodeOther      = "3"
)

type profilePageData struct {
	Name       string
	Email      string
	ProfileID  string
	GroupCount int
	ActorIDs   []string
	Groups     []groupSummaryData
}

type eventViewData struct {
	EventID   string
	Heading   string
	StartISO  string
	StartTime string
	StartAt   time.Time
	EndAt     time.Time
	GroupID   string
	GroupName string
	Status    string
	StatusKey string
	StatusCSS string
}

func buildEventViewData(events []api.Event, actorIDs []string, groupNamesByID, subGroupParentByID map[string]string, now time.Time) ([]eventViewData, []eventViewData) {
	upcoming := make([]eventViewData, 0, len(events))
	past := make([]eventViewData, 0, len(events))

	for _, event := range events {
		status, statusCSS := responseStatusForActors(event.Responses, actorIDs)
		statusKey := statusFilterKeyFromCSS(statusCSS)

		groupID := eventGroupID(event, subGroupParentByID)
		groupName := "No group"
		if groupID != "" {
			groupName = groupID
			if name, ok := groupNamesByID[groupID]; ok && strings.TrimSpace(name) != "" {
				groupName = name
			}
		}

		item := eventViewData{
			EventID:   event.Id,
			Heading:   event.Heading,
			StartISO:  event.StartTimestamp.UTC().Format(time.RFC3339),
			StartTime: event.StartTimestamp.UTC().Format("2006-01-02 15:04 UTC"),
			StartAt:   event.StartTimestamp,
			EndAt:     event.EndTimestamp,
			GroupID:   groupID,
			GroupName: groupName,
			Status:    status,
			StatusKey: statusKey,
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

func eventGroupID(event api.Event, subGroupParentByID map[string]string) string {
	if event.GroupId != nil {
		groupID := strings.TrimSpace(*event.GroupId)
		if groupID != "" {
			return groupID
		}
	}

	if event.SubGroupId != nil {
		subGroupID := strings.TrimSpace(*event.SubGroupId)
		if subGroupID == "" {
			return ""
		}

		if parentGroupID, ok := subGroupParentByID[subGroupID]; ok && strings.TrimSpace(parentGroupID) != "" {
			return parentGroupID
		}
	}

	return ""
}

func buildGroupNamesByID(groups []api.Group) map[string]string {
	items := make(map[string]string, len(groups))
	for _, group := range groups {
		items[group.Id] = strings.TrimSpace(group.Name)
	}

	return items
}

func buildSubGroupParentByID(groups []api.Group) map[string]string {
	items := map[string]string{}
	for _, group := range groups {
		if group.SubGroups == nil {
			continue
		}

		for _, subGroup := range *group.SubGroups {
			if subGroup.Id == nil {
				continue
			}

			subGroupID := strings.TrimSpace(*subGroup.Id)
			if subGroupID == "" {
				continue
			}

			items[subGroupID] = strings.TrimSpace(group.Id)
		}
	}

	return items
}

func normalizeFilterValues(values []string) map[string]struct{} {
	selected := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		selected[trimmed] = struct{}{}
	}

	return selected
}

func buildGroupFilterViewData(groups []api.Group, selected map[string]struct{}) []groupFilterViewData {
	items := make([]groupFilterViewData, 0, len(groups))
	for _, group := range groups {
		_, isSelected := selected[group.Id]
		items = append(items, groupFilterViewData{
			ID:       group.Id,
			Name:     strings.TrimSpace(group.Name),
			Selected: isSelected,
		})
	}

	return items
}

func buildStatusFilterViewData(selected map[string]struct{}) []statusFilterViewData {
	type option struct {
		Code  string
		Key   string
		Label string
	}

	allOptions := []option{
		{Code: statusFilterCodeAccepted, Key: "accepted", Label: "Accepted"},
		{Code: statusFilterCodeDeclined, Key: "declined", Label: "Declined"},
		{Code: statusFilterCodeUnanswered, Key: "unanswered", Label: "Not answered"},
		{Code: statusFilterCodeOther, Key: "other", Label: "Other"},
	}

	all := make([]statusFilterViewData, 0, len(allOptions))
	for _, item := range allOptions {
		_, isSelected := selected[item.Key]
		all = append(all, statusFilterViewData{
			Value:    item.Code,
			Label:    item.Label,
			Selected: isSelected,
		})
	}

	return all
}

func normalizeStatusFilterValues(values []string) map[string]struct{} {
	selected := map[string]struct{}{}
	for _, value := range values {
		key, ok := statusFilterKeyFromFilterValue(value)
		if !ok {
			continue
		}

		selected[key] = struct{}{}
	}

	return selected
}

func statusFilterKeyFromFilterValue(value string) (string, bool) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case statusFilterCodeAccepted, "accepted":
		return "accepted", true
	case statusFilterCodeDeclined, "declined":
		return "declined", true
	case statusFilterCodeUnanswered, "unanswered":
		return "unanswered", true
	case statusFilterCodeOther, "other":
		return "other", true
	default:
		return "", false
	}
}

func filterEventViewData(items []eventViewData, selectedGroupIDs, selectedStatuses map[string]struct{}) []eventViewData {
	if len(selectedGroupIDs) == 0 && len(selectedStatuses) == 0 {
		return items
	}

	filtered := make([]eventViewData, 0, len(items))
	for _, item := range items {
		if len(selectedGroupIDs) > 0 {
			if _, ok := selectedGroupIDs[item.GroupID]; !ok {
				continue
			}
		}

		if len(selectedStatuses) > 0 {
			if _, ok := selectedStatuses[item.StatusKey]; !ok {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	return filtered
}

func statusFilterKeyFromCSS(statusCSS string) string {
	trimmed := strings.TrimSpace(statusCSS)
	switch trimmed {
	case "accepted", "declined", "unanswered":
		return trimmed
	default:
		return "other"
	}
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
