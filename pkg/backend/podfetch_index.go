package backend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// CatalogEpisodes reports every episode PodFetch holds, read straight from its
// SQLite database in one query. The HTTP API only exposes episodes one podcast
// at a time, so a library of any size costs a request per podcast and transfers
// the entire catalog; this reads the same information locally in milliseconds.
func (c *PodFetchBackend) CatalogEpisodes() ([]CatalogEpisode, error) {
	verifyPodfetchNotDisabled("CatalogEpisodes")
	if c.DBPath == "" {
		return nil, fmt.Errorf("podfetch db_path is not configured")
	}
	if fi, err := os.Stat(c.DBPath); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("podfetch db file does not exist: %s", c.DBPath)
	}

	db, err := getPodfetchDB(c.DBPath)
	if err != nil {
		return nil, err
	}

	fileCol := podfetchEpisodeFileCol(db)
	where := ""
	if podfetchHasColumn(db, "podcast_episodes", "deleted") {
		where = " WHERE deleted = 0"
	}
	dateCol := podfetchColOrEmpty(db, "podcast_episodes", "date_of_recording")
	query := fmt.Sprintf("SELECT podcast_id, guid, url, name, %s, %s FROM podcast_episodes%s", fileCol, dateCol, where)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []CatalogEpisode
	for rows.Next() {
		var podcastID string
		var guid, url, name, filePath, published sql.NullString
		if err := rows.Scan(&podcastID, &guid, &url, &name, &filePath, &published); err != nil {
			return nil, fmt.Errorf("read catalog episode: %w", err)
		}
		episodes = append(episodes, CatalogEpisode{
			PodcastID:    podcastID,
			GUID:         strings.TrimSpace(guid.String),
			EnclosureURL: strings.TrimSpace(url.String),
			Title:        strings.TrimSpace(name.String),
			Downloaded:   filePath.Valid && strings.TrimSpace(filePath.String) != "",
			AudioPath:    strings.TrimPrefix(strings.TrimSpace(filePath.String), "podcasts/"),
			PublishedAt:  ParsePubDate(published.String),
		})
	}
	if err := rows.Err(); err != nil {
		return episodes, fmt.Errorf("error reading podcast episode rows: %w", err)
	}
	return episodes, nil
}

// ListPodcasts reports podcast metadata without the episodes. It prefers the
// local database and falls back to the podcast list endpoint, which unlike
// Podcasts() is a single request because it skips the per-podcast detail fetch.
func (c *PodFetchBackend) ListPodcasts() ([]Podcast, error) {
	verifyPodfetchNotDisabled("ListPodcasts")
	if c.DBPath != "" {
		if pods, err := listPodFetchPodcastsDB(c.DBPath); err == nil && len(pods) > 0 {
			return pods, nil
		}
	}
	if c.Host == "" {
		return nil, fmt.Errorf("neither host nor db_path configured for podfetch")
	}

	body, err := c.Request("/api/v1/podcasts", "GET", nil)
	if err != nil {
		return nil, err
	}
	dtos, err := unmarshalPodFetchPodcastList(body)
	if err != nil {
		return nil, err
	}

	podcasts := make([]Podcast, 0, len(dtos))
	for _, dto := range dtos {
		podcasts = append(podcasts, mapPodFetchDTOToPodcast(dto, nil))
	}
	return podcasts, nil
}

func unmarshalPodFetchPodcastList(body []byte) ([]podFetchItemDTO, error) {
	var dtos []podFetchItemDTO
	if err := json.Unmarshal(body, &dtos); err == nil {
		return dtos, nil
	}
	var wrapper struct {
		Podcasts []podFetchItemDTO `json:"podcasts"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Podcasts, nil
}

func listPodFetchPodcastsDB(dbPath string) ([]Podcast, error) {
	verifyPodfetchNotDisabled("listPodFetchPodcastsDB")
	if fi, err := os.Stat(dbPath); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("podfetch db file does not exist: %s", dbPath)
	}
	db, err := getPodfetchDB(dbPath)
	if err != nil {
		return nil, err
	}

	dirCol := podfetchPodcastsDirCol(db)
	imgCol := podfetchColOrEmpty(db, "podcasts", "image_url")
	sumCol := podfetchColOrEmpty(db, "podcasts", "summary")
	autCol := podfetchColOrEmpty(db, "podcasts", "author")
	query := fmt.Sprintf("SELECT id, name, %s, rssfeed, %s, %s, %s FROM podcasts ORDER BY id ASC", dirCol, imgCol, sumCol, autCol)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var podcasts []Podcast
	for rows.Next() {
		var idVal interface{}
		var name, directory, rssfeed, imageURL, summary, author sql.NullString
		if err := rows.Scan(&idVal, &name, &directory, &rssfeed, &imageURL, &summary, &author); err != nil {
			continue
		}
		idStr := fmt.Sprintf("%v", idVal)
		dir := strings.TrimPrefix(directory.String, "podcasts/")
		if dir == "" {
			dir = sanitizePodcastName(name.String)
		}
		podcasts = append(podcasts, Podcast{
			ID:        idStr,
			RelPath:   dir,
			MediaType: "podcast",
			Media: PodcastMedia{
				ID: idStr,
				Metadata: PodcastMetadata{
					Title:       name.String,
					Author:      author.String,
					Description: summary.String,
					FeedURL:     rssfeed.String,
					ImageURL:    imageURL.String,
				},
			},
		})
	}
	if err := rows.Err(); err != nil {
		return podcasts, fmt.Errorf("error reading podcast rows: %w", err)
	}
	return podcasts, nil
}
