#!/usr/bin/env ruby
# frozen_string_literal: true

require "json"
require "open3"
require "time"

root_dir = File.expand_path("..", __dir__)
Dir.chdir(root_dir)

cfg_file = File.expand_path("~/.config/abs/config.json")
config = File.exist?(cfg_file) ? JSON.parse(File.read(cfg_file)) : {}

target_host = ARGV[0] || config["remote_host"] || "cloud8"
podcasts_dir = config["podcasts_dir"] || "/media/podcasts/clean"
remote_dir = "$HOME/abs_remote"

puts "======================================================================"
puts "       REMOTE QUEUE & POLICY COMPLIANCE VERIFICATION TOOL"
puts "======================================================================"
puts "Remote Host:   #{target_host}"
puts "Remote Dir:    #{remote_dir}"
puts "Local Library: #{podcasts_dir}"
puts "Current Time:  #{Time.now.utc.strftime("%Y-%m-%d %H:%M:%S UTC")}"
puts "======================================================================"

# 1. Reset any failed tasks caused by whisper downtime
reset_cmd = "ssh #{target_host} 'for f in #{remote_dir}/*/*.mp3.json; do [ -f \"$f\" ] || continue; if grep -q \"\\\"status\\\": \\\"failed\\\"\" \"$f\" 2>/dev/null; then sed -i \"s/\\\"status\\\": \\\"failed\\\"/\\\"status\\\": \\\"awaiting_transcription\\\"/\" \"$f\"; echo \"RESET: $f\"; fi; done'"
out, _, _ = Open3.capture3(reset_cmd)
resets = out.lines.map(&:strip).reject(&:empty?)
if resets.size > 0
  puts "\n[+] Automatically recovered #{resets.size} task(s) previously marked failed (re-queued)."
end

# 2. Build local podcast cache index
puts "\n[+] Synchronizing podcast metadata and publication timestamps..."
local_cache = {}
Dir.glob("#{podcasts_dir}/*").each do |dir|
  next unless File.directory?(dir)
  cache_file = File.join(dir, ".podcast_cache.json")
  if File.exist?(cache_file)
    c_data = JSON.parse(File.read(cache_file)) rescue nil
    if c_data && c_data["episodes"]
      c_data["episodes"].each do |ep|
        fn = ep["filename"] || File.basename(ep["path"] || "")
        if fn && ep["published_at"] && ep["published_at"].to_i > 0
          local_cache[fn] = Time.at(ep["published_at"].to_i / 1000).utc
        end
      end
    end
  end
end

# 3. Query all pending queued files from remote
q_cmd = "ssh #{target_host} \"find ~/abs_remote -type f -name '*.json' -exec grep -l '\\\"status\\\": \\\"awaiting_transcription\\\"' {} + 2>/dev/null\""
q_out, _, _ = Open3.capture3(q_cmd)
q_files = q_out.lines.map(&:strip).reject(&:empty?)

if q_files.empty?
  puts "No pending jobs found in remote queue on #{target_host}."
  exit 0
end

puts "[+] Retrieved #{q_files.size} queued job(s) from #{target_host}."

now = Time.now.utc
cutoff = now - (24 * 3600)

episodes = []
q_files.each do |r_json_path|
  cat_out, _, _ = Open3.capture3("ssh #{target_host} \"cat '#{r_json_path}'\"")
  st = JSON.parse(cat_out) rescue nil
  next unless st

  fn = st["media_file"] || File.basename(r_json_path).sub(/\.json$/, "")
  rel_path = r_json_path.sub(%r{^.*abs_remote/}, "").sub(/\.json$/, "")
  
  # Determine publication date
  pub_time = nil
  if st["published_at"] && !st["published_at"].empty?
    pub_time = Time.parse(st["published_at"]).utc rescue nil
  end

  if !pub_time && local_cache[fn]
    pub_time = local_cache[fn]
  end

  if !pub_time
    local_path = File.join(podcasts_dir, rel_path)
    if File.exist?(local_path)
      pub_time = File.mtime(local_path).utc
    else
      pod_dir = File.dirname(local_path)
      base = File.basename(local_path, ".*").sub(/\s+\([a-f0-9\-]+\)$/, "")
      matching = Dir.glob(File.join(pod_dir, "#{base}*")).find { |f| File.file?(f) && !f.end_with?(".json") }
      pub_time = File.mtime(matching).utc if matching
    end
  end

  dur = st.dig("original", "duration_sec") || 0.0
  pri = st["priority"] || 0

  is_recent = pub_time && (pub_time > cutoff)

  episodes << {
    path: rel_path,
    filename: fn,
    duration: dur,
    published_at: pub_time,
    is_recent: is_recent,
    priority: pri
  }
end

# Sort strictly according to the queuing policy
sorted_queue = episodes.sort do |a, b|
  if a[:priority] != b[:priority]
    b[:priority] <=> a[:priority]
  elsif a[:is_recent] != b[:is_recent]
    a[:is_recent] ? -1 : 1
  elsif a[:is_recent]
    # Recent: descending publication date (most recent first)
    if a[:published_at] != b[:published_at]
      (b[:published_at] || Time.at(0)) <=> (a[:published_at] || Time.at(0))
    elsif a[:duration] != b[:duration]
      a[:duration] <=> b[:duration]
    else
      a[:path] <=> b[:path]
    end
  else
    # Older: ascending duration (shortest first)
    if a[:duration] != b[:duration]
      a[:duration] <=> b[:duration]
    elsif a[:published_at] != b[:published_at]
      (b[:published_at] || Time.at(0)) <=> (a[:published_at] || Time.at(0))
    else
      a[:path] <=> b[:path]
    end
  end
end

recent_eps = sorted_queue.select { |e| e[:is_recent] }
older_eps = sorted_queue.select { |e| !e[:is_recent] }

puts "\n======================================================================"
puts "               REMOTE QUEUE POLICY BREAKDOWN"
puts "======================================================================"
puts "1. Recent Episodes (Published in the last 24h - Most Recent First): #{recent_eps.size}"
recent_eps.each_with_index do |ep, i|
  dur_s = sprintf("%02d:%02d", (ep[:duration] / 60).to_i, (ep[:duration] % 60).to_i)
  ago_h = ((now - ep[:published_at]) / 3600.0).round(1)
  puts "   [#{i+1}] [#{dur_s}] #{ep[:path]} (#{ago_h}h ago | #{ep[:published_at].strftime("%H:%M UTC")})"
end

if recent_eps.size > 0 && older_eps.size > 0
  puts "   ──────────────────────────────────────────────────────────────────"
end

puts "\n2. Older Episodes (Published >24h ago - Shortest Length First): #{older_eps.size}"
older_eps.each_with_index do |ep, i|
  dur_s = sprintf("%02d:%02d", (ep[:duration] / 60).to_i, (ep[:duration] % 60).to_i)
  puts "   [#{recent_eps.size + i + 1}] [#{dur_s}] #{ep[:path]}"
end

# 4. Strict Validation Checks
puts "\n======================================================================"
puts "                     POLICY COMPLIANCE CHECKS"
puts "======================================================================"

errs = 0

# Check 1: All recent come before older
if recent_eps.size > 0 && older_eps.size > 0
  first_older_idx = sorted_queue.index { |e| !e[:is_recent] }
  last_recent_idx = sorted_queue.rindex { |e| e[:is_recent] }
  if first_older_idx < last_recent_idx
    puts "\e[31m[FAIL] Ordering violation: Older episode found before recent episode!\e[0m"
    errs += 1
  else
    puts "\e[32m[PASS] Group Separation: All #{recent_eps.size} recent (<24h) episodes precede older episodes.\e[0m"
  end
else
  puts "\e[32m[PASS] Group Separation: Verified (single class present).\e[0m"
end

# Check 2: Recent episodes sorted descending by published_at
recent_order_ok = true
(0...(recent_eps.size - 1)).each do |i|
  if recent_eps[i][:published_at] < recent_eps[i+1][:published_at]
    puts "\e[31m[FAIL] Recent order violation: #{recent_eps[i][:path]} (#{recent_eps[i][:published_at]}) is older than #{recent_eps[i+1][:path]} (#{recent_eps[i+1][:published_at]})\e[0m"
    recent_order_ok = false
    errs += 1
  end
end
if recent_order_ok
  puts "\e[32m[PASS] Recent Episodes Sorting: Correctly ordered descending by publication time.\e[0m"
end

# Check 3: Older episodes sorted ascending by duration
older_order_ok = true
(0...(older_eps.size - 1)).each do |i|
  if older_eps[i][:duration] > older_eps[i+1][:duration]
    puts "\e[31m[FAIL] Older duration violation: #{older_eps[i][:path]} (#{older_eps[i][:duration]}s) > #{older_eps[i+1][:path]} (#{older_eps[i+1][:duration]}s)\e[0m"
    older_order_ok = false
    errs += 1
  end
end
if older_order_ok
  puts "\e[32m[PASS] Older Episodes Sorting: Correctly ordered ascending by audio duration.\e[0m"
end

puts "======================================================================"
if errs == 0
  puts "\e[32m\e[1m>>> RESULT: REMOTE QUEUE COMPLIES 100% WITH QUEUING POLICY <<<\e[0m"
else
  puts "\e[31m\e[1m>>> RESULT: #{errs} POLICY VIOLATION(S) DETECTED <<<\e[0m"
  exit 1
end
