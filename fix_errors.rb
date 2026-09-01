content = File.read('audio.go')
content.sub!(/absPath, err := filepath\.Abs\(filePath\)\n\tif err != nil \{\n\t\treturn 0\.0\n\t\}/, "absPath, err := filepath.Abs(filePath)\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"failed to get absolute path: %v\\n\", err)\n\t\treturn 0.0\n\t}")
content.sub!(/output, err := cmd\.Output\(\)\n\tif err != nil \{\n\t\treturn 0\.0\n\t\}/, "output, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"ffprobe failed: %v, output: %s\\n\", err, string(output))\n\t\treturn 0.0\n\t}")

content.sub!(/absPath, err := filepath\.Abs\(filePath\)\n\tif err != nil \{\n\t\treturn nil\n\t\}/, "absPath, err := filepath.Abs(filePath)\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"failed to get absolute path: %v\\n\", err)\n\t\treturn nil\n\t}")
content.sub!(/output, err := cmd\.Output\(\)\n\tif err != nil \{\n\t\treturn nil\n\t\}/, "output, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"ffprobe failed: %v, output: %s\\n\", err, string(output))\n\t\treturn nil\n\t}")

content.sub!(/if err := scpInCmd\.Run\(\); err != nil \{\n\t\t\treturn cutAudioFFmpegWithHost\(inputFile, keepSegments, outputFile, ""\)\n\t\t\}/, "if err := scpInCmd.Run(); err != nil {\n\t\t\tfmt.Fprintf(os.Stderr, \"scp in failed: %v\\n\", err)\n\t\t\treturn cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, \"\")\n\t\t}")
content.sub!(/if err := remFFmpegCmd\.Run\(\); err != nil \{\n\t\t\treturn cutAudioFFmpegWithHost\(inputFile, keepSegments, outputFile, ""\)\n\t\t\}/, "if err := remFFmpegCmd.Run(); err != nil {\n\t\t\tfmt.Fprintf(os.Stderr, \"remote ffmpeg failed: %v\\n\", err)\n\t\t\treturn cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, \"\")\n\t\t}")
content.sub!(/if err := scpOutCmd\.Run\(\); err != nil \{\n\t\t\treturn cutAudioFFmpegWithHost\(inputFile, keepSegments, outputFile, ""\)\n\t\t\}/, "if err := scpOutCmd.Run(); err != nil {\n\t\t\tfmt.Fprintf(os.Stderr, \"scp out failed: %v\\n\", err)\n\t\t\treturn cutAudioFFmpegWithHost(inputFile, keepSegments, outputFile, \"\")\n\t\t}")

content.sub!(/return cmd\.Run\(\) == nil/, "out, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"ffmpeg failed: %v, output: %s\\n\", err, string(out))\n\t\treturn false\n\t}\n\treturn true")
content.sub!(/return cmd\.Run\(\) == nil/, "out, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"ffmpeg convert failed: %v, output: %s\\n\", err, string(out))\n\t\treturn false\n\t}\n\treturn true")
content.sub!(/return cmd\.Run\(\) == nil/, "out, err := cmd.CombinedOutput()\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"ffmpeg truncate failed: %v, output: %s\\n\", err, string(out))\n\t\treturn false\n\t}\n\treturn true")

File.write('audio.go', content)

content = File.read('output.go')
content.sub!(/data, err := readFile\(src\)\n\tif err != nil \{\n\t\treturn\n\t\}/, "data, err := readFile(src)\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"copyFile read error: %v\\n\", err)\n\t\treturn\n\t}")
content.sub!(/entries, err := os\.ReadDir\(dir\)\n\tif err != nil \{\n\t\treturn files\n\t\}/, "entries, err := os.ReadDir(dir)\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"findMP3Files read error: %v\\n\", err)\n\t\treturn files\n\t}")
File.write('output.go', content)

content = File.read('ads.go')
content.sub!(/fmt\.Printf\("Warning during LLM ad detection: %v\\n", err\)/, 'fmt.Fprintf(os.Stderr, "Error during LLM ad detection: %v\n", err)')
content.sub!(/fmt\.Printf\("Warning during keyword extraction: %v\\n", err\)/, 'fmt.Fprintf(os.Stderr, "Error during keyword extraction: %v\n", err)')
content.sub!(/if err := json\.Unmarshal\(\[\]byte\(content\[start:end\+1\]\), &ads\); err != nil \{\n\t\treturn nil\n\t\}/, "if err := json.Unmarshal([]byte(content[start:end+1]), &ads); err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"Error unmarshaling ads JSON: %v\\n\", err)\n\t\treturn nil\n\t}")
File.write('ads.go', content)
