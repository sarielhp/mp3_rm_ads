#!/usr/bin/env ruby
# frozen_string_literal: true

require 'json'
require 'fileutils'

template = {
  "_instructions" => "Configuration template for abs. Place in ~/.config/abs/config.json",
  "whisper_url" => "http://127.0.0.1:8088/inference",
  "whisper_speed_factor" => 7.0,
  "whisper_docker_container" => "",
  "whisper_language" => "",
  "whisper_prompt" => "",
  "whisper_wake_command" => "",
  "chunk_duration_sec" => 0,
  "parallel_chunks" => 1,
  "active_profile_id" => 1,
  "active_whisper_id" => 1,
  "whisper_profiles" => [
    {
      "id" => 1,
      "name" => "Local Whisper Server (Default)",
      "url" => "http://127.0.0.1:8088/inference",
      "speed_factor" => 7.0,
      "docker_container" => "",
      "language" => "",
      "prompt" => "",
      "wake_command" => ""
    },
    {
      "id" => 2,
      "name" => "Cloud8 VM",
      "url" => "http://cloud8:8000/v1/audio/transcriptions",
      "speed_factor" => 7.0,
      "docker_container" => "",
      "language" => "",
      "prompt" => "",
      "wake_command" => "cloud8 wake"
    }
  ],
  "podcasts_dir" => "/path/to/podcasts",
  "remote_host" => "",
  "default_processing" => "local",
  "remote_work_dir" => "~/.abs_remote",
  "audiobookshelf_url" => "http://127.0.0.1:8080",
  "audiobookshelf_user" => "admin",
  "audiobookshelf_pass" => "password",
  "profiles" => [
    {
      "id" => 1,
      "name" => "Ollama Local (llama3.1:8b)",
      "type" => "ollama",
      "url" => "http://127.0.0.1:11434/v1/chat/completions",
      "model" => "llama3.1:8b",
      "api_key" => ""
    },
    {
      "id" => 2,
      "name" => "OpenRouter - Claude 3.5 Sonnet",
      "type" => "openrouter",
      "url" => "https://openrouter.ai/api/v1/chat/completions",
      "model" => "anthropic/claude-3.5-sonnet",
      "api_key" => "sk-or-v1-..."
    },
    {
      "id" => 3,
      "name" => "OpenRouter - DeepSeek V4 Flash",
      "type" => "openrouter",
      "url" => "https://openrouter.ai/api/v1/chat/completions",
      "model" => "deepseek/deepseek-v4-flash",
      "api_key" => "sk-or-v1-..."
    },
    {
      "id" => 4,
      "name" => "OpenRouter - Gemini 2.5 Flash",
      "type" => "openrouter",
      "url" => "https://openrouter.ai/api/v1/chat/completions",
      "model" => "google/gemini-2.5-flash",
      "api_key" => "sk-or-v1-..."
    }
  ],
  "tui_colors" => {
    "title" => "#5fffff",
    "header" => "#ff79c6",
    "selected" => "#bd93f9",
    "badge_ad_free" => "#50fa7b",
    "badge_queued" => "#f1fa8c"
  }
}

root_dir = File.expand_path('..', __dir__)
examples_dir = File.join(root_dir, 'examples')
FileUtils.mkdir_p(examples_dir)

target_file = File.join(examples_dir, 'config.json.template')
File.write(target_file, JSON.pretty_generate(template) + "\n")

puts "Generated configuration template at: #{target_file}"
