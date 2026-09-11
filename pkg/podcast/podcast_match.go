package podcast

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"abs/pkg/backend"
)

var ErrAmbiguousPodcast = errors.New("ambiguous podcast pattern")

type AmbiguousPodcastMatch struct {
	ID   string
	Name string
}

type AmbiguousPodcastError struct {
	Query   string
	Matches []AmbiguousPodcastMatch
}

func (e *AmbiguousPodcastError) Error() string {
	return FormatPodcastMatches(e.Matches)
}

func (e *AmbiguousPodcastError) Is(target error) bool {
	return target == ErrAmbiguousPodcast
}

func FormatPodcastMatches(matches []AmbiguousPodcastMatch) string {
	var sb strings.Builder
	limit := len(matches)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		sb.WriteString(fmt.Sprintf("%s | %s\n", matches[i].ID, matches[i].Name))
	}
	if len(matches) > 5 {
		sb.WriteString("...\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func PrintAmbiguousMatches(matches []AmbiguousPodcastMatch) {
	fmt.Println(FormatPodcastMatches(matches))
}

func NewAmbiguousPodcastError(query string, matches []AmbiguousPodcastMatch) error {
	return &AmbiguousPodcastError{
		Query:   query,
		Matches: matches,
	}
}

func MatchesPodcastName(name, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	n := strings.ToLower(strings.TrimSpace(name))
	if q == "" || n == "" {
		return false
	}
	if strings.Contains(n, q) {
		return true
	}
	if re, err := regexp.Compile("(?i)" + query); err == nil {
		return re.MatchString(name)
	}
	return false
}

func buildLocalAmbiguousMatches(matches []PodcastDirEntry) []AmbiguousPodcastMatch {
	var result []AmbiguousPodcastMatch
	for _, m := range matches {
		title := m.Title
		if title == "" {
			title = m.FolderName
		}
		result = append(result, AmbiguousPodcastMatch{
			ID:   m.ShortID,
			Name: title,
		})
	}
	return result
}

func resolveLocalPodcastByID(entries []PodcastDirEntry, search string) (*PodcastDirEntry, error) {
	var idMatches []PodcastDirEntry
	for _, p := range entries {
		if strings.EqualFold(p.ShortID, search) {
			idMatches = append(idMatches, p)
		}
	}
	if len(idMatches) == 1 {
		return &idMatches[0], nil
	}
	if len(idMatches) >= 2 {
		return nil, fmt.Errorf("multiple podcasts match ID %q", search)
	}
	if idx, err := strconv.Atoi(search); err == nil && idx >= 1 && idx <= len(entries) {
		p := entries[idx-1]
		return &p, nil
	}
	for _, p := range entries {
		if strings.EqualFold(p.FolderName, search) {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("podcast matching %q not found", search)
}

func MatchLocalPodcasts(entries []PodcastDirEntry, query string) (*PodcastDirEntry, error) {
	search := strings.TrimSpace(query)
	if search == "" {
		return nil, fmt.Errorf("empty podcast query")
	}

	var nameMatches []PodcastDirEntry
	for _, p := range entries {
		title := p.Title
		if title == "" {
			title = p.FolderName
		}
		if MatchesPodcastName(title, search) || (p.FolderName != "" && MatchesPodcastName(p.FolderName, search)) {
			nameMatches = append(nameMatches, p)
		}
	}

	if len(nameMatches) == 1 {
		return &nameMatches[0], nil
	}
	if len(nameMatches) >= 2 {
		return nil, NewAmbiguousPodcastError(search, buildLocalAmbiguousMatches(nameMatches))
	}
	return resolveLocalPodcastByID(entries, search)
}

func BackendPodcastTitle(p backend.Podcast) string {
	if p.Media.Metadata.Title != "" {
		return p.Media.Metadata.Title
	}
	if p.RelPath != "" {
		return filepath.Base(p.RelPath)
	}
	return p.ID
}

func buildBackendAmbiguousMatches(matches []backend.Podcast) []AmbiguousPodcastMatch {
	var result []AmbiguousPodcastMatch
	for _, m := range matches {
		title := BackendPodcastTitle(m)
		id := m.ID
		if id == "" {
			id = m.Media.ID
		}
		if id == "" {
			id = GeneratePodcastShortID(title)
		}
		result = append(result, AmbiguousPodcastMatch{
			ID:   id,
			Name: title,
		})
	}
	return result
}

func resolveBackendPodcastByID(podcasts []backend.Podcast, search string) (*backend.Podcast, error) {
	var idMatches []backend.Podcast
	for i := range podcasts {
		if podcasts[i].ID == search || podcasts[i].Media.ID == search {
			idMatches = append(idMatches, podcasts[i])
		}
	}
	if len(idMatches) == 1 {
		return &idMatches[0], nil
	}
	if len(idMatches) >= 2 {
		return nil, fmt.Errorf("multiple podcasts match ID %q", search)
	}
	for i := range podcasts {
		title := BackendPodcastTitle(podcasts[i])
		if strings.EqualFold(GeneratePodcastShortID(title), search) {
			return &podcasts[i], nil
		}
	}
	if idx, err := strconv.Atoi(search); err == nil && idx >= 1 && idx <= len(podcasts) {
		return &podcasts[idx-1], nil
	}
	return nil, fmt.Errorf("podcast matching %q not found on server", search)
}

func MatchBackendPodcasts(podcasts []backend.Podcast, query string) (*backend.Podcast, error) {
	search := strings.TrimSpace(query)
	if search == "" {
		return nil, fmt.Errorf("empty podcast query")
	}

	var nameMatches []backend.Podcast
	for _, p := range podcasts {
		title := BackendPodcastTitle(p)
		if MatchesPodcastName(title, search) {
			nameMatches = append(nameMatches, p)
		}
	}

	if len(nameMatches) == 1 {
		return &nameMatches[0], nil
	}
	if len(nameMatches) >= 2 {
		return nil, NewAmbiguousPodcastError(search, buildBackendAmbiguousMatches(nameMatches))
	}
	return resolveBackendPodcastByID(podcasts, search)
}
