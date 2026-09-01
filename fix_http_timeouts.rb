#!/usr/bin/env ruby

content = File.read("pkg/backend/abs_episodes.go")
content = content.gsub("Timeout: 10 * time.Second", "Timeout: 60 * time.Second")
File.write("pkg/backend/abs_episodes.go", content)
