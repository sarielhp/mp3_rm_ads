package podcast

import (
	"fmt"
	"strings"

	"pod/pkg/backend"
	"pod/pkg/config"
)

type PodcastGroupKind string

const (
	GroupKindNone         PodcastGroupKind = ""
	GroupKindAll          PodcastGroupKind = "all"
	GroupKindFavorites    PodcastGroupKind = "favorites"
	GroupKindNonFavorites PodcastGroupKind = "non-favorites"
	GroupKindSingle       PodcastGroupKind = "single"
)

func ParsePodcastGroupKind(query string) (PodcastGroupKind, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	switch q {
	case "all", "*":
		return GroupKindAll, true
	case "fav", "favs", "favorite", "favorites":
		return GroupKindFavorites, true
	case "not-fav", "not-favs", "not-favorite", "not-favorites",
		"non-fav", "non-favs", "non-favorite", "non-favorites", "unfav":
		return GroupKindNonFavorites, true
	default:
		return GroupKindNone, false
	}
}

type ResolvedPodcastGroup struct {
	Kind    PodcastGroupKind
	Label   string
	Entries []PodcastDirEntry
}

func ResolvePodcastGroup(podcastsDir, query string) (*ResolvedPodcastGroup, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty podcast query")
	}

	entries := ScanPodcastDirs(podcastsDir)
	if len(entries) == 0 {
		return nil, fmt.Errorf("no podcasts found in %s", podcastsDir)
	}

	if kind, ok := ParsePodcastGroupKind(q); ok {
		return filterLocalGroupEntries(entries, kind)
	}

	matched, err := MatchLocalPodcasts(entries, q)
	if err != nil {
		return nil, err
	}
	return &ResolvedPodcastGroup{
		Kind:    GroupKindSingle,
		Label:   matched.Title,
		Entries: []PodcastDirEntry{*matched},
	}, nil
}

func filterLocalGroupEntries(entries []PodcastDirEntry, kind PodcastGroupKind) (*ResolvedPodcastGroup, error) {
	switch kind {
	case GroupKindAll:
		return &ResolvedPodcastGroup{
			Kind:    GroupKindAll,
			Label:   "all podcasts",
			Entries: entries,
		}, nil
	case GroupKindFavorites:
		var favs []PodcastDirEntry
		for _, e := range entries {
			pCfg := config.LoadPodcastConfig(e.Dir, config.PodcastConfig{})
			if pCfg.Favorite {
				favs = append(favs, e)
			}
		}
		return &ResolvedPodcastGroup{
			Kind:    GroupKindFavorites,
			Label:   "favorite podcasts",
			Entries: favs,
		}, nil
	case GroupKindNonFavorites:
		var nonFavs []PodcastDirEntry
		for _, e := range entries {
			pCfg := config.LoadPodcastConfig(e.Dir, config.PodcastConfig{})
			if !pCfg.Favorite {
				nonFavs = append(nonFavs, e)
			}
		}
		return &ResolvedPodcastGroup{
			Kind:    GroupKindNonFavorites,
			Label:   "non-favorite podcasts",
			Entries: nonFavs,
		}, nil
	default:
		return nil, fmt.Errorf("unknown group kind %q", kind)
	}
}

type ResolvedBackendGroup struct {
	Kind     PodcastGroupKind
	Label    string
	Podcasts []backend.Podcast
}

func ResolveBackendPodcastGroup(podcasts []backend.Podcast, podcastsDir, query string) (*ResolvedBackendGroup, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty podcast query")
	}

	if kind, ok := ParsePodcastGroupKind(q); ok {
		return filterBackendGroup(podcasts, podcastsDir, kind)
	}

	matched, err := MatchBackendPodcasts(podcasts, q)
	if err != nil {
		return nil, err
	}
	return &ResolvedBackendGroup{
		Kind:     GroupKindSingle,
		Label:    BackendPodcastTitle(*matched),
		Podcasts: []backend.Podcast{*matched},
	}, nil
}

func isBackendPodcastFavorite(p backend.Podcast, podcastsDir string) bool {
	dir := p.Path
	if dir == "" {
		dir = FindPodcastDirForItem(p, podcastsDir)
	}
	if dir == "" {
		return false
	}
	pCfg := config.LoadPodcastConfig(dir, config.PodcastConfig{})
	return pCfg.Favorite
}

func filterBackendGroup(podcasts []backend.Podcast, podcastsDir string, kind PodcastGroupKind) (*ResolvedBackendGroup, error) {
	switch kind {
	case GroupKindAll:
		return &ResolvedBackendGroup{
			Kind:     GroupKindAll,
			Label:    "all podcasts",
			Podcasts: podcasts,
		}, nil
	case GroupKindFavorites:
		var favs []backend.Podcast
		for _, p := range podcasts {
			if isBackendPodcastFavorite(p, podcastsDir) {
				favs = append(favs, p)
			}
		}
		return &ResolvedBackendGroup{
			Kind:     GroupKindFavorites,
			Label:    "favorite podcasts",
			Podcasts: favs,
		}, nil
	case GroupKindNonFavorites:
		var nonFavs []backend.Podcast
		for _, p := range podcasts {
			if !isBackendPodcastFavorite(p, podcastsDir) {
				nonFavs = append(nonFavs, p)
			}
		}
		return &ResolvedBackendGroup{
			Kind:     GroupKindNonFavorites,
			Label:    "non-favorite podcasts",
			Podcasts: nonFavs,
		}, nil
	default:
		return nil, fmt.Errorf("unknown group kind %q", kind)
	}
}
