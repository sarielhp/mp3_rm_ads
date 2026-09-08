package backend

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func podfetchHasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err == nil {
			if strings.EqualFold(name, column) {
				return true
			}
		}
	}
	return false
}

func podfetchPodcastsDirCol(db *sql.DB) string {
	if podfetchHasColumn(db, "podcasts", "directory_name") {
		return "directory_name"
	}
	return "directory"
}

func podfetchEpisodeFileCol(db *sql.DB) string {
	if podfetchHasColumn(db, "podcast_episodes", "file_episode_path") {
		return "file_episode_path"
	}
	return "local_url"
}

func podfetchColOrEmpty(db *sql.DB, table, col string) string {
	if podfetchHasColumn(db, table, col) {
		return col
	}
	return "'' AS " + col
}

func fetchPodFetchPodcastsDB(dbPath string) ([]Podcast, error) {
	verifyPodfetchNotDisabled("fetchPodFetchPodcastsDB")
	if dbPath == "" {
		return nil, fmt.Errorf("dbPath is empty")
	}
	if fi, err := os.Stat(dbPath); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("podfetch db file does not exist: %s", dbPath)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

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

		eps, _ := fetchPodFetchEpisodesForPodcastDB(db, idStr)

		pod := Podcast{
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
				Episodes: eps,
			},
		}
		podcasts = append(podcasts, pod)
	}

	return podcasts, nil
}

func fetchPodFetchEpisodesForPodcastDB(db *sql.DB, podcastID string) ([]Episode, error) {
	verifyPodfetchNotDisabled("fetchPodFetchEpisodesForPodcastDB")
	fileCol := podfetchEpisodeFileCol(db)
	hasStatus := podfetchHasColumn(db, "podcast_episodes", "status")
	hasDesc := podfetchHasColumn(db, "podcast_episodes", "description")

	statusExpr := "'D'"
	if hasStatus {
		statusExpr = "status"
	}
	descExpr := "''"
	if hasDesc {
		descExpr = "description"
	}

	query := fmt.Sprintf("SELECT id, episode_id, name, url, date_of_recording, total_time, %s, %s, %s FROM podcast_episodes WHERE podcast_id = ? ORDER BY id DESC", fileCol, descExpr, statusExpr)
	rows, err := db.Query(query, podcastID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var idVal interface{}
		var epID, name, url, dateOfRec, localURL, description, status sql.NullString
		var totalTime sql.NullFloat64

		if err := rows.Scan(&idVal, &epID, &name, &url, &dateOfRec, &totalTime, &localURL, &description, &status); err != nil {
			continue
		}

		idStr := fmt.Sprintf("%v", idVal)
		guid := epID.String
		if guid == "" {
			guid = idStr
		}

		dateStr := dateOfRec.String
		dur := totalTime.Float64

		ep := Episode{
			ID:           idStr,
			Title:        name.String,
			GUID:         guid,
			PubDate:      dateStr,
			PublishedAt:  ParsePubDate(dateStr),
			Duration:     dur,
			Description:  description.String,
			EnclosureURL: url.String,
		}

		cleanPath := strings.TrimPrefix(localURL.String, "podcasts/")
		if cleanPath != "" {
			ep.AudioFile = &PodcastAudioFile{
				Duration: dur,
				Metadata: &AudioFileMetadata{
					Filename: filepath.Base(cleanPath),
					Path:     cleanPath,
					RelPath:  cleanPath,
				},
			}
		}

		episodes = append(episodes, ep)
	}

	return episodes, nil
}

func fetchPodFetchPodcastDB(dbPath, id string) (*Podcast, error) {
	verifyPodfetchNotDisabled("fetchPodFetchPodcastDB")
	if dbPath == "" {
		return nil, fmt.Errorf("dbPath is empty")
	}
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dirCol := podfetchPodcastsDirCol(db)
	imgCol := podfetchColOrEmpty(db, "podcasts", "image_url")
	sumCol := podfetchColOrEmpty(db, "podcasts", "summary")
	autCol := podfetchColOrEmpty(db, "podcasts", "author")
	var idVal interface{}
	var name, directory, rssfeed, imageURL, summary, author sql.NullString

	query := fmt.Sprintf("SELECT id, name, %s, rssfeed, %s, %s, %s FROM podcasts WHERE id = ? OR name = ? OR %s = ? LIMIT 1", dirCol, imgCol, sumCol, autCol, dirCol)
	err = db.QueryRow(query, id, id, id).Scan(&idVal, &name, &directory, &rssfeed, &imageURL, &summary, &author)
	if err != nil {
		return nil, err
	}

	idStr := fmt.Sprintf("%v", idVal)
	dir := strings.TrimPrefix(directory.String, "podcasts/")
	if dir == "" {
		dir = sanitizePodcastName(name.String)
	}

	eps, _ := fetchPodFetchEpisodesForPodcastDB(db, idStr)

	pod := &Podcast{
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
			Episodes: eps,
		},
	}
	return pod, nil
}

func createPodFetchPodcastDB(dbPath, title, directory, feedURL string) (*Podcast, error) {
	verifyPodfetchNotDisabled("createPodFetchPodcastDB")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dirCol := podfetchPodcastsDirCol(db)
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := fmt.Sprintf("INSERT INTO podcasts (name, %s, rssfeed, created_at) VALUES (?, ?, ?, ?)", dirCol)
	res, err := db.Exec(query, title, directory, feedURL, nowStr)
	if err != nil {
		return nil, err
	}

	lastID, _ := res.LastInsertId()
	idStr := strconv.FormatInt(lastID, 10)

	return &Podcast{
		ID:      idStr,
		RelPath: directory,
		Media: PodcastMedia{
			ID: idStr,
			Metadata: PodcastMetadata{
				Title:   title,
				FeedURL: feedURL,
			},
		},
	}, nil
}

func deletePodFetchEpisodeDB(dbPath, podcastID, episodeID string) error {
	verifyPodfetchNotDisabled("deletePodFetchEpisodeDB")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM podcast_episodes WHERE (podcast_id = ? OR ? = '') AND (id = ? OR episode_id = ?)", podcastID, podcastID, episodeID, episodeID)
	return err
}

func deletePodFetchPodcastDB(dbPath, podcastID string) error {
	verifyPodfetchNotDisabled("deletePodFetchPodcastDB")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM podcast_episodes WHERE podcast_id = ?", podcastID); err != nil {
		return err
	}
	dirCol := podfetchPodcastsDirCol(db)
	query := fmt.Sprintf("DELETE FROM podcasts WHERE id = ? OR name = ? OR %s = ?", dirCol)
	if _, err := tx.Exec(query, podcastID, podcastID, podcastID); err != nil {
		return err
	}
	return tx.Commit()
}

func fetchActiveDownloadsDB(dbPath, podcastID string) ([]ActiveDownload, error) {
	verifyPodfetchNotDisabled("fetchActiveDownloadsDB")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if !podfetchHasColumn(db, "podcast_episodes", "status") {
		return nil, nil
	}

	query := "SELECT id, name, episode_id, url FROM podcast_episodes WHERE (status = 'P' OR status = 'DOWNLOADING') AND (podcast_id = ? OR ? = '')"
	rows, err := db.Query(query, podcastID, podcastID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dls []ActiveDownload
	for rows.Next() {
		var idVal interface{}
		var name, epID, url sql.NullString
		if err := rows.Scan(&idVal, &name, &epID, &url); err == nil {
			dls = append(dls, ActiveDownload{
				ID:                  fmt.Sprintf("%v", idVal),
				EpisodeDisplayTitle: name.String,
				Title:               name.String,
				EpisodeID:           epID.String,
				URL:                 url.String,
			})
		}
	}
	return dls, nil
}

func updatePodFetchDurationDB(dbPath, filePath string, duration float64) error {
	verifyPodfetchNotDisabled("updatePodFetchDurationDB")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	base := filepath.Base(filePath)
	likePattern := "%" + base
	fileCol := podfetchEpisodeFileCol(db)

	query := fmt.Sprintf("UPDATE podcast_episodes SET total_time = ? WHERE %s = ? OR %s LIKE ?", fileCol, fileCol)
	_, err = db.Exec(query, int(duration), filePath, likePattern)
	return err
}

func resetPodFetchDateCheckDB(dbPath, itemID, title string) error {
	verifyPodfetchNotDisabled("resetPodFetchDateCheckDB")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	if itemID != "" {
		_, err = db.Exec("UPDATE podcasts SET created_at = '1970-01-01 00:00:00' WHERE id = ?", itemID)
		if err == nil {
			return nil
		}
	}
	if title != "" {
		_, err = db.Exec("UPDATE podcasts SET created_at = '1970-01-01 00:00:00' WHERE name = ?", title)
		return err
	}
	return nil
}

func updatePodFetchSettingsDB(dbPath, identifier string, autoDownload, autoCleanup bool, autoCleanupDays int) error {
	verifyPodfetchNotDisabled("updatePodFetchSettingsDB")
	if dbPath == "" {
		return fmt.Errorf("dbPath is empty")
	}
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	var realID string
	query := "SELECT id FROM podcasts WHERE id = ? OR lower(name) = lower(?) OR lower(directory_name) = lower(?) OR lower(directory_name) = lower('podcasts/' || ?) OR lower(directory_name) = lower(?) LIMIT 1"
	cleanIdent := strings.TrimPrefix(identifier, "podcasts/")
	err = db.QueryRow(query, identifier, identifier, identifier, identifier, cleanIdent).Scan(&realID)
	if err != nil {
		realID = identifier
	}

	activeVal := 1
	if !autoDownload {
		activeVal = 0
	}
	_, _ = db.Exec("UPDATE podcasts SET active = ? WHERE id = ?", activeVal, realID)

	autoDlVal := 0
	if autoDownload {
		autoDlVal = 1
	}
	autoClVal := 0
	if autoCleanup {
		autoClVal = 1
	}

	upsert := `INSERT INTO podcast_settings (
		podcast_id,
		episode_numbering,
		auto_download,
		auto_update,
		auto_cleanup,
		auto_cleanup_days,
		replace_invalid_characters,
		use_existing_filename,
		replacement_strategy,
		episode_format,
		podcast_format,
		direct_paths,
		activated,
		podcast_prefill,
		use_one_cover_for_all_episodes,
		nfo_format,
		cover_filename,
		auto_transcribe
	) VALUES (
		?, 0, ?, 1, ?, ?, 1, 0, 'replace-with-dash-and-underscore', '{}', '{}', 0, 1, 0, 0, 'off', 'image', 0
	) ON CONFLICT(podcast_id) DO UPDATE SET
		auto_download = excluded.auto_download,
		auto_cleanup = excluded.auto_cleanup,
		auto_cleanup_days = excluded.auto_cleanup_days,
		activated = 1`

	_, err = db.Exec(upsert, realID, autoDlVal, autoClVal, autoCleanupDays)
	return err
}
