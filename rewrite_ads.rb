content = File.read('ads.go')

content.sub!('func detectAdsLLM(transcriptText string, profile LLMProfile) []AdSegment {', 'func detectAdsLLM(transcriptText string, profile LLMProfile) ([]AdSegment, error) {')
content.sub!(/fmt\.Printf\("Warning during LLM ad detection: %v\\n", err\)\n\t\treturn nil/, "return nil, fmt.Errorf(\"LLM ad detection failed: %w\", err)")
content.sub!('return extractJSONArray(content)', 'return extractJSONArray(content)') # No change here yet

content.sub!('func extractJSONArray(content string) []AdSegment {', 'func extractJSONArray(content string) ([]AdSegment, error) {')
content.sub!('if start < 0 {', "if start < 0 {\n\t\treturn nil, fmt.Errorf(\"no JSON array found\")")
content.sub!('return nil', '') # Wait, I will use targeted regex for returns in extractJSONArray

# I'll just use manual targeted replace for extractJSONArray:
content.gsub!(/return nil/) do |match|
  "return nil, fmt.Errorf(\"JSON extraction error\")"
end
# This might hit other places, let's fix that.
