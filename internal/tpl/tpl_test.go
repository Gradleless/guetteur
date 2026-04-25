package tpl_test

import (
	"testing"

	"github.com/gradleless/guetteur/internal/tpl"
)

func TestExpand(t *testing.T) {
	defaultTemplate := "{title}/Season {season:02d}/{title} - S{season:02d}E{episode:02d} [{group}][{resolution}].{ext}"

	tests := []struct {
		name    string
		tmpl    string
		data    tpl.Data
		want    string
		wantErr bool
	}{
		{
			name: "standard episode english title",
			tmpl: defaultTemplate,
			data: tpl.Data{
				TitleEnglish: "Frieren: Beyond Journey's End",
				TitleRomaji:  "Sousou no Frieren",
				Season:       1,
				Episode:      3,
				Group:        "SubsPlease",
				Resolution:   "1080p",
				Ext:          "mkv",
			},
			want: "Frieren: Beyond Journey's End/Season 01/Frieren: Beyond Journey's End - S01E03 [SubsPlease][1080p].mkv",
		},
		{
			name: "falls back to romaji when no english",
			tmpl: defaultTemplate,
			data: tpl.Data{
				TitleRomaji: "Dungeon Meshi",
				Season:      1,
				Episode:     12,
				Group:       "Erai-raws",
				Resolution:  "1080p",
				Ext:         "mkv",
			},
			want: "Dungeon Meshi/Season 01/Dungeon Meshi - S01E12 [Erai-raws][1080p].mkv",
		},
		{
			name: "zero-padded season and episode",
			tmpl: "S{season:02d}E{episode:03d}",
			data: tpl.Data{Episode: 5, Season: 2},
			want: "S02E005",
		},
		{
			name: "no format spec",
			tmpl: "{episode}",
			data: tpl.Data{Episode: 7},
			want: "7",
		},
		{
			name: "batch with episode_end",
			tmpl: "{title} - S{season:02d}E{episode:02d}-E{episode_end:02d}.{ext}",
			data: tpl.Data{
				TitleRomaji: "One Piece",
				Season:      1,
				Episode:     1,
				EpisodeEnd:  12,
				Ext:         "mkv",
			},
			want: "One Piece - S01E01-E12.mkv",
		},
		{
			name:    "unknown placeholder returns error",
			tmpl:    "{unknown}",
			data:    tpl.Data{},
			wantErr: true,
		},
		{
			name: "title_english and title_romaji placeholders",
			tmpl: "{title_english} / {title_romaji}",
			data: tpl.Data{
				TitleEnglish: "My Hero Academia",
				TitleRomaji:  "Boku no Hero Academia",
			},
			want: "My Hero Academia / Boku no Hero Academia",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tpl.Expand(tt.tmpl, tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Expand() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Expand() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Show/Season 01/My Show - S01E01.mkv", "My Show/Season 01/My Show - S01E01.mkv"},
		{`Bad:Title/Season 01/ep.mkv`, "Bad_Title/Season 01/ep.mkv"},
	}
	for _, tt := range tests {
		got := tpl.SanitizePath(tt.input)
		if got != tt.want {
			t.Errorf("SanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
