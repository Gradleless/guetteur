package nyaa

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	nyaaBase    = "https://nyaa.si"
	rssTimeout  = 20 * time.Second
	rssCategory = "1_0"
)

type RawRelease struct {
	Title    string
	NyaaURL  string
	Magnet   string
	InfoHash string
	Seeders  int
	Leechers int
	PubDate  time.Time
}

func BuildRSSURL(query, quality, group string, extras ...string) string {
	parts := make([]string, 0, 2+len(extras))
	parts = append(parts, query, quality)
	parts = append(parts, extras...)
	q := url.QueryEscape(strings.Join(parts, " "))
	base := fmt.Sprintf("%s/?page=rss&c=%s&f=0&s=seeders&o=desc&q=%s", nyaaBase, rssCategory, q)
	if group != "" {
		base += "&u=" + url.QueryEscape(group)
	}
	return base
}

func Fetch(ctx context.Context, rssURL string) ([]RawRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, rssTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "guetteur/1.0 (+https://github.com/gradleless/guetteur)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch rss %s: %w", rssURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {

		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nyaa returned HTTP %d", resp.StatusCode)
	}

	return ParseRSS(resp.Body)
}

func ParseRSS(r io.Reader) ([]RawRelease, error) {
	type nyaaItem struct {
		Title    string `xml:"title"`
		Link     string `xml:"link"`
		GUID     string `xml:"guid"`
		PubDate  string `xml:"pubDate"`
		Seeders  string `xml:"https://nyaa.si/xmlns/nyaa seeders"`
		Leechers string `xml:"https://nyaa.si/xmlns/nyaa leechers"`
		InfoHash string `xml:"https://nyaa.si/xmlns/nyaa infoHash"`
	}
	type rssChannel struct {
		Items []nyaaItem `xml:"item"`
	}
	type rssFeed struct {
		Channel rssChannel `xml:"channel"`
	}

	var feed rssFeed
	if err := xml.NewDecoder(r).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode rss xml: %w", err)
	}

	releases := make([]RawRelease, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		seeders, _ := strconv.Atoi(strings.TrimSpace(item.Seeders))
		leechers, _ := strconv.Atoi(strings.TrimSpace(item.Leechers))
		infoHash := strings.ToLower(strings.TrimSpace(item.InfoHash))

		pubDate, _ := time.Parse(time.RFC1123Z, item.PubDate)
		if pubDate.IsZero() {

			pubDate, _ = time.Parse(time.RFC1123, item.PubDate)
		}

		releases = append(releases, RawRelease{
			Title:    strings.TrimSpace(item.Title),
			NyaaURL:  strings.TrimSpace(item.GUID),
			Magnet:   buildMagnet(infoHash, item.Title),
			InfoHash: infoHash,
			Seeders:  seeders,
			Leechers: leechers,
			PubDate:  pubDate,
		})
	}
	return releases, nil
}

func buildMagnet(infoHash, displayName string) string {
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s&tr=http%%3A%%2F%%2Fnyaa.tracker.wf%%3A7777%%2Fannounce",
		infoHash, url.QueryEscape(displayName))
}
