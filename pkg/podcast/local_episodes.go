package podcast

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"pod/pkg/backend"
	"pod/pkg/pipeline"
	"pod/pkg/util"
)

type LocalEpisodeMeta struct {
	Path        string
	Filename    string
	Title       string
	GUID        string
	PubDate     string
	PublishedAt int64
	DurationSec float64
	SizeBytes   int64
	Description string
}

func CollectLocalEpisodes(podDir string, feedEpisodes []backend.FeedEpisode) []LocalEpisodeMeta {
	mp3Files := util.FindMP3Files(podDir)
	if len(mp3Files) == 0 {
		return nil
	}

	feedMap := buildFeedEpisodeLookup(feedEpisodes)
	var list []LocalEpisodeMeta
	for _, path := range mp3Files {
		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() {
			continue
		}
		item := buildEpisodeMeta(path, podDir, fi, feedMap)
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].PublishedAt != list[j].PublishedAt {
			return list[i].PublishedAt > list[j].PublishedAt
		}
		return list[i].Filename < list[j].Filename
	})
	return list
}

func buildFeedEpisodeLookup(eps []backend.FeedEpisode) map[string]backend.FeedEpisode {
	m := make(map[string]backend.FeedEpisode)
	for _, ep := range eps {
		titleKey := strings.ToLower(strings.TrimSpace(ep.Title))
		if titleKey != "" {
			m[titleKey] = ep
			sanitized := strings.ToLower(SanitizeTitle(ep.Title))
			m[sanitized] = ep
			pubMs := GetPubMS(ep)
			var pubTime time.Time
			if pubMs > 0 {
				pubTime = time.UnixMilli(pubMs).UTC()
			}
			fn := strings.ToLower(FormatEpisodeFilename(pubTime, ep.Episode, ep.Title))
			m[fn] = ep
			m[strings.TrimSuffix(fn, ".mp3")] = ep
		}
		if ep.GUID != "" {
			m[ep.GUID] = ep
		}
	}
	return m
}

func buildEpisodeMeta(path, podDir string, fi os.FileInfo, feedMap map[string]backend.FeedEpisode) LocalEpisodeMeta {
	relPath := filepath.Base(path)
	if podDir != "" {
		if r, err := filepath.Rel(podDir, path); err == nil && r != "" && !strings.HasPrefix(r, "..") {
			relPath = filepath.ToSlash(r)
		}
	}
	fn := relPath
	title := EpisodeTitleFromPath(path)
	cleanKey := strings.ToLower(SanitizeTitle(title))
	matched, hasMatch := feedMap[cleanKey]
	if !hasMatch {
		matched, hasMatch = feedMap[strings.ToLower(strings.TrimSpace(title))]
	}
	if !hasMatch {
		baseStem := strings.ToLower(util.StripExt(filepath.Base(path)))
		matched, hasMatch = feedMap[baseStem]
	}
	if !hasMatch {
		stripped := strings.ToLower(SanitizeTitle(StripEpisodeFilenamePrefix(title)))
		matched, hasMatch = feedMap[stripped]
	}
	if !hasMatch {
		strippedBase := strings.ToLower(SanitizeTitle(StripEpisodeFilenamePrefix(util.StripExt(filepath.Base(path)))))
		matched, hasMatch = feedMap[strippedBase]
	}

	guid := ""
	pubDateStr := ""
	pubMs := int64(0)
	desc := ""
	durSec := float64(0)

	if hasMatch {
		guid = matched.GUID
		pubMs = GetPubMS(matched)
		desc = matched.Description
		durSec = matched.DurationSeconds
		if matched.Title != "" {
			title = matched.Title
		}
	}
	if guid == "" {
		h := sha256.Sum256([]byte(relPath))
		guid = "pod:ep:" + hex.EncodeToString(h[:8])
	}
	if pubMs <= 0 {
		if pubTime := GetEpisodePublicationTime(path); !pubTime.IsZero() {
			pubMs = pubTime.UnixMilli()
		} else {
			pubMs = fi.ModTime().UnixMilli()
		}
	}
	if durSec <= 0 {
		if st, _ := pipeline.LoadEpisodeStatus(pipeline.StatusPathFor(path)); st != nil {
			if st.Cleaned.DurationSec > 0 {
				durSec = st.Cleaned.DurationSec
			} else if st.Original.DurationSec > 0 {
				durSec = st.Original.DurationSec
			}
		}
	}
	pubDateStr = time.UnixMilli(pubMs).UTC().Format(time.RFC1123Z)
	if desc == "" {
		desc = title
	}

	return LocalEpisodeMeta{
		Path:        path,
		Filename:    fn,
		Title:       title,
		GUID:        guid,
		PubDate:     pubDateStr,
		PublishedAt: pubMs,
		DurationSec: durSec,
		SizeBytes:   fi.Size(),
		Description: desc,
	}
}

func findLocalCover(podDir string) string {
	candidates := []string{"cover.jpg", "cover.png", "cover.jpeg", "cover.webp", "folder.jpg", "folder.png"}
	for _, c := range candidates {
		p := filepath.Join(podDir, c)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return p
		}
	}
	detailsPath := filepath.Join(podDir, ".cache", "details", "cover.jpg")
	if fi, err := os.Stat(detailsPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
		target := filepath.Join(podDir, "cover.jpg")
		if err := util.CopyFileErr(detailsPath, target); err == nil {
			return target
		}
		return detailsPath
	}
	cDir := CacheDirForPodcast(podDir)
	for _, c := range candidates {
		p := filepath.Join(cDir, c)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			target := filepath.Join(podDir, filepath.Base(p))
			if err := util.CopyFileErr(p, target); err == nil {
				return target
			}
			return p
		}
	}
	return ""
}

func EnsurePodcastCover(podDir string, sub *Subscription) string {
	if podDir == "" {
		return ""
	}
	existing := findLocalCover(podDir)
	if existing != "" {
		return existing
	}

	imageURL := resolveCoverImageURL(sub)
	if imageURL == "" {
		return ""
	}

	ext := ".jpg"
	low := strings.ToLower(imageURL)
	if strings.Contains(low, ".png") {
		ext = ".png"
	}
	destPath := filepath.Join(podDir, "cover"+ext)
	if err := DownloadCoverImage(imageURL, destPath); err == nil {
		return destPath
	}
	return ""
}

func resolveCoverImageURL(sub *Subscription) string {
	if sub == nil {
		return ""
	}
	if sub.ImageURL != "" {
		return sub.ImageURL
	}
	if sub.FeedURL == "" {
		return ""
	}
	if entry := DefaultFeedCache().Get(sub.FeedURL); entry != nil && entry.ImageURL != "" {
		sub.ImageURL = entry.ImageURL
		return entry.ImageURL
	}
	if doc, err := FetchFeedDoc(sub.FeedURL); err == nil && doc != nil && doc.ImageURL != "" {
		sub.ImageURL = doc.ImageURL
		if entry := DefaultFeedCache().Get(sub.FeedURL); entry != nil {
			entry.ImageURL = doc.ImageURL
			DefaultFeedCache().Put(sub.FeedURL, entry)
		}
		return doc.ImageURL
	}
	return ""
}
