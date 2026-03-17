package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronSpec struct {
	minute int
	hour   int
	dom    int
	month  int
	dow    int
	hasDOM bool
	hasMon bool
	hasDOW bool
}

func parseCron(expr string) (cronSpec, error) {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return cronSpec{}, fmt.Errorf("cron must have 5 fields")
	}
	parsePart := func(raw string, min, max int) (int, bool, error) {
		value := strings.TrimSpace(raw)
		if value == "*" {
			return 0, false, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < min || n > max {
			return 0, false, fmt.Errorf("invalid cron field %q", raw)
		}
		return n, true, nil
	}
	minute, _, err := parsePart(parts[0], 0, 59)
	if err != nil {
		return cronSpec{}, err
	}
	hour, _, err := parsePart(parts[1], 0, 23)
	if err != nil {
		return cronSpec{}, err
	}
	dom, hasDOM, err := parsePart(parts[2], 1, 31)
	if err != nil {
		return cronSpec{}, err
	}
	month, hasMon, err := parsePart(parts[3], 1, 12)
	if err != nil {
		return cronSpec{}, err
	}
	dow, hasDOW, err := parsePart(parts[4], 0, 6)
	if err != nil {
		return cronSpec{}, err
	}
	return cronSpec{minute: minute, hour: hour, dom: dom, month: month, dow: dow, hasDOM: hasDOM, hasMon: hasMon, hasDOW: hasDOW}, nil
}

func nextRunFromExpr(expr string, now time.Time, loc *time.Location) (time.Time, error) {
	spec, err := parseCron(expr)
	if err != nil {
		return time.Time{}, err
	}
	if loc == nil {
		loc = time.UTC
	}
	base := now.In(loc).Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 366*24*60; i++ {
		cand := base.Add(time.Duration(i) * time.Minute)
		if cand.Minute() != spec.minute || cand.Hour() != spec.hour {
			continue
		}
		if spec.hasDOM && cand.Day() != spec.dom {
			continue
		}
		if spec.hasMon && int(cand.Month()) != spec.month {
			continue
		}
		if spec.hasDOW && int(cand.Weekday()) != spec.dow {
			continue
		}
		return cand.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("no next run found for cron")
}
