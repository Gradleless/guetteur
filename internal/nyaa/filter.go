package nyaa

import (
	"log/slog"
	"strings"

	"github.com/hbollon/go-edlib"

	"github.com/gradleless/guetteur/internal/parser"
)

const defaultJaroWinklerThreshold = float32(0.85)

type FilterInput struct {
	Release RawRelease

	SeriesID        int64
	SeriesTitles    []string
	PreferredGroups []string

	QualityPriority   []string
	DefaultGroups     []string
	PreferredLanguage []string
	MinSeeders        int
	FuzzyThreshold    float32

	ExistingInfoHashes map[string]string
	// ExistingEpisodes maps episode number → status for already-known releases of
	// this series. Used to reject a second variant (e.g. MULTI) for an episode
	// that is already queued/downloading/completed with another hash.
	ExistingEpisodes map[int]string
}

type FilterResult struct {
	Match  bool
	Parsed parser.ParsedRelease
	Reason string
}

func Filter(in FilterInput) FilterResult {
	skip := func(reason string) FilterResult {
		slog.Debug("release skipped",
			"series_id", in.SeriesID,
			"info_hash", in.Release.InfoHash,
			"title", in.Release.Title,
			"reason", reason,
		)
		return FilterResult{Reason: reason}
	}

	parsed := parser.Parse(in.Release.Title)

	if !containsCI(in.QualityPriority, parsed.Resolution) {
		return skip("resolution not in quality_priority: " + parsed.Resolution)
	}

	effectiveGroups := in.DefaultGroups
	if len(in.PreferredGroups) > 0 {
		effectiveGroups = in.PreferredGroups
	}
	if !containsCI(effectiveGroups, parsed.Group) {
		if g := groupInTitle(in.Release.Title, effectiveGroups); g != "" {
			parsed.Group = g
		} else {
			return skip("group not whitelisted: " + parsed.Group)
		}
	}

	if parsed.Episode == nil && !parsed.IsBatch {
		return skip("no episode number and not a batch")
	}

	if in.Release.Seeders < in.MinSeeders {
		return skip("seeders below minimum")
	}

	// A known info_hash is never re-queued, except "failed" which is a retry
	// candidate. This also keeps superseded/skipped/deleted releases from being
	// restarted on every poll.
	if status, exists := in.ExistingInfoHashes[in.Release.InfoHash]; exists {
		if status != "failed" {
			return skip("already " + status)
		}
	}

	// Episode-level dedup: reject a second variant for the same episode even if
	// the info_hash differs (e.g. MULTI and VOSTFR releases of the same episode).
	// "deleted" counts too: a user-removed episode only comes back through an
	// explicit re-download, not through the next poll.
	if parsed.Episode != nil && !parsed.IsBatch {
		if status, exists := in.ExistingEpisodes[*parsed.Episode]; exists {
			switch status {
			case "queued", "downloading", "completed", "deleted":
				return skip("episode already " + status)
			}
		}
	}

	if !fuzzyMatchesAnySeries(parsed.Title, in.SeriesTitles, in.FuzzyThreshold) {
		return skip("fuzzy title match below threshold: " + parsed.Title)
	}

	if len(in.PreferredLanguage) > 0 {
		if !titleContainsLanguage(in.Release.Title, in.PreferredLanguage) {
			return skip("language tag not found in title")
		}
	}

	return FilterResult{Match: true, Parsed: parsed}
}

func groupInTitle(title string, groups []string) string {
	titleLower := strings.ToLower(title)
	for _, g := range groups {
		if strings.Contains(titleLower, strings.ToLower(g)) {
			return g
		}
	}
	return ""
}

func containsCI(slice []string, s string) bool {
	sLower := strings.ToLower(s)
	for _, v := range slice {
		if strings.ToLower(v) == sLower {
			return true
		}
	}
	return false
}

func fuzzyMatchesAnySeries(title string, seriesTitles []string, threshold float32) bool {
	if title == "" {
		return false
	}
	if threshold == 0 {
		threshold = defaultJaroWinklerThreshold
	}
	titleLower := strings.ToLower(title)
	for _, st := range seriesTitles {
		if st == "" {
			continue
		}
		stLower := strings.ToLower(st)

		if edlib.JaroWinklerSimilarity(titleLower, stLower) >= threshold {
			return true
		}

		if len(titleLower) >= 4 && (strings.Contains(stLower, titleLower) || strings.Contains(titleLower, stLower)) {
			return true
		}
	}
	return false
}

func titleContainsLanguage(title string, preferred []string) bool {
	titleUpper := strings.ToUpper(title)
	for _, lang := range preferred {
		if strings.Contains(titleUpper, strings.ToUpper(lang)) {
			return true
		}
	}
	return false
}
