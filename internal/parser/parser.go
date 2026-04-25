package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/nssteinbrenner/anitogo"
)

var reEncoderGroup = regexp.MustCompile(`(?i)\b(?:x264|x265|hevc|avc|h\.264|h\.265)-([A-Za-z0-9][A-Za-z0-9_-]*)`)

type ParsedRelease struct {
	Title      string
	Group      string
	Resolution string
	Episode    *int
	EpisodeEnd *int
	IsBatch    bool
	Version    int
}

func Parse(title string) ParsedRelease {
	elems := anitogo.Parse(title, anitogo.DefaultOptions)

	var out ParsedRelease
	out.Title = elems.AnimeTitle
	out.Group = elems.ReleaseGroup
	out.Resolution = elems.VideoResolution

	if out.Group == "" {
		if m := reEncoderGroup.FindStringSubmatch(title); len(m) == 2 {
			out.Group = m[1]
		}
	}

	switch len(elems.EpisodeNumber) {
	case 1:
		if n, err := strconv.Atoi(elems.EpisodeNumber[0]); err == nil {
			out.Episode = &n
		}
	case 2:
		if start, err := strconv.Atoi(elems.EpisodeNumber[0]); err == nil {
			out.Episode = &start
		}
		if end, err := strconv.Atoi(elems.EpisodeNumber[1]); err == nil {
			out.EpisodeEnd = &end
		}
		out.IsBatch = true
	}

	if !out.IsBatch {
		for _, info := range elems.ReleaseInformation {
			if strings.EqualFold(info, "batch") || strings.EqualFold(info, "complete") {
				out.IsBatch = true
				break
			}
		}
	}

	if len(elems.ReleaseVersion) > 0 {
		if v, err := strconv.Atoi(elems.ReleaseVersion[0]); err == nil {
			out.Version = v
		}
	}

	return out
}
