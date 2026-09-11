package cli

import (
	"abs/pkg/podcast"
	"abs/pkg/tui"
	"fmt"
	"github.com/sarielhp/clihelp"
	"os"
	"path/filepath"
	"strings"
)

func buildInfoTranscriptSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "transcript",
		Description: "Read an episode transcript in the system pager",
		UsageLine:   "abs info transcript <episode-id>",
		Args:        clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "info"
			opts.InfoSubcmd = "transcript"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func showEpisodeTranscript(root, id string) error {
	ep, err := resolveTranscriptEpisode(root, id)
	if err != nil {
		return err
	}
	text, err := tui.TranscriptText(ep.Path)
	if err != nil {
		return fmt.Errorf("transcript for [%s]: %w", ep.ShortID, err)
	}
	return pageText(fmt.Sprintf("Transcript [%s]: %s\n\n%s", ep.ShortID, ep.Title, text))
}

func resolveTranscriptEpisode(root, id string) (*podcast.ResolvedEpisode, error) {
	if resolved, err := resolveQueueTarget(root, id); err == nil && resolved.IsEpisode() {
		return resolved.Episode, nil
	}
	var matches []*podcast.ResolvedEpisode
	seen := make(map[string]bool)
	for _, pod := range scanQueuePodcasts(root) {
		err := filepath.WalkDir(pod.Dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == ".work" {
				return filepath.SkipDir
			}
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			var audio string
			for _, suffix := range []string{".transcript.json", ".transcript.txt"} {
				if strings.HasSuffix(path, suffix) {
					audio = strings.TrimSuffix(path, suffix) + ".mp3"
				}
			}
			if audio == "" || seen[audio] {
				return nil
			}
			seen[audio] = true
			episodeID := podcast.EpisodeShortIDReadOnly(pod.Dir, pod.ShortID, audio)
			if strings.EqualFold(id, episodeID) {
				matches = append(matches, &podcast.ResolvedEpisode{Path: audio, ShortID: episodeID, Title: podcast.EpisodeTitleFromPath(audio), PodcastDir: pod.Dir})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("episode %q matches %d retained transcripts", id, len(matches))
	}
	return matches[0], nil
}
