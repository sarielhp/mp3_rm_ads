package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type OrphanPodcast struct {
	Item             PodcastItem
	Reason           string
	DuplicateOfID    string
	DuplicateOfTitle string
	EpisodeCount     int
}

type CleanOrphansOptions struct {
	DryRun  bool
	Force   bool
	Quiet   bool
	Verbose bool
	In      io.Reader
	Out     io.Writer
}

type CleanOrphansResult struct {
	ScannedCount int
	OrphanCount  int
	DeletedCount int
	FailedCount  int
	Orphans      []OrphanPodcast
	Errors       []error
}

func normalizeFeedURL(u string) string {
	trimmed := strings.TrimSpace(u)
	trimmed = strings.TrimRight(trimmed, "/")
	return strings.ToLower(trimmed)
}

func FindOrphanPodcasts(podcasts []PodcastItem) []OrphanPodcast {
	var orphans []OrphanPodcast
	orphanedIDs := make(map[string]bool)

	for _, item := range podcasts {
		feedURL := strings.TrimSpace(item.Media.Metadata.FeedURL)
		if feedURL == "" {
			orphans = append(orphans, OrphanPodcast{
				Item:         item,
				Reason:       "Missing or empty RSS feed URL",
				EpisodeCount: len(item.Media.Episodes),
			})
			orphanedIDs[item.ID] = true
		}
	}

	feedGroups := make(map[string][]PodcastItem)
	for _, item := range podcasts {
		if orphanedIDs[item.ID] {
			continue
		}
		normURL := normalizeFeedURL(item.Media.Metadata.FeedURL)
		if normURL != "" {
			feedGroups[normURL] = append(feedGroups[normURL], item)
		}
	}

	for _, group := range feedGroups {
		if len(group) <= 1 {
			continue
		}

		sort.SliceStable(group, func(i, j int) bool {
			if len(group[i].Media.Episodes) != len(group[j].Media.Episodes) {
				return len(group[i].Media.Episodes) > len(group[j].Media.Episodes)
			}
			scoreI := 0
			if group[i].Media.CoverPath != "" {
				scoreI++
			}
			if group[i].Media.Metadata.Author != "" {
				scoreI++
			}
			if group[i].Media.Metadata.Description != "" {
				scoreI++
			}

			scoreJ := 0
			if group[j].Media.CoverPath != "" {
				scoreJ++
			}
			if group[j].Media.Metadata.Author != "" {
				scoreJ++
			}
			if group[j].Media.Metadata.Description != "" {
				scoreJ++
			}

			if scoreI != scoreJ {
				return scoreI > scoreJ
			}
			return group[i].ID < group[j].ID
		})

		primary := group[0]
		primaryTitle := primary.Media.Metadata.Title
		if primaryTitle == "" {
			primaryTitle = "Untitled"
		}

		for _, dup := range group[1:] {
			if orphanedIDs[dup.ID] {
				continue
			}
			orphans = append(orphans, OrphanPodcast{
				Item:             dup,
				Reason:           fmt.Sprintf("Duplicate feed URL (matches %q, keeping ID %s with %d episode(s))", primaryTitle, primary.ID, len(primary.Media.Episodes)),
				DuplicateOfID:    primary.ID,
				DuplicateOfTitle: primaryTitle,
				EpisodeCount:     len(dup.Media.Episodes),
			})
			orphanedIDs[dup.ID] = true
		}
	}

	return orphans
}

func RunCleanOrphans(client Backend, opts CleanOrphansOptions) (CleanOrphansResult, error) {
	if client == nil {
		return CleanOrphansResult{}, fmt.Errorf("backend client is nil")
	}
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	if !opts.Quiet {
		fmt.Fprintln(opts.Out, "Scanning Audiobookshelf library for orphaned podcasts...")
	}

	podcasts, err := client.Podcasts()
	if err != nil {
		return CleanOrphansResult{}, fmt.Errorf("failed to fetch podcasts: %w", err)
	}

	orphans := FindOrphanPodcasts(podcasts)
	res := CleanOrphansResult{
		ScannedCount: len(podcasts),
		OrphanCount:  len(orphans),
		Orphans:      orphans,
	}

	if len(orphans) == 0 {
		if !opts.Quiet {
			fmt.Fprintf(opts.Out, "No orphaned or fake podcast entries found (scanned %d podcasts).\n", len(podcasts))
		}
		return res, nil
	}

	if !opts.Quiet {
		fmt.Fprintf(opts.Out, "\nFound %d orphaned / fake podcast entry(ies) (out of %d scanned):\n", len(orphans), len(podcasts))
		for idx, o := range orphans {
			title := o.Item.Media.Metadata.Title
			if title == "" {
				title = "Untitled Podcast"
			}
			fmt.Fprintf(opts.Out, "  %d. %s\n", idx+1, title)
			fmt.Fprintf(opts.Out, "     ID: %s | Episodes: %d\n", o.Item.ID, o.EpisodeCount)
			fmt.Fprintf(opts.Out, "     Reason: %s\n", o.Reason)
			if opts.Verbose && o.Item.RelPath != "" {
				fmt.Fprintf(opts.Out, "     Path: %s\n", o.Item.RelPath)
			}
		}
		fmt.Fprintln(opts.Out)
	}

	if opts.DryRun {
		if !opts.Quiet {
			fmt.Fprintf(opts.Out, "Dry run enabled: %d podcast(s) identified but none were deleted.\n", len(orphans))
		}
		return res, nil
	}

	if !opts.Force {
		fmt.Fprintf(opts.Out, "Are you sure you want to delete these %d orphaned podcast(s) from Audiobookshelf? [y/N]: ", len(orphans))
		reader := bufio.NewReader(opts.In)
		input, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return res, fmt.Errorf("failed to read confirmation: %w", err)
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			if !opts.Quiet {
				fmt.Fprintln(opts.Out, "Aborted. No podcasts were deleted.")
			}
			return res, nil
		}
	}

	if !opts.Quiet {
		fmt.Fprintln(opts.Out, "Deleting orphaned podcasts from Audiobookshelf...")
	}

	for _, o := range orphans {
		err := client.DeletePodcast(o.Item.ID)
		title := o.Item.Media.Metadata.Title
		if title == "" {
			title = "Untitled Podcast"
		}
		if err != nil {
			res.FailedCount++
			res.Errors = append(res.Errors, err)
			if !opts.Quiet {
				fmt.Fprintf(opts.Out, "  [✗] Failed to delete %q (ID: %s): %v\n", title, o.Item.ID, err)
			}
		} else {
			res.DeletedCount++
			if !opts.Quiet {
				fmt.Fprintf(opts.Out, "  [✓] Deleted %q (ID: %s)\n", title, o.Item.ID)
			}
		}
	}

	if !opts.Quiet {
		if res.FailedCount == 0 {
			fmt.Fprintf(opts.Out, "\nSuccessfully deleted %d orphaned podcast(s) from Audiobookshelf.\n", res.DeletedCount)
		} else {
			fmt.Fprintf(opts.Out, "\nDeleted %d orphaned podcast(s), %d failed.\n", res.DeletedCount, res.FailedCount)
		}
	}

	return res, nil
}

func handleServerCleanOrphans(config Config, cli CLIOptions) {
	b, err := getBackend(config, cli.Quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error connecting to backend: %v", err))
	}

	opts := CleanOrphansOptions{
		DryRun:  cli.DryRun,
		Force:   cli.ForceDelete,
		Quiet:   cli.Quiet,
		Verbose: cli.Verbose,
		In:      os.Stdin,
		Out:     os.Stdout,
	}

	res, err := RunCleanOrphans(b, opts)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error cleaning orphans: %v", err))
	}
	if res.FailedCount > 0 && !cli.DryRun {
		fatalError("Some orphans failed to be deleted\n")
	}
}
