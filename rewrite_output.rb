content = File.read('output.go')

content.sub!('func copyFile(src, dst string) {', 'func copyFile(src, dst string) error {')
content.sub!(/data, err := readFile\(src\)\n\tif err != nil \{\n\t\treturn\n\t\}/, "data, err := readFile(src)\n\tif err != nil {\n\t\treturn err\n\t}")
content.sub!(/writeFile\(dst, data\)\n\}/, "writeFile(dst, data)\n\treturn nil\n}")

content.sub!('func findMP3Files(dir string) []string {', 'func findMP3Files(dir string) ([]string, error) {')
content.sub!(/var files \[\]string\n\tentries, err := os.ReadDir\(dir\)\n\tif err != nil \{\n\t\treturn files\n\t\}/, "var files []string\n\tentries, err := os.ReadDir(dir)\n\tif err != nil {\n\t\treturn files, err\n\t}")
content.sub!(/subFiles := findMP3Files\(filepath\.Join\(dir, entry\.Name\(\)\)\)/, "subFiles, _ := findMP3Files(filepath.Join(dir, entry.Name()))")
content.sub!(/return files\n\}/, "return files, nil\n}")

content.sub!('func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) string {', 'func convertJSONToSRT(inputFile string, data *TranscriptionData, customPath string, quiet bool) (string, error) {')
content.sub!(/fmt\.Fprintf\(os\.Stderr, "Error: Cannot convert to SRT, JSON file not found: '%s'\\n", inputFile\)\n\t\t\treturn ""/, "return \"\", fmt.Errorf(\"JSON file not found: %s\", inputFile)")
content.gsub!(/return ""\n/, "return \"\", err\n")
content.sub!('return srtFile', 'return srtFile, nil')

content.sub!('func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) string {', 'func convertJSONToTXT(inputFile string, data *TranscriptionData, totalDuration float64, customPath string, quiet bool) (string, error) {')
content.sub!(/fmt\.Fprintf\(os\.Stderr, "Error: Cannot convert to TXT, JSON file not found: '%s'\\n", inputFile\)\n\t\t\treturn ""/, "return \"\", fmt.Errorf(\"JSON file not found: %s\", inputFile)")
content.gsub!(/return ""\n/, "return \"\", err\n")
content.sub!('return txtFile', 'return txtFile, nil')

File.write('output.go', content)
