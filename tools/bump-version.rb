#!/usr/bin/env ruby
# frozen_string_literal: true

require 'open3'

root_dir = File.expand_path('..', __dir__)
Dir.chdir(root_dir)

# Run quality gate silently
_, _, status = Open3.capture3("ruby tools/check.rb")
unless status.success?
  puts "✗ Quality Gate failed! Run 'make check' to see details."
  exit 1
end

# Read current version
current = File.read("VERSION").strip
major, minor, patch = current.split('.').map(&:to_i)
patch += 1
new_ver = "#{major}.#{minor}.#{patch}"
File.write("VERSION", new_ver + "\n")

# Git add, commit, push
Open3.capture3("git add VERSION")
Open3.capture3("git commit -m 'chore: bump version to #{new_ver}'")
Open3.capture3("git push")

puts "Success #{new_ver} (commit+push)"
