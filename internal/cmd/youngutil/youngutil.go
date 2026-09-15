// Package youngutil contains the small amount of transport and date handling
// shared by Young catalog and workspace commands.
package youngutil

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
)

const (
	EventsPath                      = "/api/catalog/young-events"
	OrganizersPath                  = "/api/catalog/young-organizers"
	YoungEventSubscriptionsPath     = "/api/workspace/young-event-subscriptions"
	YoungOrganizerSubscriptionsPath = "/api/workspace/young-organizer-subscriptions"
	YoungNotificationsPath          = "/api/workspace/young-notifications"
	PersonalCalendarEventsPath      = "/api/workspace/calendar/events"
)

var Shanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

// PathID appends an opaque public identifier to a REST path safely.
func PathID(base, id string) string {
	return strings.TrimRight(base, "/") + "/" + url.PathEscape(id)
}

// PageParams validates CLI pagination flags and maps --limit to pageSize.
func PageParams(page, limit int) (url.Values, error) {
	if page < 0 {
		return nil, fmt.Errorf("--page must be zero or greater")
	}
	if limit < 0 {
		return nil, fmt.Errorf("--limit must be zero or greater")
	}
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		params.Set("pageSize", strconv.Itoa(limit))
	}
	return params, nil
}

func cloneValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, items := range values {
		clone[key] = append([]string(nil), items...)
	}
	return clone
}

// FetchAllPages reads every page of a standard {data, pagination} response.
// The returned response is rewritten as one complete page so table, JSON and
// jq output all describe the same result.
func FetchAllPages(
	ctx context.Context,
	client *api.Client,
	path string,
	params url.Values,
	key string,
	pageSize int,
	identityKey string,
) (any, error) {
	if pageSize < 1 {
		return nil, fmt.Errorf("page size must be positive")
	}

	firstParams := cloneValues(params)
	firstParams.Set("page", "1")
	firstParams.Set("pageSize", strconv.Itoa(pageSize))
	first, err := client.DoJSON(ctx, "GET", path, firstParams, nil)
	if err != nil {
		return nil, err
	}
	total, totalPages, err := pagination(first)
	if err != nil {
		return nil, err
	}
	firstResult := cmdutil.NewListResult(first, key)
	if len(firstResult.Rows) > pageSize {
		return nil, fmt.Errorf("%s returned an oversized page", path)
	}
	rows := append([]map[string]any(nil), firstResult.Rows...)
	seen := make(map[string]struct{}, len(rows))
	if identityKey != "" {
		if err := collectIDs(seen, rows, identityKey, path); err != nil {
			return nil, err
		}
	}

	if totalPages < 0 || totalPages > 10000 {
		return nil, fmt.Errorf("%s returned an invalid page count", path)
	}
	if total == 0 {
		if len(rows) != 0 || totalPages > 1 {
			return nil, fmt.Errorf("%s returned inconsistent empty pagination", path)
		}
	} else {
		if totalPages < 1 {
			return nil, fmt.Errorf("%s returned no pages for %d results", path, total)
		}
		for page := 2; page <= totalPages; page++ {
			pageParams := cloneValues(params)
			pageParams.Set("page", strconv.Itoa(page))
			pageParams.Set("pageSize", strconv.Itoa(pageSize))
			payload, fetchErr := client.DoJSON(ctx, "GET", path, pageParams, nil)
			if fetchErr != nil {
				return nil, fetchErr
			}
			pageTotal, pageCount, pageErr := pagination(payload)
			if pageErr != nil {
				return nil, pageErr
			}
			if pageTotal != total || pageCount != totalPages {
				return nil, fmt.Errorf("%s changed pagination between pages", path)
			}
			pageResult := cmdutil.NewListResult(payload, key)
			if len(pageResult.Rows) > pageSize {
				return nil, fmt.Errorf("%s returned an oversized page", path)
			}
			if identityKey != "" {
				if err := collectIDs(seen, pageResult.Rows, identityKey, path); err != nil {
					return nil, err
				}
			}
			rows = append(rows, pageResult.Rows...)
		}
	}
	if len(rows) != total {
		return nil, fmt.Errorf("%s returned %d of %d results", path, len(rows), total)
	}

	result := cmdutil.NewListResult(first, key)
	result.Raw = cmdutil.WithListRows(first, key, rows, total, 1)
	if m := cmdutil.AsMap(result.Raw); m != nil {
		if pg := cmdutil.AsMap(m["pagination"]); pg != nil {
			pg["page"] = 1
			resultPageSize := len(rows)
			if resultPageSize == 0 {
				resultPageSize = pageSize
			}
			pg["pageSize"] = resultPageSize
			pg["total"] = total
			pg["totalPages"] = 1
		}
	}
	return result.Raw, nil
}

func pagination(data any) (total, totalPages int, err error) {
	m := cmdutil.AsMap(data)
	if m == nil {
		return 0, 0, fmt.Errorf("paginated response is not an object")
	}
	pg := cmdutil.AsMap(m["pagination"])
	if pg == nil {
		return 0, 0, fmt.Errorf("paginated response has no pagination")
	}
	total, ok := integer(pg["total"])
	if !ok || total < 0 {
		return 0, 0, fmt.Errorf("paginated response has an invalid total")
	}
	totalPages, ok = integer(pg["totalPages"])
	if !ok || totalPages < 0 {
		return 0, 0, fmt.Errorf("paginated response has an invalid totalPages")
	}
	return total, totalPages, nil
}

func integer(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		if v != float64(int(v)) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

func collectIDs(seen map[string]struct{}, rows []map[string]any, key, path string) error {
	for _, row := range rows {
		value, ok := row[key]
		if !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return fmt.Errorf("%s returned a row without %s", path, key)
		}
		id := fmt.Sprint(value)
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%s returned duplicate %s %q", path, key, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

type DateView string

const (
	DateDay   DateView = "day"
	DateWeek  DateView = "week"
	DateMonth DateView = "month"
)

// DateRange returns inclusive Shanghai calendar-date bounds for an anchor.
// An empty anchor means today in Asia/Shanghai.
func DateRange(view DateView, anchor string, now time.Time) (string, string, error) {
	date, err := anchorDate(anchor, now)
	if err != nil {
		return "", "", err
	}
	switch view {
	case DateDay:
		formatted := formatDate(date)
		return formatted, formatted, nil
	case DateWeek:
		offset := (int(date.Weekday()) + 6) % 7
		start := date.AddDate(0, 0, -offset)
		return formatDate(start), formatDate(start.AddDate(0, 0, 6)), nil
	case DateMonth:
		start := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, Shanghai)
		end := start.AddDate(0, 1, -1)
		return formatDate(start), formatDate(end), nil
	default:
		return "", "", fmt.Errorf("unsupported date view %q", view)
	}
}

func anchorDate(anchor string, now time.Time) (time.Time, error) {
	if strings.TrimSpace(anchor) == "" {
		now = now.In(Shanghai)
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, Shanghai), nil
	}
	date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(anchor), Shanghai)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD)", anchor)
	}
	return date, nil
}

func formatDate(date time.Time) string {
	return date.In(Shanghai).Format("2006-01-02")
}
