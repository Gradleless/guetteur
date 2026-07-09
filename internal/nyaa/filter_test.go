package nyaa

import (
	"testing"
)

func baseInput() FilterInput {
	return FilterInput{
		Release: RawRelease{
			Title:    "[SubsPlease] Frieren - 12 (1080p) [ABC12345].mkv",
			InfoHash: "aabbccdd",
			Seeders:  42,
		},
		SeriesID:           1,
		SeriesTitles:       []string{"Frieren: Beyond Journey's End", "Sousou no Frieren"},
		PreferredGroups:    nil,
		QualityPriority:    []string{"1080p", "720p"},
		DefaultGroups:      []string{"SubsPlease", "Erai-raws"},
		PreferredLanguage:  nil,
		MinSeeders:         3,
		ExistingInfoHashes: map[string]string{},
		ExistingEpisodes:   map[int]string{},
	}
}

func TestFilter_HappyPath(t *testing.T) {
	in := baseInput()
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected match, got skip reason: %s", got.Reason)
	}
	if got.Parsed.Group != "SubsPlease" {
		t.Errorf("Parsed.Group: got %q, want SubsPlease", got.Parsed.Group)
	}
}

func TestFilter_ResolutionNotInPriority(t *testing.T) {
	in := baseInput()
	in.Release.Title = "[SubsPlease] Frieren - 12 (480p) [ABC12345].mkv"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip, got match")
	}
}

func TestFilter_GroupNotWhitelisted(t *testing.T) {
	in := baseInput()
	in.Release.Title = "[RandomGroup] Frieren - 12 (1080p) [ABC12345].mkv"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip, got match")
	}
}

func TestFilter_GroupCaseInsensitive(t *testing.T) {
	in := baseInput()

	in.DefaultGroups = []string{"subsplease"}
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected match (case-insensitive group), got: %s", got.Reason)
	}
}

func TestFilter_PerSeriesGroupOverride(t *testing.T) {
	in := baseInput()

	in.PreferredGroups = []string{"Tsundere-Raws"}
	got := Filter(in)
	if got.Match {
		t.Error("expected skip when group not in per-series override")
	}
}

func TestFilter_NoEpisodeNumber(t *testing.T) {
	in := baseInput()

	in.Release.Title = "[SubsPlease] Frieren (1080p) [ABC12345].mkv"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: no episode number and not a batch")
	}
}

func TestFilter_BatchPassesWithoutEpisode(t *testing.T) {
	in := baseInput()
	in.Release.Title = "[SubsPlease] Frieren (01-12) [1080p] [Batch]"
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected batch to match, got: %s", got.Reason)
	}
	if !got.Parsed.IsBatch {
		t.Error("expected IsBatch=true")
	}
}

func TestFilter_SeedersBelow(t *testing.T) {
	in := baseInput()
	in.Release.Seeders = 1
	in.MinSeeders = 3
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: seeders below minimum")
	}
}

func TestFilter_AlreadyCompleted(t *testing.T) {
	in := baseInput()
	in.ExistingInfoHashes["aabbccdd"] = "completed"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: already completed")
	}
}

func TestFilter_AlreadyDownloading(t *testing.T) {
	in := baseInput()
	in.ExistingInfoHashes["aabbccdd"] = "downloading"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: already downloading")
	}
}

func TestFilter_QueuedHashSkipped(t *testing.T) {
	// A queued release is already tracked; re-matching it would re-trigger
	// startDownload on every poll.
	in := baseInput()
	in.ExistingInfoHashes["aabbccdd"] = "queued"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: hash already queued")
	}
}

func TestFilter_FailedHashRetried(t *testing.T) {
	in := baseInput()
	in.ExistingInfoHashes["aabbccdd"] = "failed"
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected match for failed hash (retry candidate), got: %s", got.Reason)
	}
}

func TestFilter_DeletedHashSkipped(t *testing.T) {
	in := baseInput()
	in.ExistingInfoHashes["aabbccdd"] = "deleted"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: hash deleted by user")
	}
}

func TestFilter_FuzzyTitleMiss(t *testing.T) {
	in := baseInput()
	in.SeriesTitles = []string{"Completely Different Anime"}
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: fuzzy title miss")
	}
}

func TestFilter_FuzzyTitlePassesVariant(t *testing.T) {
	in := baseInput()

	in.SeriesTitles = []string{"Frieren"}
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected match with exact title, got: %s", got.Reason)
	}
}

func TestFilter_LanguageFilter_VostfrPresent(t *testing.T) {
	in := baseInput()
	in.PreferredLanguage = []string{"VOSTFR"}
	in.Release.Title = "Hells Paradise S02E11 1080p x264-Tsundere-Raws (VOSTFR)"
	in.Release.InfoHash = "cafebabe"
	in.DefaultGroups = []string{"Tsundere-Raws"}
	in.SeriesTitles = []string{"Hells Paradise", "Jigokuraku"}
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected match with VOSTFR tag, got: %s", got.Reason)
	}
}

func TestFilter_LanguageFilter_NoTag_Rejected(t *testing.T) {

	in := baseInput()
	in.PreferredLanguage = []string{"VOSTFR"}
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: SubsPlease has no VOSTFR tag")
	}
}

func TestFilter_LanguageFilter_Disabled(t *testing.T) {

	in := baseInput()
	in.PreferredLanguage = nil
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected match with no language filter, got: %s", got.Reason)
	}
}

func TestFilter_EpisodeAlreadyQueued(t *testing.T) {
	// Different hash, same episode — should be rejected (e.g. MULTI vs VOSTFR).
	in := baseInput()
	ep := 12
	in.ExistingEpisodes[ep] = "queued"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: episode 12 already queued with different hash")
	}
}

func TestFilter_EpisodeAlreadyCompleted(t *testing.T) {
	in := baseInput()
	in.ExistingEpisodes[12] = "completed"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: episode 12 already completed")
	}
}

func TestFilter_EpisodeDeleted(t *testing.T) {
	// A deleted episode must not come back through the poll, even via another
	// release with a different hash — only an explicit re-download restores it.
	in := baseInput()
	in.ExistingEpisodes[12] = "deleted"
	got := Filter(in)
	if got.Match {
		t.Error("expected skip: episode 12 deleted")
	}
}

func TestFilter_EpisodeDifferentNotBlocked(t *testing.T) {
	// Episode 11 is queued; episode 12 (our release) should still match.
	in := baseInput()
	in.ExistingEpisodes[11] = "queued"
	got := Filter(in)
	if !got.Match {
		t.Errorf("expected match for ep 12 when only ep 11 is queued, got: %s", got.Reason)
	}
}
