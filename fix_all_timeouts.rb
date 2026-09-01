#!/usr/bin/env ruby

files = ["pm_simplecast.go", "feed_cache.go", "pkg/backend/podfetch_api.go"]
files.each do |f|
  content = File.read(f)
  content = content.gsub(/Timeout: \d+ \* time\.Second/, "Timeout: 60 * time.Second")
  File.write(f, content)
end
