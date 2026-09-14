package cli

import (
	"encoding/json"
	"fmt"
	"github.com/sarielhp/clihelp"
	"os"
	"path/filepath"
	"pod/pkg/config"
	"pod/pkg/pipeline"
	"pod/pkg/podcast"
	"sort"
	"strconv"
)

func buildQueuePrioritySubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "priority",
		Description: "Show or set a podcast's persistent queue priority (0–10)",
		UsageLine:   "pod queue priority <podcast-id> [0–10]",
		Args:        clihelp.RangeArgs(1, 2),
		Run: func(ctx *clihelp.Context) error {
			*action = "queue"
			opts.QueueSubcmd = "priority"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func handleQueuePriority(root string, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("use queue priority <podcast-id> [0–10]")
	}
	priority := 0
	if len(args) == 2 {
		var err error
		priority, err = strconv.Atoi(args[1])
		if err != nil || priority < 0 || priority > 10 {
			return fmt.Errorf("priority must be an integer between 0 and 10")
		}
	}
	res, err := resolveQueueTarget(root, args[0])
	if err != nil {
		return err
	}
	if !res.IsPodcast() {
		return fmt.Errorf("persistent priority can only be assigned to a podcast")
	}
	pod := res.Podcast
	data, err := os.ReadFile(filepath.Join(pod.Dir, config.PodcastConfigFileName))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	cfg := config.DefaultPodcastConfig(nil)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("read podcast settings: %w", err)
		}
	}
	if len(args) == 2 {
		cfg.Priority = priority
		if err := config.SavePodcastConfig(pod.Dir, cfg); err != nil {
			return err
		}
	}
	fmt.Printf("Podcast %s [%s] priority: %d\n", pod.Title, pod.ShortID, cfg.Priority)
	return nil
}

func sortQueueItems(items []queueEpisodeItem) {
	for i := range items {
		items[i].Priority = podcast.EpisodePriority(items[i].PodcastDir, items[i].AudioPath)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Priority > items[j].Priority })
}

func clearPodcastQueue(dir string) error {
	entries, err := pipeline.ReadQueue(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path, err := pipeline.ResolveQueueAudioPath(dir, entry)
		if err != nil {
			continue
		}
		if err := pipeline.ClearQueuePriority(path); err != nil {
			return err
		}
	}
	return pipeline.UpdateQueue(dir, func([]string) []string {
		return []string{}
	})
}
