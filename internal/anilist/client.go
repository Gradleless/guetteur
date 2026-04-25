package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const defaultBaseURL = "https://graphql.anilist.co"

type CharacterEdge struct {
	Node struct {
		Name struct{ Full string } `json:"name"`
	} `json:"node"`
	VoiceActors []struct {
		Name struct{ Full string } `json:"name"`
	} `json:"voiceActors"`
}

type RelationEdge struct {
	RelationType string `json:"relationType"`
	Node         struct {
		ID    int64 `json:"id"`
		Title struct {
			Romaji  string `json:"romaji"`
			English string `json:"english"`
		} `json:"title"`
	} `json:"node"`
}

type Media struct {
	ID    int64 `json:"id"`
	Title struct {
		Romaji  string `json:"romaji"`
		English string `json:"english"`
		Native  string `json:"native"`
	} `json:"title"`
	Synonyms    []string `json:"synonyms"`
	Description string   `json:"description"`
	Episodes    *int     `json:"episodes"`
	Status      string   `json:"status"`
	Source      string   `json:"source"`
	Genres      []string `json:"genres"`
	MeanScore   *int     `json:"meanScore"`
	Season      *string  `json:"season"`
	SeasonYear  *int     `json:"seasonYear"`
	CoverImage  struct {
		Large string `json:"large"`
	} `json:"coverImage"`
	SiteURL string `json:"siteUrl"`
	Studios struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"studios"`
	Characters struct {
		Edges []CharacterEdge `json:"edges"`
	} `json:"characters"`
	Relations struct {
		Edges []RelationEdge `json:"edges"`
	} `json:"relations"`
	AiringSchedule struct {
		Nodes []AiringNode `json:"nodes"`
	} `json:"airingSchedule"`
	NextAiringEpisode *AiringNode `json:"nextAiringEpisode"`
}

type AiringNode struct {
	Episode  int   `json:"episode"`
	AiringAt int64 `json:"airingAt"`
}

type Client struct {
	http    *http.Client
	baseURL string
}

func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultBaseURL,
	}
}

func SeasonOf(t time.Time) (season string, year int) {
	year = t.Year()
	switch t.Month() {
	case 1, 2, 3:
		season = "WINTER"
	case 4, 5, 6:
		season = "SPRING"
	case 7, 8, 9:
		season = "SUMMER"
	default:
		season = "FALL"
	}
	return
}

const mediaFragment = `
  id
  title { romaji english native }
  synonyms
  description(asHtml: false)
  episodes
  status
  source
  genres
  meanScore
  season
  seasonYear
  coverImage { large }
  siteUrl
  studios(isMain: true) { nodes { name } }
  characters(sort: ROLE, perPage: 6) {
    edges {
      node { name { full } }
      voiceActors(language: JAPANESE, sort: RELEVANCE) { name { full } }
    }
  }
  relations {
    edges {
      relationType
      node { id title { romaji english } }
    }
  }
  airingSchedule(notYetAired: false, perPage: 50) { nodes { episode airingAt } }
  nextAiringEpisode { episode airingAt }
`

func (c *Client) SeasonAll(ctx context.Context, season string, year int) ([]Media, error) {
	var all []Media
	for page := 1; ; page++ {
		if page > 1 {
			select {
			case <-ctx.Done():
				return all, ctx.Err()
			case <-time.After(time.Second):
			}
		}
		result, err := c.SeasonPage(ctx, season, year, page)
		if err != nil {
			return all, fmt.Errorf("season page %d: %w", page, err)
		}
		all = append(all, result.Media...)
		slog.Debug("anilist season page", "season", season, "year", year, "page", page, "count", len(result.Media))
		if !result.HasNextPage {
			break
		}
	}
	return all, nil
}

type seasonPageResult struct {
	HasNextPage bool
	Media       []Media
}

func (c *Client) SeasonPage(ctx context.Context, season string, year, page int) (*seasonPageResult, error) {
	query := `
query ($season: MediaSeason!, $year: Int!, $page: Int!) {
  Page(page: $page, perPage: 50) {
    pageInfo { hasNextPage }
    media(season: $season, seasonYear: $year, type: ANIME, sort: POPULARITY_DESC) {
` + mediaFragment + `
    }
  }
}`

	var resp struct {
		Data struct {
			Page struct {
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"pageInfo"`
				Media []Media `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	if err := c.do(ctx, query, map[string]any{
		"season": season,
		"year":   year,
		"page":   page,
	}, &resp); err != nil {
		return nil, err
	}

	return &seasonPageResult{
		HasNextPage: resp.Data.Page.PageInfo.HasNextPage,
		Media:       resp.Data.Page.Media,
	}, nil
}

func (c *Client) MediaByID(ctx context.Context, id int64) (*Media, error) {
	query := `
query ($id: Int!) {
  Media(id: $id, type: ANIME) {
` + mediaFragment + `
  }
}`

	var resp struct {
		Data struct {
			Media Media `json:"Media"`
		} `json:"data"`
	}

	if err := c.do(ctx, query, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	return &resp.Data.Media, nil
}

type SearchResult struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	CoverURL string  `json:"cover_url"`
	Season   *string `json:"season"`
	Year     *int    `json:"year"`
	Status   string  `json:"status"`
}

func (c *Client) SearchMedia(ctx context.Context, q string) ([]SearchResult, error) {
	const query = `
query ($q: String!) {
  Page(page: 1, perPage: 10) {
    media(search: $q, type: ANIME) {
      id
      title { romaji english }
      coverImage { large }
      season
      seasonYear
      status
    }
  }
}`

	var resp struct {
		Data struct {
			Page struct {
				Media []struct {
					ID    int64 `json:"id"`
					Title struct {
						Romaji  string `json:"romaji"`
						English string `json:"english"`
					} `json:"title"`
					CoverImage struct {
						Large string `json:"large"`
					} `json:"coverImage"`
					Season     *string `json:"season"`
					SeasonYear *int    `json:"seasonYear"`
					Status     string  `json:"status"`
				} `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	if err := c.do(ctx, query, map[string]any{"q": q}, &resp); err != nil {
		return nil, err
	}

	out := make([]SearchResult, 0, len(resp.Data.Page.Media))
	for _, m := range resp.Data.Page.Media {
		t := m.Title.Romaji
		if m.Title.English != "" {
			t = m.Title.English
		}
		out = append(out, SearchResult{
			ID:       m.ID,
			Title:    t,
			CoverURL: m.CoverImage.Large,
			Season:   m.Season,
			Year:     m.SeasonYear,
			Status:   m.Status,
		})
	}
	return out, nil
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func (c *Client) do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("anilist HTTP %d: %s", res.StatusCode, snippet)
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
