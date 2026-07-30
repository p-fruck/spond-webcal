package web

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func mergedSortedEvents(upcoming, past []eventViewData) []eventViewData {
	all := make([]eventViewData, 0, len(upcoming)+len(past))
	all = append(all, upcoming...)
	all = append(all, past...)

	sort.Slice(all, func(i, j int) bool {
		return all[i].StartAt.Before(all[j].StartAt)
	})

	return all
}

func includePastEvents(past []eventViewData, includePast bool) []eventViewData {
	if !includePast {
		return nil
	}

	return past
}

func buildICSCalendar(events []eventViewData, generatedAt time.Time) string {
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//spond-webcal//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
	}

	dtStamp := icsDateTimeUTC(generatedAt)
	for _, event := range events {
		uid := strings.TrimSpace(event.EventID)
		if uid == "" {
			uid = fmt.Sprintf("%d-%s@spond-webcal", event.StartAt.UTC().Unix(), sanitizeUID(event.Heading))
		}

		descriptionParts := []string{}
		if strings.TrimSpace(event.GroupName) != "" {
			descriptionParts = append(descriptionParts, "Group: "+event.GroupName)
		}
		if strings.TrimSpace(event.Status) != "" {
			descriptionParts = append(descriptionParts, "Status: "+event.Status)
		}

		lines = append(lines,
			"BEGIN:VEVENT",
			"UID:"+icsEscape(uid),
			"DTSTAMP:"+dtStamp,
			"DTSTART:"+icsDateTimeUTC(event.StartAt),
			"DTEND:"+icsDateTimeUTC(event.EndAt),
			"SUMMARY:"+icsEscape(event.Heading),
		)

		if len(descriptionParts) > 0 {
			lines = append(lines, "DESCRIPTION:"+icsEscape(strings.Join(descriptionParts, "\n")))
		}

		if strings.TrimSpace(event.GroupName) != "" {
			lines = append(lines, "CATEGORIES:"+icsEscape(event.GroupName))
		}

		lines = append(lines, "END:VEVENT")
	}

	lines = append(lines, "END:VCALENDAR")
	return strings.Join(lines, "\r\n") + "\r\n"
}

func icsDateTimeUTC(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func icsEscape(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\r\n", "\\n",
		"\n", "\\n",
		"\r", "\\n",
	)

	return replacer.Replace(strings.TrimSpace(value))
}

func sanitizeUID(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "event"
	}

	builder := strings.Builder{}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	return strings.Trim(builder.String(), "-")
}
