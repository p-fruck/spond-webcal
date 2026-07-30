package web

import (
	"fmt"
	"strings"
)

const accessTokenCategoryICal = "ical"

type createAccessTokenRequest struct {
	Category  string                   `json:"category"`
	Groups    []createAccessTokenGroup `json:"groups"`
	ExpiresAt string                   `json:"expiresAt"`
}

type createAccessTokenGroup struct {
	GroupID     string   `json:"groupId"`
	Statuses    []string `json:"statuses"`
	IncludePast bool     `json:"includePast"`
}

type iCalAccessScope struct {
	ActorIDs []string               `json:"actorIds"`
	Groups   []iCalAccessScopeGroup `json:"groups"`
}

type iCalAccessScopeGroup struct {
	GroupID     string   `json:"groupId"`
	Statuses    []string `json:"statuses"`
	IncludePast bool     `json:"includePast"`
}

func normalizeCreateAccessTokenRequest(req createAccessTokenRequest, actorIDs []string) (string, iCalAccessScope, error) {
	category := strings.TrimSpace(strings.ToLower(req.Category))
	if category == "" {
		category = accessTokenCategoryICal
	}

	if category != accessTokenCategoryICal {
		return "", iCalAccessScope{}, fmt.Errorf("unsupported access token category %q", category)
	}

	if len(req.Groups) == 0 {
		return "", iCalAccessScope{}, fmt.Errorf("at least one group rule is required")
	}

	normalizedGroups := make([]iCalAccessScopeGroup, 0, len(req.Groups))
	seenGroupIDs := map[string]struct{}{}
	for _, group := range req.Groups {
		groupID := strings.TrimSpace(group.GroupID)
		if groupID == "" {
			return "", iCalAccessScope{}, fmt.Errorf("groupId is required")
		}
		if _, exists := seenGroupIDs[groupID]; exists {
			return "", iCalAccessScope{}, fmt.Errorf("groupId %q is duplicated", groupID)
		}
		seenGroupIDs[groupID] = struct{}{}

		if len(group.Statuses) == 0 {
			return "", iCalAccessScope{}, fmt.Errorf("group %q requires at least one status", groupID)
		}

		statusSet := map[string]struct{}{}
		for _, status := range group.Statuses {
			normalized, ok := statusFilterKeyFromFilterValue(status)
			if !ok {
				return "", iCalAccessScope{}, fmt.Errorf("group %q includes unsupported status %q", groupID, status)
			}
			statusSet[normalized] = struct{}{}
		}

		normalizedStatuses := make([]string, 0, len(statusSet))
		for _, key := range []string{"accepted", "declined", "unanswered", "other"} {
			if _, ok := statusSet[key]; ok {
				normalizedStatuses = append(normalizedStatuses, key)
			}
		}

		normalizedGroups = append(normalizedGroups, iCalAccessScopeGroup{
			GroupID:     groupID,
			Statuses:    normalizedStatuses,
			IncludePast: group.IncludePast,
		})
	}

	normalizedActorIDs := normalizeFilterList(actorIDs)
	if len(normalizedActorIDs) == 0 {
		return "", iCalAccessScope{}, fmt.Errorf("actor ids are required for iCal scope")
	}

	return category, iCalAccessScope{ActorIDs: normalizedActorIDs, Groups: normalizedGroups}, nil
}

func filterEventsByICalScope(upcoming, past []eventViewData, scope iCalAccessScope) []eventViewData {
	rules := map[string]iCalAccessScopeGroup{}
	for _, group := range scope.Groups {
		rules[group.GroupID] = group
	}

	matches := func(event eventViewData, includePast bool) bool {
		rule, ok := rules[event.GroupID]
		if !ok {
			return false
		}
		if !includePast && !rule.IncludePast {
			return false
		}

		allowedStatuses := map[string]struct{}{}
		for _, status := range rule.Statuses {
			allowedStatuses[status] = struct{}{}
		}

		_, ok = allowedStatuses[event.StatusKey]
		return ok
	}

	filteredUpcoming := make([]eventViewData, 0, len(upcoming))
	for _, event := range upcoming {
		if matches(event, true) {
			filteredUpcoming = append(filteredUpcoming, event)
		}
	}

	filteredPast := make([]eventViewData, 0, len(past))
	for _, event := range past {
		if matches(event, false) {
			filteredPast = append(filteredPast, event)
		}
	}

	return mergedSortedEvents(filteredUpcoming, filteredPast)
}
