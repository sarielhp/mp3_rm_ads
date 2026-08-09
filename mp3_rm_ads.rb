#!/usr/bin/env ruby
# frozen_string_literal: true

# mp3_rm_ads
# Automatically detects and removes ad/sponsor segments from a local MP3/audio podcast file
# using local AMD GPU Whisper transcription and configurable local/remote LLM ad-detection.
# Saves full timestamped transcript / SRT / JSON files before cutting.
#
# Config File Location: ~/.config/mp3_rm_ads/config.json

require 'optparse'
require 'net/http'
require 'uri'
require 'json'
require 'tempfile'
require 'open3'
require 'fileutils'
require 'socket'

$stdout.sync = true

WORK_DIR_NAME = '.work'

def work_dir_for(path)
  File.join(File.dirname(File.expand_path(path)), WORK_DIR_NAME)
end

def verify_temp_file(file_path)
  abs = File.expand_path(file_path)
  unless abs =~ /\/#{Regexp.escape(WORK_DIR_NAME)}\//
    puts "❌ ERROR: Temp file '#{file_path}' is not in .work/ directory. Aborting."
    exit 1
  end
end

begin
  require 'rainbow'
rescue LoadError
  module Rainbow
    def self.call(str)
      DummyString.new(str)
    end
    class DummyString < String
      def cyan; self; end
      def green; self; end
      def yellow; self; end
      def red; self; end
      def blue; self; end
      def magenta; self; end
      def bold; self; end
      def underline; self; end
      def bright; self; end
      def ljust(n, *args); super; end
    end
  end
  def Rainbow(str)
    Rainbow.call(str)
  end
end

CONFIG_DIR = File.expand_path('~/.config/mp3_rm_ads')
CONFIG_FILE = File.join(CONFIG_DIR, 'config.json')
OPENCODE_CONFIG_FILE = File.expand_path('~/.config/opencode/opencode.json')

SYSTEM_PROMPT = <<~PROMPT
  You are an expert podcast editor assistant.
  Your job is to analyze the timestamped transcript of a podcast episode and identify all advertisement segments, host-read sponsor plugs, promotional breaks, midroll/preroll ads, and sponsor call-outs.

  Return ONLY a raw JSON array of objects with the exact start and end seconds of each ad segment, like this:
  [
    {"start": 15.0, "end": 65.5, "reason": "Host read sponsor plug for VPN"},
    {"start": 1200.0, "end": 1290.0, "reason": "Midroll ad break"}
  ]

  If NO ads or sponsor plugs are found, return an empty JSON array: []
  Do not include markdown formatting or commentary outside the JSON array.
PROMPT

DEFAULT_CONFIG = {
  "_instructions" => "Configuration file for mp3_rm_ads. Select profiles by ID or set active_profile_id.",
  "whisper_url" => "http://192.168.1.230:8088/inference",
  "whisper_speed_factor" => 7.0,
  "whisper_docker_container" => "",
  "whisper_language" => "",
  "whisper_prompt" => "",
  "chunk_duration_sec" => 0,
  "parallel_chunks" => 1,
  "active_profile_id" => 1,
  "profiles" => [
    {
      "id" => 1,
      "name" => "Ollama Local (llama3.1:8b)",
      "type" => "ollama",
      "url" => "http://192.168.1.230:11434/v1/chat/completions",
      "model" => "llama3.1:8b",
      "api_key" => ""
    },
    {
      "id" => 2,
      "name" => "OpenRouter - Claude 3.5 Sonnet",
      "type" => "openrouter",
      "url" => "https://openrouter.ai/api/v1/chat/completions",
      "model" => "anthropic/claude-3.5-sonnet",
      "api_key" => ""
    },
    {
      "id" => 3,
      "name" => "OpenRouter - DeepSeek V4 Flash",
      "type" => "openrouter",
      "url" => "https://openrouter.ai/api/v1/chat/completions",
      "model" => "deepseek/deepseek-v4-flash",
      "api_key" => ""
    },
    {
      "id" => 4,
      "name" => "OpenRouter - Gemini 2.5 Flash",
      "type" => "openrouter",
      "url" => "https://openrouter.ai/api/v1/chat/completions",
      "model" => "google/gemini-2.5-flash",
      "api_key" => ""
    }
  ]
}.freeze

def local_ip
  Socket.ip_address_list.each do |addr|
    next unless addr.ipv4? && !addr.ipv4_loopback? && !addr.ipv4_multicast?
    ip = addr.ip_address
    return ip unless ip.start_with?('127.', '169.254.')
  end
  '127.0.0.1'
rescue StandardError
  '127.0.0.1'
end

def ensure_config_exists
  FileUtils.mkdir_p(CONFIG_DIR)
  unless File.exist?(CONFIG_FILE)
    config = DEFAULT_CONFIG.dup
    ip = local_ip
    config['whisper_url'] = "http://#{ip}:8088/inference"
    config['profiles'].each do |p|
      p['url'] = p['url'].sub('192.168.1.230', ip) if p['url'].include?('192.168.1.230')
    end
    File.write(CONFIG_FILE, JSON.pretty_generate(config) + "\n")
    puts "⚙️  Created default configuration file at: '#{CONFIG_FILE}'"
  end
end

def load_config
  ensure_config_exists
  JSON.parse(File.read(CONFIG_FILE))
rescue StandardError => e
  puts "⚠️ Warning: Failed to parse '#{CONFIG_FILE}' (#{e.message}). Using default configuration."
  DEFAULT_CONFIG.dup
end

def save_config(config)
  ensure_config_exists
  File.write(CONFIG_FILE, JSON.pretty_generate(config) + "\n")
end

$openrouter_models_cache = nil

def fetch_openrouter_models_api
  return $openrouter_models_cache if $openrouter_models_cache

  uri = URI.parse('https://openrouter.ai/api/v1/models')
  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true
  http.open_timeout = 5
  http.read_timeout = 5
  resp = http.get(uri.request_uri)

  if resp.code == '200'
    json = JSON.parse(resp.body)
    $openrouter_models_cache = json['data'] || []
  else
    $openrouter_models_cache = []
  end
rescue StandardError
  $openrouter_models_cache = []
end

def get_profile_cost(profile)
  type = profile['type'].to_s.downcase
  url = profile['url'].to_s.downcase
  model = profile['model'].to_s

  if type == 'ollama' || url.include?('11434') || url.include?('localhost') || url.include?('127.0.0.1')
    return {
      type: 'Local',
      cost_str: 'Free ($0.00 / Local GPU)',
      est_1h_str: '$0.00'
    }
  end

  if url.include?('openrouter.ai') || type == 'openrouter'
    api_models = fetch_openrouter_models_api

    clean = model.downcase.sub(%r{^[^/]+/}, '').sub(/^~/, '').sub(/:.*$/, '')
    tokens = clean.split(/[\/\._-]/).reject { |t| t.empty? }

    found = api_models.find do |m|
      m_id = m['id'].downcase.sub(/^~/, '').sub(/:.*$/, '')
      m_id == model.downcase || m_id == clean
    end

    unless found
      found = api_models.find do |m|
        m_id = m['id'].downcase.sub(/^~/, '').sub(/:.*$/, '')
        tokens.all? { |tok| m_id.include?(tok) }
      end
    end

    unless found
      key_terms = tokens.select { |t| %w[sonnet haiku opus flash deepseek llama gemini qwen mistral].include?(t) }
      found = api_models.find do |m|
        m_id = m['id'].downcase
        key_terms.any? && key_terms.all? { |kt| m_id.include?(kt) }
      end
    end

    if found && found['pricing']
      pr = found['pricing']
      prompt_price = pr['prompt'].to_f
      completion_price = pr['completion'].to_f

      in_1m = (prompt_price * 1_000_000).round(4)
      out_1m = (completion_price * 1_000_000).round(4)
      est_1h = (13000 * prompt_price + 300 * completion_price).round(4)

      return {
        type: "OpenRouter (#{found['id']})",
        in_1m: in_1m,
        out_1m: out_1m,
        cost_str: "In: $#{in_1m}/1M, Out: $#{out_1m}/1M",
        est_1h_str: "~$#{sprintf('%.4f', est_1h)} / 1-hr episode"
      }
    end
  end

  {
    type: 'Unknown',
    cost_str: 'Dynamic pricing unavailable (Check OpenRouter API)',
    est_1h_str: 'N/A'
  }
end

def list_profiles(config)
  active_id = config["active_profile_id"]
  puts "\n" + Rainbow("=" * 70).cyan
  puts Rainbow("🤖 AVAILABLE LLM PROFILES & PRICING IN CONFIG (#{CONFIG_FILE}):").cyan.bold
  puts Rainbow("=" * 70).cyan

  non_default_profiles = config["profiles"].reject { |p| p["id"] == active_id }
  default_profile = config["profiles"].find { |p| p["id"] == active_id }

  ordered_profiles = non_default_profiles.dup
  ordered_profiles << default_profile if default_profile

  ordered_profiles.each do |p|
    is_default = (p["id"] == active_id)
    default_badge = is_default ? Rainbow("  ★ [DEFAULT]").green.bold : ""
    has_key = p["api_key"].to_s.empty? ? "" : " (Key set)"
    cost_info = get_profile_cost(p)

    header_str = "  [#{p['id']}] #{p['name']}"
    formatted_header = is_default ? Rainbow(header_str).green.bold : header_str

    puts "#{formatted_header}#{default_badge}"
    puts "      • Model:     #{p['model']}"
    puts "      • Type:      #{p['type']}#{has_key}"
    puts "      • Pricing:   #{cost_info[:cost_str]}"
    puts "      • Est. 1-Hr: #{cost_info[:est_1h_str]}"
    puts "      • URL:       #{p['url']}"
    puts ""
  end
  puts Rainbow("=" * 70).cyan + "\n"
end

def copy_llm_from_opencode(config)
  unless File.exist?(OPENCODE_CONFIG_FILE)
    puts "❌ OpenCode configuration file not found at: '#{OPENCODE_CONFIG_FILE}'"
    return
  end

  begin
    opencode_raw = File.read(OPENCODE_CONFIG_FILE)
    opencode_json = JSON.parse(opencode_raw)
  rescue StandardError => e
    puts "❌ Error reading OpenCode config: #{e.message}"
    return
  end

  api_key = ENV['OPENROUTER_API_KEY'] || ENV['OPENAI_API_KEY'] || ''
  
  opencode_model = opencode_json['model'] || ''
  opencode_small_model = opencode_json['small_model'] || ''

  clean_model = opencode_model.sub(%r{^openrouter/}, '')
  clean_small_model = opencode_small_model.sub(%r{^openrouter/}, '')

  puts "🔄 Copying LLM settings from OpenCode ('#{OPENCODE_CONFIG_FILE}')..."
  puts "   • Imported Primary Model: '#{clean_model}'"
  puts "   • Imported Small Model:   '#{clean_small_model}'"
  puts "   • API Key detected:       #{api_key.empty? ? 'No key in ENV (Set OPENROUTER_API_KEY)' : 'Yes (sk-or-...)'}"

  updated = false
  next_id = (config['profiles'].map { |p| p['id'] }.max || 0) + 1

  [clean_model, clean_small_model].each do |mod|
    next if mod.empty?

    existing = config['profiles'].find { |p| p['model'] == mod || p['name'].include?(mod) }
    if existing
      existing['api_key'] = api_key unless api_key.empty?
      existing['url'] = 'https://openrouter.ai/api/v1/chat/completions'
      existing['type'] = 'openrouter'
      puts "   ✔ Updated existing profile [#{existing['id']}] for model '#{mod}'"
    else
      new_profile = {
        'id' => next_id,
        'name' => "OpenRouter - #{mod}",
        'type' => 'openrouter',
        'url' => 'https://openrouter.ai/api/v1/chat/completions',
        'model' => mod,
        'api_key' => api_key
      }
      config['profiles'] << new_profile
      puts "   ✔ Added new profile [#{next_id}] for model '#{mod}'"
      next_id += 1
    end
    updated = true
  end

  if updated
    target_profile = config['profiles'].find { |p| p['model'] == clean_model }
    config['active_profile_id'] = target_profile['id'] if target_profile

    save_config(config)
    puts "✅ Successfully imported OpenCode configuration! Active default profile is now [#{config['active_profile_id']}]."
  end
end

def display_usage_and_help
  prog = File.basename($PROGRAM_NAME)
  puts "\n" + Rainbow("=" * 75).cyan
  puts Rainbow("  🎧 mp3_rm_ads — Automatic Podcast Ad & Sponsor Segment Remover").cyan.bold
  puts Rainbow("=" * 75).cyan

  puts "\n" + Rainbow("USAGE:").yellow.bold
  puts "  #{Rainbow(prog).green.bold} #{Rainbow('<file1.mp3> [file2.mp3 ...] [options]').bright}"
  puts "  #{Rainbow(prog).green.bold} #{Rainbow('<transcript.json> [options]').bright}"

  puts "\n" + Rainbow("DESCRIPTION:").yellow.bold
  puts "  Automatically detects and removes advertisement, sponsor, and promotional"
  puts "  breaks from podcast MP3 files using local AMD GPU Whisper transcription"
  puts "  and configurable LLM detection. Preserves original files in `.mp3.precut`."

  puts "\n" + Rainbow("OPTIONS:").yellow.bold
  puts "  #{Rainbow('-o, --output PATH').green.bold.ljust(35)} Output MP3 path or directory"
  puts "  #{Rainbow('-q, --quiet').green.bold.ljust(35)} Suppress progress and informational output"
  puts "  #{Rainbow('-srt, --srt').green.bold.ljust(35)} Export/convert transcript JSON to SubRip (.srt)"
  puts "  #{Rainbow('-txt, --txt').green.bold.ljust(35)} Export/convert transcript JSON to text (.txt)"
  puts "  #{Rainbow('--recut').green.bold.ljust(35)} Recut audio using .cuts.json (no Whisper/LLM)"
  puts "  #{Rainbow('--no-transcript').green.bold.ljust(35)} Disable saving default .transcript.json file"
  puts "  #{Rainbow('--use-chunks').green.bold.ljust(35)} Split long audio into chunks for reliable transcription"
  puts "  #{Rainbow('--extract-keywords').green.bold.ljust(35)} Extract keywords via LLM to improve Whisper accuracy"
  puts "  #{Rainbow('-t, --transcribe-minutes Nm').green.bold.ljust(35)} Only transcribe first N minutes (e.g. -t 10m)"
  puts "  #{Rainbow('--force-llm').green.bold.ljust(35)} Force re-running LLM ad detection even if .cuts.json exists"
  puts "  #{Rainbow('--force-transcribe').green.bold.ljust(35)} Force re-transcribing audio even if .transcript.json exists"
  puts "  #{Rainbow('--use-llm ID_OR_NAME').green.bold.ljust(35)} Select active LLM profile by ID or name"
  puts "  #{Rainbow('--list-llms').green.bold.ljust(35)} List all configured LLM profiles and exit"
  puts "  #{Rainbow('--set-default ID').green.bold.ljust(35)} Set default LLM profile ID in config file"
  puts "  #{Rainbow('--copy_llm_from_opencode').green.bold.ljust(35)} Import LLM settings from OpenCode config"
  puts "  #{Rainbow('-h, --help').green.bold.ljust(35)} Show this detailed usage message"

  puts "\n" + Rainbow("EXAMPLES:").yellow.bold
  puts "  #{Rainbow('# 1. Process a single podcast MP3 file:').magenta}"
  puts "  #{prog} episode.mp3"
  puts ""
  puts "  #{Rainbow('# 2. Process multiple podcast files in batch sequentially:').magenta}"
  puts "  #{prog} episode1.mp3 episode2.mp3 episode3.mp3"
  puts ""
  puts "  #{Rainbow('# 3. Recut an MP3 using existing .cuts.json metadata without re-transcribing:').magenta}"
  puts "  #{prog} --recut episode.mp3"
  puts ""
  puts "  #{Rainbow('# 4. Process in quiet mode and export subtitle transcript (.srt):').magenta}"
  puts "  #{prog} -q --srt episode.mp3"
  puts ""
  puts "  #{Rainbow('# 5. Convert an existing transcript JSON to SRT & TXT without re-transcribing:').magenta}"
  puts "  #{prog} episode.transcript.json -srt -txt"
  puts ""
  puts "  #{Rainbow('# 6. Use a specific LLM profile by ID or model name:').magenta}"
  puts "  #{prog} --use-llm 2 episode.mp3"
  puts ""
  puts "  #{Rainbow('# 7. Import LLM settings and API key from OpenCode config:').magenta}"
  puts "  #{prog} --copy_llm_from_opencode"
  puts ""
  puts "  #{Rainbow('# 8. Skip reprocessing if output files already exist:').magenta}"
  puts "  #{prog} episode.mp3"
  puts "  #{prog} episode.mp3  # Will skip since .transcript.json and .cuts.json exist"
  puts ""
  puts "  #{Rainbow('# 9. Force re-transcribe and/or re-run LLM detection:').magenta}"
  puts "  #{prog} --force-transcribe episode.mp3  # Re-transcribe but skip LLM if .cuts.json exists"
  puts "  #{prog} --force-llm episode.mp3  # Re-run LLM but skip transcription if .transcript.json exists"
  puts "  #{prog} --force-transcribe --force-llm episode.mp3  # Force both transcription and LLM"

  puts "\n" + Rainbow("=" * 75).cyan + "\n"
end

def format_time(seconds)
  sec = seconds.to_f
  hrs = (sec / 3600).to_i
  mins = ((sec % 3600) / 60).to_i
  secs = (sec % 60).round(1)
  hrs > 0 ? sprintf('%02d:%02d:%04.1f', hrs, mins, secs) : sprintf('%02d:%04.1f', mins, secs)
end

def format_clock(seconds)
  sec = [seconds.to_f.round, 0].max
  hrs = sec / 3600
  mins = (sec % 3600) / 60
  secs = sec % 60
  hrs > 0 ? sprintf('%02d:%02d:%02d', hrs, mins, secs) : sprintf('%02d:%02d', mins, secs)
end

def format_srt_time(seconds)
  sec = seconds.to_f
  hrs = (sec / 3600).to_i
  mins = ((sec % 3600) / 60).to_i
  secs = (sec % 60).to_i
  millis = ((sec - sec.to_i) * 1000).round
  sprintf('%02d:%02d:%02d,%03d', hrs, mins, secs, millis)
end

WAV_SAMPLE_RATE = 16000
WAV_BYTES_PER_SEC = WAV_SAMPLE_RATE * 2

def build_wav_header(data_size)
  channels = 1
  bits_per_sample = 16
  byte_rate = WAV_SAMPLE_RATE * channels * bits_per_sample / 8
  block_align = channels * bits_per_sample / 8
  riff_size = 36 + data_size

  header = [0x52, 0x49, 0x46, 0x46].pack('C4')
  header += [riff_size].pack('V')
  header += [0x57, 0x41, 0x56, 0x45].pack('C4')
  header += [0x66, 0x6d, 0x74, 0x20].pack('C4')
  header += [16].pack('V')
  header += [1].pack('v')
  header += [channels].pack('v')
  header += [WAV_SAMPLE_RATE].pack('V')
  header += [byte_rate].pack('V')
  header += [block_align].pack('v')
  header += [bits_per_sample].pack('v')
  header += [0x64, 0x61, 0x74, 0x61].pack('C4')
  header += [data_size].pack('V')
  header.force_encoding('ASCII-8BIT')
end

def validate_wav_file(file_path)
  stdout, status = Open3.capture2(
    'ffprobe', '-v', 'error',
    '-show_entries', 'format=duration',
    '-of', 'default=noprint_wrappers=1:nokey=1',
    file_path
  )
  status.success? && stdout.to_f > 0
rescue StandardError
  false
end

def detect_script_language(text)
  return nil if text.nil? || text.strip.empty?

  hebrew_chars = text.scan(/\p{Hebrew}/).size
  arabic_chars = text.scan(/\p{Arabic}/).size
  cyrillic_chars = text.scan(/\p{Cyrillic}/).size
  greek_chars = text.scan(/\p{Greek}/).size
  total_letters = text.scan(/\p{L}/).size

  return nil if total_letters == 0

  ratios = {
    'he' => hebrew_chars.to_f / total_letters,
    'ar' => arabic_chars.to_f / total_letters,
    'ru' => cyrillic_chars.to_f / total_letters,
    'el' => greek_chars.to_f / total_letters
  }

  best = ratios.select { |_, r| r > 0.3 }.max_by { |_, r| r }
  best&.first
end

def get_audio_duration(file_path)
  abs_path = File.expand_path(file_path)
  stdout, status = Open3.capture2(
    'ffprobe', '-v', 'error',
    '-show_entries', 'format=duration',
    '-of', 'default=noprint_wrappers=1:nokey=1',
    abs_path
  )
  status.success? ? stdout.to_f : 0.0
rescue StandardError
  0.0
end

def extract_id3_tags(file_path)
  abs_path = File.expand_path(file_path)
  stdout, status = Open3.capture2(
    'ffprobe', '-v', 'error',
    '-show_entries', 'format_tags',
    '-of', 'default=noprint_wrappers=1',
    abs_path
  )
  return {} unless status.success?

  tags = {}
  stdout.each_line do |line|
    line = line.encode('UTF-8', invalid: :replace, undef: :replace)
    if line =~ /^TAG:(.+?)=(.+)$/
      key = $1.strip.downcase
      val = $2.strip
      tags[key] = val unless val.empty?
    end
  end
  tags
rescue StandardError
  {}
end

def validate_transcript_sanity(transcription_data, total_duration, quiet: false)
  return true if total_duration.to_f <= 0.0

  segments = transcription_data['segments'] || []
  full_text = transcription_data['text'] || segments.map { |s| s['text'] }.join(' ')
  word_count = full_text.scan(/\S+/).size

  min_expected_words = [(total_duration / 60.0 * 15.0).to_i, 20].max

  last_segment_end = segments.map { |s| s['end'].to_f }.max || 0.0
  min_required_coverage = total_duration * 0.85

  failed_reasons = []

  if word_count < min_expected_words
    failed_reasons << "Word count too low (#{word_count} words found, expected at least #{min_expected_words} words for #{format_clock(total_duration)} audio)"
  end

  if total_duration >= 30.0 && last_segment_end < min_required_coverage
    failed_reasons << "Transcript ended prematurely at #{format_clock(last_segment_end)} (expected coverage up to at least #{format_clock(min_required_coverage)})"
  end

  if failed_reasons.any?
    unless quiet
      puts "\n" + "⚠️ " * 35
      puts "⚠️ TRANSCRIPT SANITY CHECK FAILED!"
      puts "⚠️ " * 35
      failed_reasons.each do |reason|
        puts "  • #{reason}"
      end
      puts "  • The Whisper transcription appears incomplete or corrupted."
      puts "  • Aborting ad detection and audio cutting for safety."
      puts "⚠️ " * 35 + "\n"
    end
    return false
  end

  true
end

def fetch_docker_logs(container_name, tail: 50)
  return '' if container_name.to_s.empty?
  result = ''
  $docker_mutex.synchronize do
    stdout, stderr, status = Open3.capture3('docker', 'logs', '--tail', tail.to_s, container_name)
    if status.success?
      result = (stderr && !stderr.empty? ? stderr : stdout) || ''
      result = result.encode('UTF-8', invalid: :replace, undef: :replace)
    end
  end
  result
rescue StandardError
  ''
end

def poll_whisper_docker_progress(container_name)
  return nil if container_name.to_s.empty?

  log_text = fetch_docker_logs(container_name, tail: 20)
  return nil if log_text.empty?

  has_failed = false
  log_text.each_line do |line|
    if line =~ /failed to (decode|encode)/i
      has_failed = true
    elsif line =~ /processing\s+audio\s*\((\d+):(\d+):(\d+)\.\d+\s*->\s*(\d+):(\d+):(\d+)\.\d+\)/
      h, m, s = $4.to_i, $5.to_i, $6.to_f
      current_end = h * 3600 + m * 60 + s
      return current_end
    elsif line =~ /processing\s+audio\s*\((\d+):(\d+)\.\d+\s*->\s*(\d+):(\d+)\.\d+\)/
      m, s = $3.to_i, $4.to_f
      current_end = m * 60 + s
      return current_end
    elsif line =~ /progress_common:\s*([\d.]+)%/
      pct = $1.to_f
      return pct / 100.0
    end
  end

  return :failed_to_decode if has_failed
  nil
rescue StandardError
  nil
end

def report_whisper_error(container_name)
  log_text = fetch_docker_logs(container_name, tail: 50)
  return if log_text.empty?

  lines = log_text.lines.map(&:chomp)
  err_idx = lines.index { |l| l =~ /failed to (decode|encode)/i }
  return if err_idx.nil?

  start_idx = [err_idx - 5, 0].max
  end_idx = [err_idx + 5, lines.size - 1].min

  puts "\n🐳 Whisper Docker logs (error context):"
  (start_idx..end_idx).each do |i|
    prefix = i == err_idx ? '>>>' : '   '
    puts "  #{prefix} #{lines[i]}"
  end
end

$docker_mutex = Mutex.new

def detect_whisper_docker_container(whisper_url)
  uri = URI.parse(whisper_url)
  host = uri.host

  is_local = case host
  when 'localhost', '127.0.0.1', '0.0.0.0', '::1'
    true
  else
    local_ips = Socket.ip_address_list.select { |a| a.ipv4? && !a.ipv4_loopback? && !a.ipv4_multicast? }.map { |a| a.ip_address }
    local_ips << '127.0.0.1'
    local_ips.include?(host)
  end

  return nil unless is_local

  containers = []
  $docker_mutex.synchronize do
    stdout, status = Open3.capture2('docker', 'ps', '--format', '{{.ID}}\t{{.Image}}\t{{.Names}}')
    return nil unless status.success?

    stdout.each_line do |line|
      id, image, name = line.strip.split("\t", 3)
      containers << { id: id, image: image, name: name } if image
    end
  end

  whisper_keywords = %w[whisper whisper.cpp whisper-cpp whispercpp]

  containers.each do |c|
    image_lower = c[:image].downcase
    if whisper_keywords.any? { |kw| image_lower.include?(kw) }
      return c[:name]
    end
  end

  proxy_keywords = %w[traefik nginx caddy haproxy envoy]

  containers.each do |c|
    image_lower = c[:image].downcase
    next if proxy_keywords.any? { |kw| image_lower.include?(kw) }

    ports_stdout = nil
    $docker_mutex.synchronize do
      ports_stdout, status = Open3.capture2('docker', 'port', c[:id])
      ports_stdout = nil unless status.success?
    end
    next unless ports_stdout

    if ports_stdout.include?(uri.port.to_s)
      return c[:name]
    end
  end

  nil
rescue StandardError
  nil
end

def transcribe_whisper(audio_path, whisper_url, quiet: false, total_duration: 0.0, speed_factor: 7.0, docker_container: nil, prompt: nil, language: nil, pcm_data: nil)
  $decode_failed = false
  uri = URI.parse(whisper_url)
  boundary = "----RubyWhisperBoundary#{Time.now.to_i}#{rand(1000)}"

  body = []
  filename = File.basename(audio_path)

  if pcm_data
    audio_content = build_wav_header(pcm_data.bytesize) + pcm_data
    content_type = 'audio/wav'
  else
    audio_content = File.binread(audio_path)
    ext = File.extname(audio_path).downcase
    content_type = case ext
                   when '.wav' then 'audio/wav'
                   when '.mp3' then 'audio/mpeg'
                   else 'audio/mpeg'
                   end
  end

  body << "--#{boundary}\r\n"
  body << "Content-Disposition: form-data; name=\"file\"; filename=\"#{filename}\"\r\n"
  body << "Content-Type: #{content_type}\r\n\r\n"
  body << audio_content
  body << "\r\n"

  body << "--#{boundary}\r\n"
  body << "Content-Disposition: form-data; name=\"response_format\"\r\n\r\n"
  body << "verbose_json\r\n"

  body << "--#{boundary}\r\n"
  body << "Content-Disposition: form-data; name=\"temperature\"\r\n\r\n"
  body << "0.0\r\n"

  body << "--#{boundary}\r\n"
  body << "Content-Disposition: form-data; name=\"language\"\r\n\r\n"
  body << "#{language}\r\n"

  if prompt && !prompt.strip.empty?
    body << "--#{boundary}\r\n"
    body << "Content-Disposition: form-data; name=\"prompt\"\r\n\r\n"
    body << "#{prompt.strip}\r\n"
  end

  body << "--#{boundary}--\r\n"

  payload_body = body.map { |s| s.dup.force_encoding('ASCII-8BIT') }.join

  max_retries = 5
  retry_delay = 5

  est_transcribe_seconds = (total_duration.to_f > 0 && speed_factor.to_f > 0) ? (total_duration.to_f / speed_factor.to_f) : 900.0
  read_timeout = [(est_transcribe_seconds * 1.5).to_i, 600].max

  (1..max_retries).each do |attempt|
    progress_thread = nil
    start_time = Time.now

    begin
      http = Net::HTTP.new(uri.host, uri.port)
      http.use_ssl = (uri.scheme == 'https')
      http.read_timeout = read_timeout
      http.open_timeout = 10

      req = Net::HTTP::Post.new(uri.request_uri)
      req.content_type = "multipart/form-data; boundary=#{boundary}"
      req.body = payload_body

      unless quiet
        est_total_seconds = (total_duration.to_f > 0 && speed_factor.to_f > 0) ? (total_duration.to_f / speed_factor.to_f) : 0.0
        $decode_failed = false
        progress_thread = Thread.new do
          loop do
            elapsed = Time.now - start_time

            docker_progress = poll_whisper_docker_progress(docker_container)
            if docker_progress == :failed_to_decode
              $decode_failed = true
              print "\r⚠️ Whisper failed to decode audio (file too long or corrupted).   \n"
              STDOUT.flush
              Thread.main.raise("Whisper failed to decode audio (file too long or corrupted)")
              break
            elsif docker_progress.is_a?(Float) && docker_progress <= 1.0
              pct = docker_progress
              remaining = pct > 0 ? [(elapsed / pct) - elapsed, 0].max : est_total_seconds - elapsed
              rem_str = remaining > 0 ? format_clock(remaining) : "00:00 (finishing...)"
              print "\r⏳ Transcribing audio... #{sprintf('%5.1f', pct * 100)}% | Elapsed: #{format_clock(elapsed)} | Est. remaining: #{rem_str}   "
            elsif docker_progress.is_a?(Numeric) && docker_progress > 0 && total_duration > 0
              pct = docker_progress / total_duration
              pct = [pct, 0.99].min
              remaining = [(elapsed / pct) - elapsed, 0].max
              rem_str = remaining > 0 ? format_clock(remaining) : "00:00 (finishing...)"
              print "\r⏳ Transcribing audio... #{sprintf('%5.1f', pct * 100)}% | Elapsed: #{format_clock(elapsed)} | Est. remaining: #{rem_str}   "
            elsif est_total_seconds > 0
              remaining = [est_total_seconds - elapsed, 0].max
              rem_str = remaining > 0 ? format_clock(remaining) : "00:00 (finishing...)"
              print "\r⏳ Transcribing audio... Elapsed: #{format_clock(elapsed)} | Est. remaining: #{rem_str}   "
            else
              print "\r⏳ Transcribing audio... Elapsed: #{format_clock(elapsed)}   "
            end
            STDOUT.flush
            sleep 2
          end
        end
      end

      resp = http.request(req)

      if progress_thread
        progress_thread.kill
        progress_thread.join rescue nil
        elapsed = Time.now - start_time
        print "\r⏳ Transcription finished in #{format_clock(elapsed)}!#{' ' * 30}\n" unless quiet
      end

      if $decode_failed
        raise "Whisper failed to decode audio (file too long or corrupted)"
      end

      if resp.code == '200'
        return JSON.parse(resp.body)
      else
        puts "\n⚠️ Whisper server returned status #{resp.code}: #{resp.body}" unless quiet
        if attempt < max_retries
          puts "   Retrying in #{retry_delay} seconds (attempt #{attempt}/#{max_retries})..." unless quiet
          sleep retry_delay
        end
      end

    rescue StandardError => e
      if progress_thread
        progress_thread.kill
        progress_thread.join rescue nil
        print "\r#{' ' * 75}\r" unless quiet
      end

      if $decode_failed || e.message.include?('failed to')
        raise "failed to process audio"
      end

      if attempt < max_retries
        unless quiet
          puts "⚠️ Whisper server connection error (attempt #{attempt}/#{max_retries}): #{e.message}"
          puts "   Whisper Docker container may be sleeping or waking up. Retrying in #{retry_delay} seconds..."
        end
        sleep retry_delay
      else
        puts "❌ Error: Failed to connect to Whisper GPU server at '#{whisper_url}' after #{max_retries} attempts (#{e.message})."
        puts "   Whisper Docker container is not working properly. Please check that the Whisper Docker container is running and healthy."
        exit 1
      end
    end
  end

  {}
end

def transcribe_chunks(audio_path, whisper_url, quiet: false, total_duration: 0.0, speed_factor: 7.0, chunk_duration: 600, parallel: 1, docker_container: nil, prompt: nil, language: nil)
  overlap = 30
  max_chunk = [chunk_duration.to_f, 1200.0].min
  num_chunks = [(total_duration / max_chunk).ceil, 1].max

  unless quiet
    puts "   Converting to WAV and splitting #{format_time(total_duration)} audio into #{num_chunks} chunks of #{format_time(max_chunk)}..."
  end

  work_dir = work_dir_for(audio_path)
  FileUtils.mkdir_p(work_dir)
  wav_path = File.join(work_dir, "#{File.basename(audio_path)}.wav")
  verify_temp_file(wav_path)

  system('ffmpeg', '-y', '-loglevel', 'error',
         '-i', audio_path,
         '-ar', WAV_SAMPLE_RATE.to_s, '-ac', '1', '-c:a', 'pcm_s16le', wav_path)

  wav_size = File.size(wav_path)
  pcm_size = wav_size - 44
  actual_dur = pcm_size.to_f / WAV_BYTES_PER_SEC

  if num_chunks <= 1
    pcm_data = File.binread(wav_path, pcm_size, 44)
    File.unlink(wav_path) rescue nil
    FileUtils.rm_rf(work_dir) if Dir.exist?(work_dir)
    return transcribe_whisper(audio_path, whisper_url, quiet: quiet, total_duration: total_duration, speed_factor: speed_factor, docker_container: docker_container, prompt: prompt, language: language, pcm_data: pcm_data)
  end

  chunks = []
  num_chunks.times do |i|
    actual_start = i * max_chunk
    actual_end = [(i + 1) * max_chunk, total_duration].min
    extract_start = [actual_start - overlap, 0].max
    extract_end = [actual_end + overlap, total_duration].min
    start_byte = (extract_start * WAV_BYTES_PER_SEC).to_i
    data_size = ((extract_end - extract_start) * WAV_BYTES_PER_SEC).to_i
    chunks << {
      index: i,
      actual_start: actual_start,
      actual_end: actual_end,
      extract_start: extract_start,
      extract_end: extract_end,
      start_byte: start_byte,
      data_size: data_size
    }
  end

  all_segments = []
  mutex = Mutex.new

  workers = [parallel, chunks.size].min

  chunks.each_slice(workers) do |batch|
    batch_results = []
    batch_threads = []

    batch.each do |chunk|
      batch_threads << Thread.new do
        idx = chunk[:index]
        unless quiet
          mutex.synchronize do
            chunk_len = chunk[:actual_end] - chunk[:actual_start]
            puts "\nWorking on chunk #{idx + 1}/#{chunks.size}: #{format_time(chunk[:actual_start])} -> #{format_time(chunk[:actual_end])} (#{format_time(chunk_len)})"
          end
        end

        chunk_path = File.join(work_dir, "chunk_#{format('%04d', idx)}.wav")
        File.open(wav_path, 'rb') do |src|
          src.seek(44 + chunk[:start_byte])
          pcm_data = src.read(chunk[:data_size])
          File.binwrite(chunk_path, build_wav_header(pcm_data.bytesize) + pcm_data)
        end

        unless validate_wav_file(chunk_path)
          mutex.synchronize do
            puts "❌ Chunk #{idx + 1} failed WAV validation — corrupt or invalid."
          end
          Thread.current.raise("Chunk #{idx + 1} failed WAV validation")
        end

        chunk_data = transcribe_whisper(
          chunk_path, whisper_url,
          quiet: quiet,
          total_duration: chunk[:actual_end] - chunk[:actual_start],
          speed_factor: speed_factor,
          docker_container: docker_container,
          prompt: prompt,
          language: language
        )

        File.unlink(chunk_path) rescue nil

        mutex.synchronize do
          batch_results << { data: chunk_data, chunk: chunk }
        end
      end
    end

    batch_threads.each(&:join)

    batch_results.each do |result|
      chunk_data = result[:data]
      chunk = result[:chunk]
      next unless chunk_data && chunk_data['segments']

      is_first = chunk[:index] == 0
      is_last = chunk[:index] == chunks.size - 1
      cut_start = chunk[:actual_start]
      cut_end = chunk[:actual_end]

      chunk_data['segments'].each do |seg|
        seg_start = seg['start'].to_f + chunk[:extract_start]
        seg_end = seg['end'].to_f + chunk[:extract_start]

        if is_first
          next if seg_end <= cut_start
          seg['start'] = [seg_start, cut_start].max
          seg['end'] = [seg_end, cut_end].min
        elsif is_last
          next if seg_start >= cut_end
          seg['start'] = [seg_start, cut_start].max
          seg['end'] = [seg_end, cut_end].min
        else
          mid = cut_end
          next if seg_start >= mid
          seg['start'] = [seg_start, cut_start].max
          seg['end'] = [seg_end, mid].min
        end

        if seg['words']
          seg['words'].each do |w|
            w['start'] = (w['start'].to_f + chunk[:extract_start]) rescue w['start']
            w['end'] = (w['end'].to_f + chunk[:extract_start]) rescue w['end']
          end
        end

        all_segments << seg
      end
    end
  end

  File.unlink(wav_path) rescue nil
  FileUtils.rm_rf(work_dir) if Dir.exist?(work_dir)

  all_segments.sort_by! { |s| s['start'].to_f }

  merged_segments = []
  all_segments.each do |seg|
    if merged_segments.empty?
      merged_segments << seg
    else
      last = merged_segments.last
      if seg['start'].to_f <= last['end'].to_f + 0.5
        if seg['end'].to_f > last['end'].to_f
          last['end'] = seg['end']
          last['text'] = seg['text']
        end
      else
        merged_segments << seg
      end
    end
  end

  full_text = merged_segments.map { |s| s['text'] }.join(' ')

  {
    'text' => full_text,
    'segments' => merged_segments,
    'language' => (all_segments.first && all_segments.first['language']) || 'he'
  }
end

def save_json_transcript(input_file, transcription_data, custom_path = nil, quiet: false, id3_tags: nil)
  ext = File.extname(input_file)
  base = input_file.sub(/\.transcript\.json$/, '').chomp(ext)
  json_file = custom_path || "#{base}.transcript.json"
  output_data = transcription_data.dup
  if id3_tags && id3_tags.any?
    id3_tags.each { |k, v| output_data["id3_#{k}"] = v }
  end
  File.write(json_file, JSON.pretty_generate(output_data) + "\n")
  puts "📄 Saved raw Whisper JSON data (.json) to: '#{json_file}'" unless quiet
  json_file
end

def convert_json_to_srt(input_file_or_path, transcription_data = nil, custom_path: nil, quiet: false)
  if transcription_data.nil?
    unless File.exist?(input_file_or_path)
      puts "❌ Error: Cannot convert to SRT, JSON file not found: '#{input_file_or_path}'"
      return nil
    end
    transcription_data = JSON.parse(File.read(input_file_or_path))
  end

  segments = transcription_data['segments'] || []
  ext = File.extname(input_file_or_path)
  base = input_file_or_path.sub(/\.transcript\.json$/, '').chomp(ext)
  srt_file = custom_path && custom_path.end_with?('.srt') ? custom_path : "#{base}.srt"

  srt_lines = []
  segments.each_with_index do |seg, idx|
    st = format_srt_time(seg['start'] || 0.0)
    en = format_srt_time(seg['end'] || 0.0)
    text = (seg['text'] || '').strip
    srt_lines << (idx + 1).to_s
    srt_lines << "#{st} --> #{en}"
    srt_lines << text
    srt_lines << ""
  end

  File.write(srt_file, srt_lines.join("\n") + "\n")
  puts "📄 Converted and saved SubRip Subtitle file (.srt) to: '#{srt_file}'" unless quiet
  srt_file
end

def convert_json_to_txt(input_file_or_path, transcription_data = nil, total_duration: 0.0, custom_path: nil, quiet: false)
  if transcription_data.nil?
    unless File.exist?(input_file_or_path)
      puts "❌ Error: Cannot convert to TXT, JSON file not found: '#{input_file_or_path}'"
      return nil
    end
    transcription_data = JSON.parse(File.read(input_file_or_path))
  end

  segments = transcription_data['segments'] || []
  lang = transcription_data['language'] || 'auto'
  ext = File.extname(input_file_or_path)
  base = input_file_or_path.sub(/\.transcript\.json$/, '').chomp(ext)
  txt_file = custom_path && custom_path.end_with?('.txt') ? custom_path : "#{base}.transcript.txt"

  lines = []
  lines << "=" * 80
  lines << "PODCAST TRANSCRIPTION: #{File.basename(base)}"
  lines << "Original Duration: #{format_time(total_duration)} (#{total_duration.round(1)}s) | Language: #{lang.upcase}"
  lines << "=" * 80
  lines << ""

  if segments.empty? && transcription_data['text']
    lines << "[00:00.0 -> #{format_time(total_duration)}] #{transcription_data['text']}"
  else
    segments.each do |seg|
      st = seg['start'] || 0.0
      en = seg['end'] || 0.0
      text = (seg['text'] || '').strip
      lines << "[#{format_time(st)} -> #{format_time(en)}] #{text}"

      words = seg['words'] || []
      if words.any?
        word_str = words.map { |w| "#{w['word']}(#{w['start'].to_f.round(2)}-#{w['end'].to_f.round(2)})" }.join(" ")
        lines << "   └─ Words: #{word_str}"
      end
    end
  end

  File.write(txt_file, lines.join("\n") + "\n")
  puts "📄 Converted and saved timestamped text transcript (.txt) to: '#{txt_file}'" unless quiet
  txt_file
end

def save_cuts_json(main_mp3_file, total_duration, ad_segments, selected_profile = nil, custom_path = nil, quiet: false)
  ext = File.extname(main_mp3_file)
  base = main_mp3_file.sub(/\.cuts\.json$/, '').sub(/\.transcript\.json$/, '').chomp(ext)
  cuts_file = custom_path || "#{base}.cuts.json"

  existing_cuts_data = nil
  existing_merged_intervals = []
  existing_raw_intervals = []

  if File.exist?(cuts_file)
    begin
      existing_cuts_data = JSON.parse(File.read(cuts_file))
      existing_merged_intervals = (existing_cuts_data['merged_cut_intervals'] || []).map do |m|
        { "start" => m['start'].to_f.round(2), "end" => m['end'].to_f.round(2) }
      end
      existing_raw_intervals = existing_cuts_data['cut_intervals'] || []
    rescue StandardError => e
      puts "⚠️ Warning: Could not parse existing cuts file '#{cuts_file}': #{e.message}" unless quiet
    end
  end

  combined_raw_intervals = []

  existing_raw_intervals.each do |ad|
    st = (ad['start_sec'] || ad['start']).to_f
    en = (ad['end_sec'] || ad['end']).to_f
    combined_raw_intervals << {
      'start' => st,
      'end' => en,
      'reason' => ad['reason']
    }
  end

  ad_segments.each do |ad|
    st = ad['start'].to_f
    en = ad['end'].to_f
    combined_raw_intervals << {
      'start' => st,
      'end' => en,
      'reason' => ad['reason']
    }
  end

  formatted_raw_cut_intervals = combined_raw_intervals.map do |ad|
    st = ad['start'].to_f
    en = ad['end'].to_f
    dur = (en - st).round(2)
    item = {
      "start_sec" => st,
      "end_sec" => en,
      "duration_sec" => dur,
      "start_formatted" => format_clock(st),
      "end_formatted" => format_clock(en)
    }
    item["reason"] = ad['reason'] if ad['reason'] && !ad['reason'].to_s.strip.empty?
    item
  end

  all_bounds = combined_raw_intervals.map { |a| [a['start'].to_f, a['end'].to_f] }.sort_by { |a| [a[0], a[1]] }
  new_merged_intervals = []
  all_bounds.each do |(st, en)|
    if new_merged_intervals.empty?
      new_merged_intervals << [st, en]
    else
      last = new_merged_intervals.last
      last_st = last[0]
      last_en = last[1]

      dur1 = last_en - last_st
      dur2 = en - st
      min_dur = [dur1, dur2].min

      allowed_gap = if min_dur >= 30.0
                      5.0
                    elsif min_dur >= 20.0
                      4.0
                    elsif min_dur >= 10.0
                      3.0
                    else
                      0.0
                    end

      if st <= (last_en + allowed_gap)
        last[1] = [last_en, en].max
      else
        new_merged_intervals << [st, en]
      end
    end
  end

  formatted_merged_cut_intervals = new_merged_intervals.map do |(st, en)|
    { "start" => st.round(2), "end" => en.round(2) }
  end

  keep_intervals = []
  curr_start = 0.0
  formatted_merged_cut_intervals.each do |mc|
    c_st = mc["start"]
    c_en = mc["end"]
    keep_intervals << { "start" => curr_start, "end" => c_st } if c_st > curr_start
    curr_start = [curr_start, c_en].max
  end
  keep_intervals << { "start" => curr_start, "end" => total_duration.round(2) } if curr_start < total_duration.round(2)

  if existing_cuts_data && existing_merged_intervals == formatted_merged_cut_intervals
    puts Rainbow("⚠️  No new ad cuts were discovered (cut set remains unchanged).").yellow.bold unless quiet
    return {
      cuts_file: cuts_file,
      keep_segments: keep_intervals.map { |k| [k["start"], k["end"]] },
      changed: false
    }
  end

  total_cut_sec = formatted_merged_cut_intervals.sum { |mc| mc["end"] - mc["start"] }.round(2)
  generator_name = File.basename($PROGRAM_NAME)
  llm_info = selected_profile ? "#{selected_profile['name']} (#{selected_profile['model']})" : "Unknown"

  cuts_data = {
    "version" => 1,
    "generator" => generator_name,
    "llm_used" => llm_info,
    "target_file" => File.basename(main_mp3_file),
    "original_duration_sec" => total_duration.round(2),
    "total_cut_duration_sec" => total_cut_sec,
    "cut_intervals" => formatted_raw_cut_intervals,
    "merged_cut_intervals" => formatted_merged_cut_intervals,
    "keep_intervals" => keep_intervals
  }

  File.write(cuts_file, JSON.pretty_generate(cuts_data) + "\n")
  puts "📄 Saved updated cut metadata (.json) to: '#{cuts_file}'" unless quiet

  {
    cuts_file: cuts_file,
    keep_segments: keep_intervals.map { |k| [k["start"], k["end"]] },
    changed: true
  }
end

def detect_ads_llm(transcript_text, profile)
  uri = URI.parse(profile['url'])
  payload = {
    model: profile['model'],
    messages: [
      { role: 'system', content: SYSTEM_PROMPT },
      { role: 'user', content: "Here is the podcast transcript with timestamps in seconds:\n\n#{transcript_text}" }
    ],
    temperature: 0.1
  }

  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = (uri.scheme == 'https')
  http.read_timeout = 300
  http.open_timeout = 10

  req = Net::HTTP::Post.new(uri.request_uri)
  req.content_type = 'application/json'

  api_key = profile['api_key'].to_s.empty? ? ENV['OPENROUTER_API_KEY'] : profile['api_key']
  req['Authorization'] = "Bearer #{api_key}" unless api_key.to_s.empty?

  req.body = JSON.generate(payload)

  resp = http.request(req)
  unless resp.code == '200'
    puts "❌ Server returned status code #{resp.code}: #{resp.body}"
    return []
  end

  res_json = JSON.parse(resp.body)
  content = res_json.dig('choices', 0, 'message', 'content') || '[]'

  match = content.match(/\[\s*\{.*\}\s*\]/m)
  content = match[0] if match

  JSON.parse(content)
rescue StandardError => e
  puts "⚠️ Warning during LLM ad detection: #{e.message}"
  []
end

KEYWORD_EXTRACTION_PROMPT = <<~PROMPT
  You are a transcription assistant. Your job is to extract key topics, names, technical terms,
  brand names, and unusual words from a podcast transcript segment.

  Return ONLY a comma-separated list of 10-20 keywords/phrases (each 1-3 words).
  Focus on: guest names, topic-specific jargon, product names, locations, and any words
  that are unusual or容易被误听 (easily misheard).

  Keep each keyword short. Do not include markdown or commentary.
PROMPT

def extract_keywords_llm(transcript_text, profile, quiet: false)
  uri = URI.parse(profile['url'])
  payload = {
    model: profile['model'],
    messages: [
      { role: 'system', content: KEYWORD_EXTRACTION_PROMPT },
      { role: 'user', content: "Extract keywords from this podcast transcript segment:\n\n#{transcript_text}" }
    ],
    temperature: 0.1,
    max_tokens: 200
  }

  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = (uri.scheme == 'https')
  http.read_timeout = 60
  http.open_timeout = 10

  req = Net::HTTP::Post.new(uri.request_uri)
  req.content_type = 'application/json'

  api_key = profile['api_key'].to_s.empty? ? ENV['OPENROUTER_API_KEY'] : profile['api_key']
  req['Authorization'] = "Bearer #{api_key}" unless api_key.to_s.empty?

  req.body = JSON.generate(payload)

  resp = http.request(req)
  unless resp.code == '200'
    puts "⚠️ Keyword extraction LLM returned status #{resp.code}: #{resp.body}" unless quiet
    return ''
  end

  res_json = JSON.parse(resp.body)
  content = res_json.dig('choices', 0, 'message', 'content') || ''

  keywords = content.gsub(/[\[\]"]/, '').split(',').map(&:strip).reject(&:empty?)
  keywords.first(30).join(', ')
rescue StandardError => e
  puts "⚠️ Warning during keyword extraction: #{e.message}" unless quiet
  ''
end

def merge_intervals(intervals, quiet: false)
  return [] if intervals.empty?

  sorted = intervals.sort_by { |ad| ad['start'].to_f }
  merged = [sorted.first.dup]

  sorted[1..-1].each do |ad|
    last = merged.last
    last_st = last['start'].to_f
    last_en = last['end'].to_f
    ad_st = ad['start'].to_f
    ad_en = ad['end'].to_f

    dur1 = last_en - last_st
    dur2 = ad_en - ad_st
    min_dur = [dur1, dur2].min

    allowed_gap = if min_dur >= 30.0
                    5.0
                  elsif min_dur >= 20.0
                    4.0
                  elsif min_dur >= 10.0
                    3.0
                  else
                    0.0
                  end

    if ad_st <= (last_en + allowed_gap)
      last['end'] = [last_en, ad_en].max
    else
      merged << ad.dup
    end
  end

  merged
end

def calculate_keep_segments(total_duration, ad_segments)
  sorted_ads = ad_segments.sort_by { |ad| ad['start'].to_f }
  keep_segments = []
  current_start = 0.0

  sorted_ads.each do |ad|
    ad_start = ad['start'].to_f
    ad_end = ad['end'].to_f

    keep_segments << [current_start, ad_start] if ad_start > current_start
    current_start = [current_start, ad_end].max
  end

  keep_segments << [current_start, total_duration] if current_start < total_duration
  keep_segments
end

def cut_audio_ffmpeg(input_file, keep_segments, output_file)
  return false if keep_segments.empty?

  abs_input = File.expand_path(input_file)
  abs_output = File.expand_path(output_file)

  filter_parts = []
  concat_inputs = []

  keep_segments.each_with_index do |(st, en), idx|
    filter_parts << "[0:a]atrim=start=#{sprintf('%.3f', st)}:end=#{sprintf('%.3f', en)},asetpts=PTS-STARTPTS[a#{idx}];"
    concat_inputs << "[a#{idx}]"
  end

  filter_complex = filter_parts.join + concat_inputs.join + "concat=n=#{keep_segments.size}:v=0:a=1[aout]"

  cmd = [
    'ffmpeg', '-y', '-loglevel', 'error',
    '-i', abs_input,
    '-filter_complex', filter_complex,
    '-map', '[aout]',
    '-c:a', 'libmp3lame',
    '-b:a', '192k',
    abs_output
  ]

  system(*cmd)
end

def safe_move(src, dst)
  FileUtils.rm_f(dst) if File.symlink?(dst)
  FileUtils.mv(src, dst)
end

def check_precut_symlink(precut_file)
  if File.symlink?(precut_file)
    puts "❌ ERROR: Pre-cut backup file '#{precut_file}' is a symlink. Refusing to overwrite."
    exit 1
  end
end

config = load_config

ARGV.map! do |arg|
  case arg
  when '-srt', '-SRT' then '--srt'
  when '-txt', '-TXT' then '--txt'
  when '-recut', '-RECUT' then '--recut'
  else arg
  end
end

cli_options = {
  output: nil,
  transcript_path: nil,
  save_transcript: true,
  export_srt: false,
  export_txt: false,
  recut: false,
  force_llm: false,
  force_transcribe: false,
  use_llm: nil,
  set_default: nil,
  list_llms: false,
  copy_opencode: false,
  quiet: false,
  use_chunks: false,
  transcribe_minutes: nil,
  extract_keywords: false
}

OptionParser.new do |opts|
  opts.banner = "Usage: #{$PROGRAM_NAME} <path_to_podcast.mp3|transcript.json> [options]"

  opts.on('-o', '--output PATH', String, 'Output MP3 file path (default: <input>_adfree.mp3)') do |o|
    cli_options[:output] = o
  end

  opts.on('-t', '--transcript-file PATH', String, 'Custom transcript output file path') do |t|
    cli_options[:transcript_path] = t
  end

  opts.on('--srt', 'Convert transcript JSON to SubRip subtitle (.srt) format') do
    cli_options[:export_srt] = true
  end

  opts.on('--txt', 'Convert transcript JSON to timestamped text (.txt) format') do
    cli_options[:export_txt] = true
  end

  opts.on('--recut', 'Recut audio using existing .cuts.json metadata without Whisper or LLM') do
    cli_options[:recut] = true
  end

  opts.on('--force-llm', 'Force re-running LLM ad detection even if .cuts.json exists') do
    cli_options[:force_llm] = true
  end

  opts.on('--force-transcribe', 'Force re-transcribing audio even if .transcript.json exists') do
    cli_options[:force_transcribe] = true
  end

  opts.on('--no-transcript', 'Disable saving default .transcript.json file') do
    cli_options[:save_transcript] = false
  end

  opts.on('--use-chunks', 'Split long audio into chunks for more reliable transcription') do
    cli_options[:use_chunks] = true
  end

  opts.on('--extract-keywords', 'Extract keywords via LLM from first minutes to improve Whisper transcription accuracy') do
    cli_options[:extract_keywords] = true
  end

  opts.on('-t', '--transcribe-minutes MINUTES', String, 'Only transcribe first N minutes (e.g. -t 10m)') do |t|
    cli_options[:transcribe_minutes] = t
  end

  opts.on('-q', '--quiet', 'Suppress progress and informational output') do
    cli_options[:quiet] = true
  end

  opts.on('--list-llms', '--list-profiles', 'List all configured LLM profiles and exit') do
    cli_options[:list_llms] = true
  end

  opts.on('--use-llm ID_OR_NAME', '--profile ID_OR_NAME', 'Select LLM profile ID or name for this run') do |p|
    cli_options[:use_llm] = p
  end

  opts.on('--set-default ID', Integer, 'Set default active LLM profile ID in config file and exit') do |id|
    cli_options[:set_default] = id
  end

  opts.on('--copy_llm_from_opencode', 'Import LLM model settings & API key from OpenCode config') do
    cli_options[:copy_opencode] = true
  end

  opts.on('-h', '--help', 'Show help message') do
    display_usage_and_help
    exit 0
  end
end.parse!

if cli_options[:copy_opencode]
  copy_llm_from_opencode(config)
  exit 0 if ARGV.empty?
end

if cli_options[:list_llms]
  list_profiles(config)
  exit 0
end

if cli_options[:set_default]
  target_id = cli_options[:set_default]
  profile = config['profiles'].find { |p| p['id'] == target_id }
  if profile
    config['active_profile_id'] = target_id
    save_config(config)
    puts "✅ Default LLM profile updated to [#{target_id}] #{profile['name']}"
  else
    puts "❌ Error: Profile ID [#{target_id}] not found in configuration."
  end
  exit 0
end

if ARGV.empty?
  display_usage_and_help
  exit 0
end

selected_profile = nil
if cli_options[:use_llm]
  query = cli_options[:use_llm]
  if query =~ /^\d+$/
    selected_profile = config['profiles'].find { |p| p['id'] == query.to_i }
  else
    selected_profile = config['profiles'].find { |p| p['name'].downcase.include?(query.downcase) || p['model'].downcase.include?(query.downcase) }
  end
  unless selected_profile
    puts "❌ Error: Profile '#{query}' not found."
    list_profiles(config)
    exit 1
  end
else
  selected_profile = config['profiles'].find { |p| p['id'] == config['active_profile_id'] } || config['profiles'].first
end

batch_start_time = Time.now

expanded_args = []
ARGV.each do |arg|
  if File.directory?(arg)
    mp3_files = Dir.glob(File.join(arg, '**', '*.mp3')).sort
    if mp3_files.empty?
      puts "⚠️  No MP3 files found in directory '#{arg}'." unless cli_options[:quiet]
    else
      expanded_args.concat(mp3_files)
    end
  else
    expanded_args << arg
  end
end
ARGV.replace(expanded_args)

total_files = ARGV.size

ARGV.each_with_index do |input_file, idx|
  file_start_time = Time.now

  if total_files > 1 && !cli_options[:quiet]
    puts "\n" + "═" * 65
    puts "📁 Processing File [#{idx + 1}/#{total_files}]: '#{input_file}'"
    puts "═" * 65
  end

  if input_file.end_with?('.json')
    unless File.exist?(input_file)
      puts "❌ Error: Transcript JSON file '#{input_file}' not found."
      next
    end

    puts "📄 Processing transcript JSON file: '#{input_file}'" unless cli_options[:quiet]

    if !cli_options[:export_srt] && !cli_options[:export_txt]
      cli_options[:export_srt] = true
      cli_options[:export_txt] = true
    end

    convert_json_to_srt(input_file, custom_path: cli_options[:transcript_path], quiet: cli_options[:quiet]) if cli_options[:export_srt]
    convert_json_to_txt(input_file, custom_path: cli_options[:transcript_path], quiet: cli_options[:quiet]) if cli_options[:export_txt]
    next
  end

  if input_file.end_with?('.precut')
    precut_file = input_file
    main_mp3_file = input_file.sub(/\.precut$/, '')
  else
    main_mp3_file = input_file
    precut_file = "#{input_file}.precut"
  end

  if File.exist?(precut_file)
    check_precut_symlink(precut_file)
    source_audio_file = precut_file
    puts "📦 Found existing pre-cut audio source: '#{precut_file}'" unless cli_options[:quiet]
  elsif File.exist?(main_mp3_file)
    source_audio_file = main_mp3_file
  else
    puts "❌ Error: Input audio file '#{input_file}' (or '#{precut_file}') not found."
    next
  end

  base_name = main_mp3_file.sub(/\.[^.]+\z/, '')
  json_file = cli_options[:transcript_path] || "#{base_name}.transcript.json"
  cuts_file = cli_options[:transcript_path] || "#{base_name}.cuts.json"

  if total_files > 1 && cli_options[:output] && File.directory?(cli_options[:output])
    output_file = File.join(cli_options[:output], File.basename(main_mp3_file))
  else
    output_file = cli_options[:output] || main_mp3_file
  end

  speed_factor = (config['whisper_speed_factor'] || 7.0).to_f

  if File.exist?(json_file) && File.exist?(cuts_file) && !cli_options[:force_transcribe] && !cli_options[:force_llm]
    puts "⏭️  Skipping '#{input_file}' (both .transcript.json and .cuts.json exist). Use --force-transcribe or --force-llm to reprocess." unless cli_options[:quiet]
    next
  end

  puts "🎧 Processing local file: '#{source_audio_file}'" unless cli_options[:quiet]
  total_duration = get_audio_duration(source_audio_file)

  if cli_options[:transcribe_minutes]
    raw = cli_options[:transcribe_minutes].to_s
    minutes = raw.sub(/[mM].*$/, '').to_f
    truncate_sec = minutes * 60.0
    if truncate_sec < total_duration
      work_dir = work_dir_for(source_audio_file)
      FileUtils.mkdir_p(work_dir)
      truncated_path = File.join(work_dir, "#{File.basename(source_audio_file)}.truncated.wav")
      verify_temp_file(truncated_path)
      unless cli_options[:quiet]
        puts "   📖 Reading only first #{format_time(truncate_sec)} (#{raw})..."
      end
      system('ffmpeg', '-y', '-loglevel', 'error',
             '-ss', '0', '-i', source_audio_file,
             '-to', sprintf('%.3f', truncate_sec),
             '-ar', '16000', '-ac', '1', '-c:a', 'pcm_s16le', truncated_path)
      source_audio_file = truncated_path
      total_duration = get_audio_duration(source_audio_file)
    end
  end

  if cli_options[:recut]
    cuts_file = cli_options[:transcript_path] || "#{base_name}.cuts.json"
    unless File.exist?(cuts_file)
      puts "❌ Error: Cut metadata JSON file '#{cuts_file}' not found for recutting."
      next
    end

    if File.exist?(json_file) && !cli_options[:force_transcribe] && !cli_options[:force_llm]
      puts "⏭️  Skipping '#{input_file}' (.transcript.json exists). Use --force-transcribe or --force-llm to reprocess." unless cli_options[:quiet]
      next
    end

    puts "✂️ Recutting audio using existing cut metadata: '#{cuts_file}'" unless cli_options[:quiet]
    cuts_data = JSON.parse(File.read(cuts_file))
    existing_raw_intervals = (cuts_data['cut_intervals'] || []).map do |ad|
      st = (ad['start_sec'] || ad['start']).to_f
      en = (ad['end_sec'] || ad['end']).to_f
      { 'start' => st, 'end' => en, 'reason' => ad['reason'] }
    end

    existing_raw_intervals = merge_intervals(existing_raw_intervals, quiet: cli_options[:quiet]) unless existing_raw_intervals.empty?

    cuts_result = save_cuts_json(main_mp3_file, total_duration, existing_raw_intervals, selected_profile, quiet: cli_options[:quiet])
    keep_segments = cuts_result[:keep_segments]

    if keep_segments.empty?
      puts "⚠️  No keep segments found in cut metadata." unless cli_options[:quiet]
      next
    end

    updated_cuts = JSON.parse(File.read(cuts_file)) rescue {}
    merged_intervals = (updated_cuts['merged_cut_intervals'] || []).map do |m|
      [m['start'].to_f, m['end'].to_f]
    end

    unless cli_options[:quiet]
      if merged_intervals.empty?
        puts "✅ No cut intervals specified in metadata!"
      else
        puts "\n" + "=" * 65
        puts "✂️ CUT INTERVALS TO REMOVE (#{merged_intervals.size} segment(s)):"
        puts "=" * 65
        merged_intervals.each do |(st, en)|
          duration = en - st
          puts "  • [#{format_time(st)} -> #{format_time(en)}] (#{duration.round(1)}s)"
        end
        puts "=" * 65 + "\n"
      end
    end

    t0_recut = Time.now
    puts "🎬 Cutting ads with ffmpeg (#{keep_segments.size} non-ad clips)..." unless cli_options[:quiet]

    work_dir = work_dir_for(output_file)
    FileUtils.mkdir_p(work_dir)
    temp_output_file = File.join(work_dir, "#{File.basename(output_file)}.tmp#{File.extname(output_file)}")
    verify_temp_file(temp_output_file)

    if cut_audio_ffmpeg(source_audio_file, keep_segments, temp_output_file)
      recut_duration = Time.now - t0_recut
      puts "⏱️  Audio Recutting finished in #{format_clock(recut_duration)}" unless cli_options[:quiet]

      if source_audio_file == main_mp3_file && File.exist?(main_mp3_file)
        check_precut_symlink(precut_file)
        safe_move(main_mp3_file, precut_file)
        puts "📦 Original file preserved at: '#{precut_file}'" unless cli_options[:quiet]
      end

      safe_move(temp_output_file, output_file)
      FileUtils.rm_rf(work_dir) if Dir.exist?(work_dir)

      new_duration = get_audio_duration(output_file)
      actual_cut = total_duration - new_duration
      pct_cut = (actual_cut / total_duration * 100).round(1)
      file_total_duration = Time.now - file_start_time

      unless cli_options[:quiet]
        puts "\n" + "=" * 65
        puts "📊 DURATION & TIME SAVED SUMMARY (RECUT):"
        puts "=" * 65
        puts "  • Original Episode Length: #{format_time(total_duration)} (#{total_duration.round(1)}s)"
        puts "  • Total Ad Time Cut:       #{format_time(actual_cut)} (#{actual_cut.round(1)}s)"
        puts "  • New Episode Length:      #{format_time(new_duration)} (#{new_duration.round(1)}s)"
        puts "  • Reduction:               #{pct_cut}% of episode trimmed"
        puts "  • Total Recut Time:        #{format_clock(file_total_duration)}"
        puts "=" * 65 + "\n"

        puts "🎉 Success! Recut ad-free episode saved to: '#{output_file}'"
      else
        FileUtils.rm_f(temp_output_file)
        FileUtils.rm_rf(work_dir) if Dir.exist?(work_dir)
      end
    end

    next
  end

  cost_info = get_profile_cost(selected_profile)

  puts "⏱️  Original Episode Length: #{format_time(total_duration)} (#{total_duration.round(1)}s)" unless cli_options[:quiet]
  puts "🤖 Active LLM Profile:       " + Rainbow("[#{selected_profile['id']}] #{selected_profile['name']} (#{selected_profile['model']})").green.bold unless cli_options[:quiet]
  puts "💰 LLM Model Pricing:        #{cost_info[:cost_str]} (#{cost_info[:est_1h_str]})" unless cli_options[:quiet]

  t0_step1 = Time.now
  is_newly_transcribed = false

  whisper_language = config['whisper_language'].to_s
  whisper_prompt = config['whisper_prompt'].to_s
  id3_tags = {}

  if File.exist?(json_file)
    puts "📄 Found existing transcript JSON file: '#{json_file}'. Reusing transcript..." unless cli_options[:quiet]
    transcription_data = JSON.parse(File.read(json_file))
    step1_duration = Time.now - t0_step1
    puts "⏱️  Step 1/3 (Transcript Loaded) finished in #{format_clock(step1_duration)}" unless cli_options[:quiet]
  else
    puts "🚀 Step 1/3: Transcribing audio via AMD GPU Whisper server..." unless cli_options[:quiet]

    if whisper_prompt.empty?
      id3_tags = extract_id3_tags(source_audio_file)
      tag_text = [id3_tags['title'], id3_tags['artist'], id3_tags['album'], id3_tags['genre'],
                  id3_tags['comment'], id3_tags['description'], id3_tags['synopsis'],
                  id3_tags['purl'], id3_tags['encodedby'], id3_tags['copyright']]
                   .compact.reject(&:empty?).join("\n")
      unless tag_text.strip.empty?
        unless cli_options[:quiet]
          puts "   🏷️  Extracted ID3 metadata: #{id3_tags.keys.join(', ')}"
          puts "   🔑 Extracting keywords from metadata to improve transcription accuracy..."
        end
        extracted = extract_keywords_llm(tag_text, selected_profile, quiet: cli_options[:quiet])
        unless extracted.empty?
          whisper_prompt = extracted
          puts "   🔑 Using keywords: #{whisper_prompt}" unless cli_options[:quiet]
        end
      else
        puts "   ⚠️  No ID3 metadata found in file for keyword extraction." unless cli_options[:quiet]
      end
    end

    chunk_duration = (config['chunk_duration_sec'] || 0).to_i
    use_chunks = cli_options[:use_chunks] || (chunk_duration > 0 && total_duration > chunk_duration * 1.5)
    docker_container = config['whisper_docker_container'].to_s
    if docker_container.empty?
      docker_container = detect_whisper_docker_container(config['whisper_url'])
      unless cli_options[:quiet] || docker_container.nil?
        puts "   🐳 Auto-detected whisper Docker container: '#{docker_container}'"
      end
    end
    docker_container = nil if docker_container.to_s.empty?

    whisper_lang_arg = whisper_language.empty? ? nil : whisper_language
    whisper_prompt_arg = whisper_prompt.empty? ? nil : whisper_prompt

    if use_chunks
      parallel_chunks = (config['parallel_chunks'] || 1).to_i
      puts "   📦 Audio is #{format_time(total_duration)} long — splitting into #{((total_duration / chunk_duration).ceil)} chunks of #{format_time(chunk_duration)} for reliability..." unless cli_options[:quiet]
      puts "   ⚡ Parallel chunks: #{parallel_chunks}" if parallel_chunks > 1 && !cli_options[:quiet]
      transcription_data = transcribe_chunks(
        source_audio_file,
        config['whisper_url'],
        quiet: cli_options[:quiet],
        total_duration: total_duration,
        speed_factor: speed_factor,
        chunk_duration: chunk_duration,
        parallel: parallel_chunks,
        docker_container: docker_container,
        prompt: whisper_prompt_arg,
        language: whisper_lang_arg
      )
    else
      begin
        transcription_data = transcribe_whisper(
          source_audio_file,
          config['whisper_url'],
          quiet: cli_options[:quiet],
          total_duration: total_duration,
          speed_factor: speed_factor,
          docker_container: docker_container,
          prompt: whisper_prompt_arg,
          language: whisper_lang_arg
        )
      rescue => e
        if e.message.include?('failed to') && total_duration > 300
          unless cli_options[:quiet]
            puts "\n⚠️ Full-file transcription failed — retrying in chunks..."
          end
          chunk_duration = config['chunk_duration_sec'] || 900
          chunk_duration = 900 if chunk_duration.to_i <= 0
          transcription_data = transcribe_chunks(
            source_audio_file,
            config['whisper_url'],
            quiet: cli_options[:quiet],
            total_duration: total_duration,
            speed_factor: speed_factor,
            chunk_duration: chunk_duration.to_i,
            parallel: 1,
            docker_container: docker_container,
            prompt: whisper_prompt_arg,
            language: whisper_lang_arg
          )
        else
          raise
        end
      end
    end
    step1_duration = Time.now - t0_step1
    puts "⏱️  Step 1/3 (Transcription) finished in #{format_clock(step1_duration)}" unless cli_options[:quiet]
    is_newly_transcribed = true
  end

  detected_lang = transcription_data['language'] || transcription_data.dig('segments', 0, 'language') || ''
  unless cli_options[:quiet] || detected_lang.empty?
    lang_label = whisper_language.empty? ? "(auto-detected)" : "(config override)"
    puts "   🌐 Detected language: #{detected_lang.upcase} #{lang_label}"
  end

  if whisper_language.empty? && is_newly_transcribed
    full_text = transcription_data['text'] || (transcription_data['segments'] || []).map { |s| s['text'] }.join(' ')
    script_lang = detect_script_language(full_text)
    if script_lang && script_lang != detected_lang
      transcription_data['language'] = script_lang
      transcription_data['_original_whisper_language'] = detected_lang
      puts "   ✏️  Corrected language from #{detected_lang.upcase} to #{script_lang.upcase} (detected from script)" unless cli_options[:quiet]
    end
  end

  unless validate_transcript_sanity(transcription_data, total_duration, quiet: cli_options[:quiet])
    next
  end

  if is_newly_transcribed && cli_options[:save_transcript]
    save_json_transcript(main_mp3_file, transcription_data, json_file, quiet: cli_options[:quiet], id3_tags: id3_tags)
  end

  if cli_options[:export_srt]
    convert_json_to_srt(json_file, transcription_data, custom_path: cli_options[:transcript_path], quiet: cli_options[:quiet])
  end

  if cli_options[:export_txt]
    convert_json_to_txt(json_file, transcription_data, total_duration: total_duration, custom_path: cli_options[:transcript_path], quiet: cli_options[:quiet])
  end

  if cli_options[:export_srt] || cli_options[:export_txt]
    file_total_duration = Time.now - file_start_time
    puts "⏱️  Export completed in #{format_clock(file_total_duration)}" unless cli_options[:quiet]
    next
  end

  if cli_options[:transcribe_minutes]
    file_total_duration = Time.now - file_start_time
    puts "⏱️  Preview transcription completed in #{format_clock(file_total_duration)}" unless cli_options[:quiet]
    puts "   📄 Transcript saved — original file was not modified." unless cli_options[:quiet]
    File.unlink(source_audio_file) if source_audio_file.end_with?('.truncated.wav') rescue nil
    next
  end

  segments = transcription_data['segments'] || []
  if segments.empty? && transcription_data['text']
    formatted_transcript = "[0.0s -> #{total_duration.round(1)}s] #{transcription_data['text']}"
  else
    formatted_transcript = segments.map do |seg|
      st = seg['start'] || 0.0
      en = seg['end'] || 0.0
      "[#{st.round(1)}s -> #{en.round(1)}s] #{seg['text']}"
    end.join("\n")
  end

  t0_step2 = Time.now
  puts "🧠 Step 2/3: Detecting ad/sponsor segments via LLM (#{selected_profile['model']})..." unless cli_options[:quiet]
  ad_segments = detect_ads_llm(formatted_transcript, selected_profile)
  ad_segments = merge_intervals(ad_segments, quiet: cli_options[:quiet]) unless ad_segments.empty?
  step2_duration = Time.now - t0_step2
  puts "⏱️  Step 2/3 (Ad Detection) finished in #{format_clock(step2_duration)}" unless cli_options[:quiet]

  if ad_segments.empty?
    save_cuts_json(main_mp3_file, total_duration, ad_segments, selected_profile, quiet: cli_options[:quiet])
    file_total_duration = Time.now - file_start_time
    unless cli_options[:quiet]
      puts "✅ No ad segments detected by LLM!"
      puts "📊 TIMING SUMMARY:"
      puts "   • Original Length:     #{format_time(total_duration)} (#{total_duration.round(1)}s)"
      puts "   • Time Cut:            00:00.0 (0.0s)"
      puts "   • New Episode Length:  #{format_time(total_duration)} (#{total_duration.round(1)}s)"
      puts "   • Running Times:"
      puts "       - Step 1 (Transcription): #{format_clock(step1_duration)}"
      puts "       - Step 2 (Ad Detection):  #{format_clock(step2_duration)}"
      puts "       - Total File Processing:  #{format_clock(file_total_duration)}"
    end
    FileUtils.cp(source_audio_file, output_file) unless source_audio_file == output_file
    puts "🎉 Result saved to: '#{output_file}'" unless cli_options[:quiet]
    next
  end

  unless cli_options[:quiet]
    puts "\n" + "=" * 65
    puts "✂️ AD SEGMENTS DETECTED TO REMOVE (#{ad_segments.size} segment(s)):"
    puts "=" * 65
    ad_segments.each do |ad|
      st = ad['start'].to_f
      en = ad['end'].to_f
      duration = en - st
      reason = ad['reason'] || 'Ad segment'
      puts "  • [#{format_time(st)} -> #{format_time(en)}] (#{duration.round(1)}s): #{reason}"
    end
    puts "=" * 65 + "\n"
  end

  cuts_result = save_cuts_json(main_mp3_file, total_duration, ad_segments, selected_profile, quiet: cli_options[:quiet])
  keep_segments = cuts_result[:keep_segments]

  t0_step3 = Time.now
  puts "🎬 Step 3/3: Cutting ads with ffmpeg (#{keep_segments.size} non-ad clips)..." unless cli_options[:quiet]

  work_dir = work_dir_for(output_file)
  FileUtils.mkdir_p(work_dir)
  temp_output_file = File.join(work_dir, "#{File.basename(output_file)}.tmp#{File.extname(output_file)}")
  verify_temp_file(temp_output_file)

  if cut_audio_ffmpeg(source_audio_file, keep_segments, temp_output_file)
    step3_duration = Time.now - t0_step3
    puts "⏱️  Step 3/3 (Audio Cutting) finished in #{format_clock(step3_duration)}" unless cli_options[:quiet]

    if source_audio_file == main_mp3_file && File.exist?(main_mp3_file)
      check_precut_symlink(precut_file)
      safe_move(main_mp3_file, precut_file)
      puts "📦 Original file preserved at: '#{precut_file}'" unless cli_options[:quiet]
    end

    safe_move(temp_output_file, output_file)
    FileUtils.rm_rf(work_dir) if Dir.exist?(work_dir)

    new_duration = get_audio_duration(output_file)
    actual_cut = total_duration - new_duration
    pct_cut = (actual_cut / total_duration * 100).round(1)
    file_total_duration = Time.now - file_start_time

    unless cli_options[:quiet]
      puts "\n" + "=" * 65
      puts "📊 DURATION & TIME SAVED SUMMARY:"
      puts "=" * 65
      puts "  • Original Episode Length: #{format_time(total_duration)} (#{total_duration.round(1)}s)"
      puts "  • Total Ad Time Cut:       #{format_time(actual_cut)} (#{actual_cut.round(1)}s across #{ad_segments.size} segment(s))"
      puts "  • New Episode Length:      #{format_time(new_duration)} (#{new_duration.round(1)}s)"
      puts "  • Reduction:               #{pct_cut}% of episode trimmed"
      puts "  • Running Times:"
      puts "      - Step 1 (Transcription): #{format_clock(step1_duration)}"
      puts "      - Step 2 (Ad Detection):  #{format_clock(step2_duration)}"
      puts "      - Step 3 (Audio Cut):     #{format_clock(step3_duration)}"
      puts "      - Total File Processing:  #{format_clock(file_total_duration)}"
      puts "=" * 65 + "\n"

      puts "🎉 Success! Ad-free episode saved to: '#{output_file}'"
    end
  else
    FileUtils.rm_f(temp_output_file)
    FileUtils.rm_rf(work_dir) if Dir.exist?(work_dir)
    puts "❌ Failed to output ad-free audio for '#{input_file}'."
  end

  File.unlink(source_audio_file) if source_audio_file.end_with?('.truncated.wav') rescue nil
end

if total_files > 1 && !cli_options[:quiet]
  batch_duration = Time.now - batch_start_time
  puts "\n" + "═" * 65
  puts "🎉 Batch Completed! Processed #{total_files} file(s) in #{format_clock(batch_duration)}."
  puts "═" * 65 + "\n"
end
