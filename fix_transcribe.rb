#!/usr/bin/env ruby

content = File.read("transcribe.go")

old_go_func = <<~GO
			go func(done chan struct{}) {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						elapsed := time.Since(startTime)
						fmt.Printf("\\rTranscribing audio... Elapsed: %s   ", formatClock(elapsed.Seconds()))
					}
				}
			}(progressDone)
GO

new_go_func = <<~GO
			go func(done chan struct{}) {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						elapsed := time.Since(startTime)
						var progressMsg string
						if dockerContainer != "" {
							prog := pollWhisperDockerProgress(dockerContainer)
							if prog == failedToDecodeSentinel {
								progressMsg = "Error: Decode failed. "
							} else if val, ok := prog.(float64); ok {
								if val <= 1.0 && val > 0.0 {
									progressMsg = fmt.Sprintf("Progress: %.1f%% ", val*100)
								} else if totalDuration > 0 {
									pct := (val / totalDuration) * 100
									if pct > 100 {
										pct = 100
									}
									progressMsg = fmt.Sprintf("Progress: %.1f%% ", pct)
								}
							}
						}
						fmt.Printf("\\rTranscribing audio... %sElapsed: %s   ", progressMsg, formatClock(elapsed.Seconds()))
					}
				}
			}(progressDone)
GO

content = content.sub(old_go_func, new_go_func)
File.write("transcribe.go", content)
