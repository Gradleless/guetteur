package tpl

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Data struct {
	Title        string
	TitleEnglish string
	TitleRomaji  string
	Season       int
	Episode      int
	EpisodeEnd   int
	Group        string
	Resolution   string
	Ext          string
	TmdbID       int64
}

var placeholderRE = regexp.MustCompile(`\{(\w+)(?::([^}]+))?\}`)

func Expand(tmpl string, d Data) (string, error) {

	title := d.TitleRomaji
	if d.TitleEnglish != "" {
		title = d.TitleEnglish
	}
	if d.Title != "" {
		title = d.Title
	}

	var expandErr error
	result := placeholderRE.ReplaceAllStringFunc(tmpl, func(match string) string {
		if expandErr != nil {
			return match
		}
		sub := placeholderRE.FindStringSubmatch(match)
		name := sub[1]
		fmtSpec := sub[2]

		var rawVal any
		switch name {
		case "title":
			rawVal = title
		case "title_english":
			rawVal = d.TitleEnglish
		case "title_romaji":
			rawVal = d.TitleRomaji
		case "season":
			rawVal = d.Season
		case "episode":
			rawVal = d.Episode
		case "episode_end":
			rawVal = d.EpisodeEnd
		case "group":
			rawVal = d.Group
		case "resolution":
			rawVal = d.Resolution
		case "ext":
			rawVal = d.Ext
		case "tmdb_id":
			rawVal = d.TmdbID
		case "tmdb_tag":
			if d.TmdbID > 0 {
				return fmt.Sprintf(" [tmdbid-%d]", d.TmdbID)
			}
			return ""
		default:
			expandErr = fmt.Errorf("unknown placeholder %q", name)
			return match
		}

		return formatValue(rawVal, fmtSpec)
	})
	if expandErr != nil {
		return "", expandErr
	}
	return result, nil
}

func formatValue(v any, spec string) string {
	if spec == "" {
		return fmt.Sprintf("%v", v)
	}

	switch n := v.(type) {
	case int:
		return formatInt(int64(n), spec)
	case int64:
		return formatInt(n, spec)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatInt(n int64, spec string) string {

	s := strings.TrimSuffix(spec, "d")
	if s == "" {
		return strconv.FormatInt(n, 10)
	}
	pad := '0'
	width := 0
	if strings.HasPrefix(s, "0") {
		s = s[1:]
	} else {
		pad = ' '
	}
	if w, err := strconv.Atoi(s); err == nil {
		width = w
	}
	formatted := strconv.FormatInt(n, 10)
	for len(formatted) < width {
		formatted = string(pad) + formatted
	}
	return formatted
}

func SanitizePath(p string) string {
	parts := strings.Split(p, string(filepath.Separator))
	for i, part := range parts {
		parts[i] = sanitizeSegment(part)
	}

	var result []string
	for _, segment := range strings.Split(strings.Join(parts, string(filepath.Separator)), "/") {
		result = append(result, sanitizeSegment(segment))
	}
	return strings.Join(result, "/")
}

var unsafeChars = regexp.MustCompile(`[<>:"|?*\\]`)

func sanitizeSegment(s string) string {
	return unsafeChars.ReplaceAllString(s, "_")
}
