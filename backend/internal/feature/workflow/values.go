package workflow

import (
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
)

var variables = regexp.MustCompile(`\{\{[^}]+\}\}`)
var wallTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
var emailCandidates = regexp.MustCompile(`[^\p{Z}\t\n\r\f\v\x{feff}<>()",;:]+@[^\p{Z}\t\n\r\f\v\x{feff}<>()",;:]+`)
var emailLocal = regexp.MustCompile("^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+$")
var emailDomain = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`)

func ExtractEmail(text string) string {
	unique := map[string]bool{}
	for _, candidate := range emailCandidates.FindAllString(text, -1) {
		unique[strings.TrimRight(candidate, ".!?")] = true
	}
	if len(unique) != 1 {
		return ""
	}
	for address := range unique {
		parts := strings.Split(address, "@")
		if len(parts) != 2 {
			return ""
		}
		local, domain := parts[0], parts[1]
		if len(address) > 254 || len(local) > 64 || !emailLocal.MatchString(local) || strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, "..") || !emailDomain.MatchString(domain) {
			return ""
		}
		for _, label := range strings.Split(domain, ".") {
			if len(label) > 63 {
				return ""
			}
		}
		return address
	}
	return ""
}
func Due(s Step, now time.Time) (time.Time, error) {
	if s.Kind == "delay" {
		return now.Add(time.Duration(s.Minutes) * time.Minute), nil
	}
	if s.Mode != "next_time" {
		return time.Parse(time.RFC3339, s.At)
	}
	location, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	instant := now.Truncate(time.Minute)
	if instant.Before(now) {
		instant = instant.Add(time.Minute)
	}
	for range 72 * 60 {
		if instant.In(location).Format("15:04") == s.Time {
			return instant, nil
		}
		instant = instant.Add(time.Minute)
	}
	return time.Time{}, errorsSchedule
}

var errorsSchedule = &scheduleError{}

type scheduleError struct{}

func (*scheduleError) Error() string { return "no schedule occurrence within 72 hours" }
