package main

import (
	"sync"
	"testing"
	"time"
)

// The render path used to read globalPlayer.Current, nil-check it, and then
// dereference it ~13 lines later with no lock, while the ffplay wait goroutine
// could nil it in between. Run the two against each other.
func TestRenderMiniPlayerBarIsSafeWhileTheTrackChanges(t *testing.T) {
	m := makeTestModel()
	m.screen = screenPodcasts
	m.width = 80
	t.Cleanup(globalPlayer.Stop)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			globalPlayer.PlayTrack(PlayerTrack{Title: "Ep", Podcast: "Pod", Duration: 120})
			globalPlayer.Stop()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = m.renderMiniPlayerBar()
			_ = m.drawPlayQueueScreen()
		}
	}()

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
}
