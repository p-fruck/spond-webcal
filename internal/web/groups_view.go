package web

import (
	"fmt"
	"strings"

	"code.p-fruck.eu/spond-webcal/internal/api"
)

type groupSummaryData struct {
	ID          string
	Name        string
	MemberCount int
}

type groupMemberData struct {
	MemberID  string
	ProfileID string
	Name      string
	Email     string
}

type groupDetailData struct {
	ID        string
	Name      string
	Members   []groupMemberData
	SubGroups int
}

type groupDetailPageData struct {
	Group groupDetailData
}

func buildGroupSummaryData(groups []api.Group) []groupSummaryData {
	items := make([]groupSummaryData, 0, len(groups))
	for _, group := range groups {
		items = append(items, groupSummaryData{
			ID:          group.Id,
			Name:        group.Name,
			MemberCount: len(group.Members),
		})
	}

	return items
}

func findGroupByID(groups []api.Group, groupID string) (api.Group, bool) {
	for _, group := range groups {
		if group.Id == groupID {
			return group, true
		}
	}

	return api.Group{}, false
}

func buildGroupDetailData(group api.Group) groupDetailData {
	members := make([]groupMemberData, 0, len(group.Members))
	for _, member := range group.Members {
		email := ""
		if member.Email != nil {
			email = string(*member.Email)
		}

		profileID := ""
		if member.Profile != nil && member.Profile.Id != nil {
			profileID = *member.Profile.Id
		}

		members = append(members, groupMemberData{
			MemberID:  member.Id,
			ProfileID: profileID,
			Name:      fullNameFromProfile(member.FirstName, member.LastName, member.Id),
			Email:     email,
		})
	}

	subGroupCount := 0
	if group.SubGroups != nil {
		subGroupCount = len(*group.SubGroups)
	}

	return groupDetailData{
		ID:        group.Id,
		Name:      strings.TrimSpace(group.Name),
		Members:   members,
		SubGroups: subGroupCount,
	}
}

func groupNotFoundError(groupID string) string {
	return fmt.Sprintf("group %q not found", groupID)
}
