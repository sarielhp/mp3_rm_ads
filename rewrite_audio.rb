content = File.read('audio.go')

content.sub!('func getAudioDuration(filePath string) float64 {', 'func getAudioDuration(filePath string) (float64, error) {')
content.sub!('return 0.0', 'return 0.0, fmt.Errorf("failed to get absolute path: %w", err)')
content.sub!(/output, err := cmd.Output\(\)\s+if err != nil \{\s+return 0.0\s+\}/, "output, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\treturn 0.0, fmt.Errorf(\"ffprobe failed: %w, output: %s\", err, string(output))\n\t}")
content.sub!('return dur', 'return dur, nil')

content.sub!('func extractID3Tags(filePath string) map[string]string {', 'func extractID3Tags(filePath string) (map[string]string, error) {')
content.sub!('return nil', 'return nil, fmt.Errorf("failed to get absolute path: %w", err)')
content.sub!(/output, err := cmd.Output\(\)\s+if err != nil \{\s+return nil\s+\}/, "output, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"ffprobe failed: %w, output: %s\", err, string(output))\n\t}")
content.sub!('return tags', 'return tags, nil')

content.sub!('func cutAudioFFmpeg(inputFile string, keepSegments [][2]float64, outputFile string) bool {', 'func cutAudioFFmpeg(inputFile string, keepSegments [][2]float64, outputFile string) error {')
content.sub!('return cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, "")', 'return cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, "")')

content.sub!('func cutAudioFFmpegWithHost(inputFile string, keepSegments [][2]float64, outputFile, remoteHost string) bool {', 'func cutAudioFFmpegWithHost(inputFile string, keepSegments [][2]float64, outputFile, remoteHost string) error {')
content.sub!('return false', 'return fmt.Errorf("no keep segments provided")')
content.sub!('remIn := fmt.Sprintf("/tmp/%s_in%s", tempID, ext)', 'remIn := fmt.Sprintf(".work/%s_in%s", tempID, ext)')
content.sub!('remOut := fmt.Sprintf("/tmp/%s_out%s", tempID, filepath.Ext(absOutput))', 'remOut := fmt.Sprintf(".work/%s_out%s", tempID, filepath.Ext(absOutput))')

content.gsub!(/if err := scpInCmd.Run\(\); err != nil \{\s+return cutAudioFFmpegWithHost\(inputFile, keepSegments, outputFile, ""\)\s+\}/, "if err := scpInCmd.Run(); err != nil {\n\t\t\tfmt.Fprintf(os.Stderr, \"scp in failed: %v\\n\", err)\n\t\t\treturn cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, \"\")\n\t\t}")
content.gsub!(/if err := remFFmpegCmd.Run\(\); err != nil \{\s+return cutAudioFFmpegWithHost\(inputFile, keepSegments, outputFile, ""\)\s+\}/, "if err := remFFmpegCmd.Run(); err != nil {\n\t\t\tfmt.Fprintf(os.Stderr, \"remote ffmpeg failed: %v\\n\", err)\n\t\t\treturn cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, \"\")\n\t\t}")
content.gsub!(/if err := scpOutCmd.Run\(\); err != nil \{\s+return cutAudioFFmpegWithHost\(inputFile, keepSegments, outputFile, ""\)\s+\}/, "if err := scpOutCmd.Run(); err != nil {\n\t\t\tfmt.Fprintf(os.Stderr, \"scp out failed: %v\\n\", err)\n\t\t\treturn cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, \"\")\n\t\t}")
content.sub!('return true', 'return nil')

content.sub!('return cmd.Run() == nil', "out, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\treturn fmt.Errorf(\"ffmpeg failed: %w, output: %s\", err, string(out))\n\t}\n\treturn nil")

content.sub!('func convertToWAV(inputPath, wavPath string) bool {', 'func convertToWAV(inputPath, wavPath string) error {')
content.sub!('return cmd.Run() == nil', "out, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\treturn fmt.Errorf(\"ffmpeg convert failed: %w, output: %s\", err, string(out))\n\t}\n\treturn nil")

content.sub!('func truncateAudio(inputPath, outputPath string, durationSec float64) bool {', 'func truncateAudio(inputPath, outputPath string, durationSec float64) error {')
content.sub!('return cmd.Run() == nil', "out, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\treturn fmt.Errorf(\"ffmpeg truncate failed: %w, output: %s\", err, string(out))\n\t}\n\treturn nil")

content.sub!('dur := getAudioDuration(filePath)', 'dur, _ := getAudioDuration(filePath)')

File.write('audio.go', content)
