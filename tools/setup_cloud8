#!/usr/bin/env ruby
# frozen_string_literal: true

require "open3"

PACKAGES = %w[
  ffmpeg
  libmp3lame0
  rsync
  curl
  jq
].freeze

def run_remote(cmd)
  puts "==> Running on Cloud8: #{cmd}"
  stdout, stderr, status = Open3.capture3("ssh", "-o", "BatchMode=yes", "cloud8", cmd)
  unless status.success?
    warn "Error executing command on Cloud8: #{stderr}"
    exit 1
  end
  puts stdout unless stdout.empty?
end

puts "--- Configuring Cloud8 for Remote Audio Processing ---"

run_remote("sudo apt-get update -y")
run_remote("DEBIAN_FRONTEND=noninteractive sudo apt-get install -y #{PACKAGES.join(" ")}")

puts "\n--- Verifying Installed Utilities on Cloud8 ---"
run_remote("ffmpeg -version | head -n 2")
run_remote("which ffprobe rsync jq curl")

puts "\n[✓] Cloud8 is fully provisioned and ready for remote audio processing!"
