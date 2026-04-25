package parser

import (
	"testing"
)

func intPtr(n int) *int { return &n }

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTitle string
		wantGroup string
		wantRes   string
		wantEp    *int
		wantEpEnd *int
		wantBatch bool
		wantVer   int
	}{
		{
			name:      "SubsPlease standard",
			input:     "[SubsPlease] Frieren - 12 (1080p) [ABC12345].mkv",
			wantTitle: "Frieren",
			wantGroup: "SubsPlease",
			wantRes:   "1080p",
			wantEp:    intPtr(12),
		},
		{
			name:      "Erai-raws with dash",
			input:     "[Erai-raws] Dan Da Dan - 05 [1080p][Multiple Subtitle][ABC12345]",
			wantTitle: "Dan Da Dan",
			wantGroup: "Erai-raws",
			wantRes:   "1080p",
			wantEp:    intPtr(5),
		},
		{
			name:      "ASW HEVC",
			input:     "[ASW] Solo Leveling - 08 [1080p HEVC][MultiSub].mkv",
			wantTitle: "Solo Leveling",
			wantGroup: "ASW",
			wantRes:   "1080p",
			wantEp:    intPtr(8),
		},
		{
			name:      "Judas S02E notation",
			input:     "[Judas] Jujutsu Kaisen - S02E15 (1080p) [Multi-Sub]",
			wantTitle: "Jujutsu Kaisen",
			wantGroup: "Judas",
			wantRes:   "1080p",
			wantEp:    intPtr(15),
		},
		{
			name:      "batch range",
			input:     "[SubsPlease] Bocchi the Rock (01-12) [1080p] [Batch]",
			wantTitle: "Bocchi the Rock",
			wantGroup: "SubsPlease",
			wantRes:   "1080p",
			wantEp:    intPtr(1),
			wantEpEnd: intPtr(12),
			wantBatch: true,
		},
		{
			name:      "v2 release",
			input:     "[SubsPlease] Show - 01v2 (1080p)",
			wantTitle: "Show",
			wantGroup: "SubsPlease",
			wantRes:   "1080p",
			wantEp:    intPtr(1),
			wantVer:   2,
		},
		{
			name:      "multi-season S03 notation",
			input:     "[Group] Show S03 - 04 [1080p].mkv",
			wantTitle: "Show",
			wantGroup: "Group",
			wantRes:   "1080p",
			wantEp:    intPtr(4),
		},
		{
			name:      "unicode title",
			input:     "[SubsPlease] シリーズ名 - 01 (720p) [ABCDEF01].mkv",
			wantGroup: "SubsPlease",
			wantRes:   "720p",
			wantEp:    intPtr(1),
		},
		{
			name:      "empty/malformed",
			input:     "random.mkv",
			wantTitle: "",
			wantGroup: "",
			wantRes:   "",
			wantEp:    nil,
		},
		{

			name:      "Tsundere-Raws un-bracketed group",
			input:     "Hells Paradise S02E11 MULTi AD 1080p CR WEB-DL AAC2.0 x264-Tsundere-Raws (VF, FRENCH, SUBFRENCH, VOSTFR, Hell's Paradise Season 2, Jigokuraku 2nd Season)",
			wantTitle: "Hells Paradise",
			wantGroup: "Tsundere-Raws",
			wantRes:   "1080p",
			wantEp:    intPtr(11),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.input)

			if tc.wantTitle != "" && got.Title != tc.wantTitle {
				t.Errorf("Title: got %q, want %q", got.Title, tc.wantTitle)
			}
			if tc.wantGroup != "" && got.Group != tc.wantGroup {
				t.Errorf("Group: got %q, want %q", got.Group, tc.wantGroup)
			}
			if tc.wantRes != "" && got.Resolution != tc.wantRes {
				t.Errorf("Resolution: got %q, want %q", got.Resolution, tc.wantRes)
			}
			if tc.wantEp == nil && got.Episode != nil {
				t.Errorf("Episode: got %v, want nil", *got.Episode)
			}
			if tc.wantEp != nil {
				if got.Episode == nil {
					t.Errorf("Episode: got nil, want %d", *tc.wantEp)
				} else if *got.Episode != *tc.wantEp {
					t.Errorf("Episode: got %d, want %d", *got.Episode, *tc.wantEp)
				}
			}
			if tc.wantEpEnd == nil && got.EpisodeEnd != nil {
				t.Errorf("EpisodeEnd: got %v, want nil", *got.EpisodeEnd)
			}
			if tc.wantEpEnd != nil {
				if got.EpisodeEnd == nil {
					t.Errorf("EpisodeEnd: got nil, want %d", *tc.wantEpEnd)
				} else if *got.EpisodeEnd != *tc.wantEpEnd {
					t.Errorf("EpisodeEnd: got %d, want %d", *got.EpisodeEnd, *tc.wantEpEnd)
				}
			}
			if got.IsBatch != tc.wantBatch {
				t.Errorf("IsBatch: got %v, want %v", got.IsBatch, tc.wantBatch)
			}
			if got.Version != tc.wantVer {
				t.Errorf("Version: got %d, want %d", got.Version, tc.wantVer)
			}
		})
	}
}
