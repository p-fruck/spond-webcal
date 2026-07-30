package web

import (
	"encoding/json"
	"strings"
	"time"

	"code.p-fruck.eu/spond-webcal/internal/db"
)

type accessTokenListPageData struct {
	Name             string
	Email            string
	CreatedToken     string
	CreatedExportURL string
	Tokens           []accessTokenListItem
}

type accessTokenListItem struct {
	ID        uint
	Category  string
	CreatedAt string
	ExpiresAt string
	IsExpired bool
	Rules     []accessTokenRuleView
}

type accessTokenRuleView struct {
	GroupID     string
	Statuses    string
	IncludePast bool
}

type accessTokenCreatePageData struct {
	Name       string
	Email      string
	Groups     []groupSummaryData
	Error      string
	Expiration string
}

func buildAccessTokenListItems(records []db.AccessTokenRecord) []accessTokenListItem {
	items := make([]accessTokenListItem, 0, len(records))
	for _, record := range records {
		rules := []accessTokenRuleView{}

		if strings.TrimSpace(strings.ToLower(record.Category)) == accessTokenCategoryICal {
			var scope iCalAccessScope
			if err := json.Unmarshal([]byte(record.Scope), &scope); err == nil {
				for _, groupRule := range scope.Groups {
					statuses := make([]string, 0, len(groupRule.Statuses))
					for _, status := range groupRule.Statuses {
						statuses = append(statuses, statusLabelFromKey(status))
					}

					rules = append(rules, accessTokenRuleView{
						GroupID:     groupRule.GroupID,
						Statuses:    strings.Join(statuses, ", "),
						IncludePast: groupRule.IncludePast,
					})
				}
			}
		}

		createdAt := "-"
		if !record.CreatedAt.IsZero() {
			createdAt = record.CreatedAt.UTC().Format(time.RFC3339)
		}

		expiresAt := "Never"
		expired := false
		if record.ExpiresAt != nil {
			expiresAt = record.ExpiresAt.UTC().Format(time.RFC3339)
			expired = !record.ExpiresAt.After(time.Now().UTC())
		}

		items = append(items, accessTokenListItem{
			ID:        record.ID,
			Category:  record.Category,
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
			IsExpired: expired,
			Rules:     rules,
		})
	}

	return items
}

func statusLabelFromKey(key string) string {
	switch strings.TrimSpace(strings.ToLower(key)) {
	case "accepted":
		return "Accepted"
	case "declined":
		return "Declined"
	case "unanswered":
		return "Not answered"
	case "other":
		return "Other"
	default:
		return key
	}
}
