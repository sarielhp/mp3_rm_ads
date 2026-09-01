#!/usr/bin/env ruby

content = File.read('output.go')
content.gsub!(/func copyFile\(src, dst string\) \{/, 'func copyFile(src, dst string) error {')
content.gsub!(/data, err := readFile\(src\)\n\tif err != nil \{\n\t\treturn\n\t\}/, "data, err := readFile(src)\n\tif err != nil {\n\t\treturn err\n\t}")
content.gsub!(/writeFile\(dst, data\)\n\}/, "writeFile(dst, data)\n\treturn nil\n}")

content.gsub!(/func findMP3Files\(dir string\) \[\]string \{/, 'func findMP3Files(dir string) ([]string, error) {')
content.gsub!(/var files \[\]string\n\tentries, err := os\.ReadDir\(dir\)\n\tif err != nil \{\n\t\treturn files\n\t\}/, "var files []string\n\tentries, err := os.ReadDir(dir)\n\tif err != nil {\n\t\treturn files, err\n\t}")
content.gsub!(/subFiles := findMP3Files\(filepath\.Join\(dir, entry\.Name\(\)\)\)\n\t\t\tfiles = append\(files, subFiles\.\.\.\)/, "subFiles, _ := findMP3Files(filepath.Join(dir, entry.Name()))\n\t\t\tfiles = append(files, subFiles...)")
content.gsub!(/return files\n\}/, "return files, nil\n}")

# For convertJSONToSRT and convertJSONToTXT: returning strings is fine, but they swallow error, maybe return (string, error)?
# For now, changing signature might be too big for those 2, let's fix them to return (string, error)
content.gsub!(/func convertJSONToSRT\(inputFile string, data \*TranscriptionData, customPath string, quiet bool\) string \{/, 'func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) (string, error) {')
content.gsub!(/return ""\n/, "return \"\", err\n")
# wait, the file-not-found one doesn't have an err variable.
content.gsub!(/fmt\.Fprintf\(os\.Stderr, "Error: Cannot convert to SRT, JSON file not found: '%s'\\n", inputFile\)\n\t\t\treturn "", err/, "return \"\", fmt.Errorf(\"JSON file not found: %s\", inputFile)")
content.gsub!(/return srtFile\n/, "return srtFile, nil\n")

content.gsub!(/func convertJSONToTXT\(inputFile string, data \*TranscriptionData, totalDuration float64, customPath string, quiet bool\) string \{/, 'func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) (string, error) {')
# apply same replacement for file not found
content.gsub!(/fmt\.Fprintf\(os\.Stderr, "Error: Cannot convert to TXT, JSON file not found: '%s'\\n", inputFile\)\n\t\t\treturn "", err/, "return \"\", fmt.Errorf(\"JSON file not found: %s\", inputFile)")
content.gsub!(/return txtFile\n/, "return txtFile, nil\n")

File.write('output.go', content)
