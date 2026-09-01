#!/usr/bin/env ruby
# frozen_string_literal: true

require 'open3'
require 'fileutils'
require 'optparse'
require 'pathname'

options = {
  iterations: 5,
  model: 'gemini-3.1-pro-high',
  effort: 'high',
  reports_dir: 'reports',
  prefix: 'code_review',
  triage: 's',
  check_cmd: nil,
  dir: Dir.pwd,
  archive: true,
  early_exit: true
}

parser = OptionParser.new do |opts|
  opts.banner = <<~BANNER
    Usage: agy_review_code_loop [options]

    Run automated, multi-iteration code review and remediation cycles across any Git codebase
    using sandboxed AI agents (bws + agy-run-wild) with automatic report archiving, numbering, and convergence exit.

    Options:
  BANNER

  opts.on('-n', '--iterations N', Integer, 'Number of review/fix cycles to run (default: 5)') do |v|
    options[:iterations] = v
  end

  opts.on('-m', '--model MODEL', String, 'Model to use for review & fix (default: gemini-3.1-pro-high)') do |v|
    options[:model] = v
  end

  opts.on('-e', '--effort EFFORT', String, 'Reasoning effort (low|medium|high, default: high)') do |v|
    options[:effort] = v
  end

  opts.on('-r', '--reports-dir DIR', String, 'Directory for review & fix reports (default: reports)') do |v|
    options[:reports_dir] = v
  end

  opts.on('-p', '--prefix PREFIX', String, 'Prefix for report filenames (default: code_review)') do |v|
    options[:prefix] = v
  end

  opts.on('-c', '--check-cmd CMD', String, 'Custom verification / test command (auto-detected by default)') do |v|
    options[:check_cmd] = v
  end

  opts.on('-t', '--triage ACTION', String, 'Triage action on exit: s (squash), m (merge), k (keep) (default: s)') do |v|
    options[:triage] = v
  end

  opts.on('-C', '--dir DIR', String, 'Target repository directory (default: current directory)') do |v|
    options[:dir] = File.expand_path(v)
  end

  opts.on('--[no-]archive', 'Archive existing reports in reports/archive/ before starting (default: true)') do |v|
    options[:archive] = v
  end

  opts.on('--[no-]early-exit', 'Stop loop early when convergence is reached with 0 code changes (default: true)') do |v|
    options[:early_exit] = v
  end

  opts.on('-h', '--help', 'Show this help message') do
    puts opts
    exit 0
  end
end

parser.parse!

# Verify Git Repository
Dir.chdir(options[:dir])
git_root, _, status = Open3.capture3('git rev-parse --show-toplevel')
unless status.success?
  warn "\e[31m✗ Error: #{options[:dir]} is not a Git repository.\e[0m"
  exit 1
end

root_dir = git_root.strip
Dir.chdir(root_dir)

# Ensure reports directory exists
reports_path = File.join(root_dir, options[:reports_dir])
archive_path = File.join(reports_path, 'archive')
FileUtils.mkdir_p(reports_path)

# Archive previous reports if present
if options[:archive]
  old_reports = Dir.glob(File.join(reports_path, "#{options[:prefix]}_*.md"))
  if old_reports.any?
    FileUtils.mkdir_p(archive_path)
    old_reports.each do |f|
      dest = File.join(archive_path, File.basename(f))
      FileUtils.mv(f, dest)
    end
    puts "\e[33m--> Archived #{old_reports.size} previous reports into #{options[:reports_dir]}/archive/\e[0m"
  end
end

# Auto-detect verification check command if not supplied
def detect_check_command(root_dir)
  if File.exist?(File.join(root_dir, 'tools', 'check.rb'))
    'ruby tools/check.rb'
  elsif File.exist?(File.join(root_dir, 'Makefile')) && File.read(File.join(root_dir, 'Makefile')).include?('check:')
    'make check'
  elsif File.exist?(File.join(root_dir, 'Cargo.toml'))
    'cargo test'
  elsif File.exist?(File.join(root_dir, 'go.mod'))
    'go test ./...'
  elsif File.exist?(File.join(root_dir, 'package.json'))
    'npm test'
  elsif File.exist?(File.join(root_dir, 'pyproject.toml')) || File.exist?(File.join(root_dir, 'pytest.ini'))
    'pytest'
  end
end

check_cmd = options[:check_cmd] || detect_check_command(root_dir)

# Auto-detect project guideline documents
guideline_hints = []
guideline_hints << "strictly follow all rules in AGENTS.md" if File.exist?(File.join(root_dir, 'AGENTS.md'))
guideline_hints << "strictly follow all rules in CLAUDE.md" if File.exist?(File.join(root_dir, 'CLAUDE.md'))
guideline_str = guideline_hints.empty? ? "" : " Ensure you #{guideline_hints.join(' and ')}."

# Graceful cleanup on Ctrl+C
trap('INT') do
  puts "\n\e[33m[!] Process interrupted by user. Exiting cleanly...\e[0m"
  exit 130
end

puts "\e[1;36m======================================================================\e[0m"
puts "\e[1;36m  Automated Code Review & Fix Loop (agy_review_code_loop)           \e[0m"
puts "\e[1;36m======================================================================\e[0m"
puts "  Target Repo  : \e[1m#{root_dir}\e[0m"
puts "  Reports Dir  : \e[1m#{options[:reports_dir]}/ (archive: #{options[:archive]})\e[0m"
puts "  Cycles       : \e[1m#{options[:iterations]} (early exit: #{options[:early_exit]})\e[0m"
puts "  Model        : \e[1m#{options[:model]} (effort: #{options[:effort]})\e[0m"
puts "  Check Command: \e[1m#{check_cmd || '(none detected - skipping host check)'}\e[0m"
puts "\e[1;36m======================================================================\e[0m\n"

completed_reports = []

options[:iterations].times do |i|
  # Determine next sequential index across both reports/ and reports/archive/
  glob_pattern = File.join(reports_path, "**", "#{options[:prefix]}_*.md")
  existing_nums = Dir.glob(glob_pattern).map do |f|
    next unless f =~ /#{Regexp.escape(options[:prefix])}_(\d+)(?:_fix)?\.md$/
    $1.to_i
  end.compact

  next_num = (existing_nums.max || 0) + 1
  idx_str = format('%03d', next_num)
  branch_name = "review-#{idx_str}"
  review_file = "#{options[:reports_dir]}/#{options[:prefix]}_#{idx_str}.md"
  fix_file = "#{options[:reports_dir]}/#{options[:prefix]}_#{idx_str}_fix.md"

  puts "\n\e[1;34m----------------------------------------------------------------------\e[0m"
  puts "\e[1;34m▶ [Cycle #{i + 1}/#{options[:iterations]}] Iteration #{idx_str} (Branch: #{branch_name})\e[0m"
  puts "\e[1;34m----------------------------------------------------------------------\e[0m"

  # 1. Commit pre-sandbox checkpoint if working tree is dirty
  dirty_status = `git status --porcelain`.strip
  if dirty_status.length > 0
    puts "--> Auto-committing working changes before sandbox dispatch..."
    `git add -A && git commit --no-verify -m "wip: pre-sandbox checkpoint #{idx_str}"`
  end

  # 2. Formulate grounded prompt with convergence rules
  prompt_parts = []
  prompt_parts << "/goal Conduct a deep, thorough code review of the codebase."
  prompt_parts << "First, verify that the next available report file is #{review_file}."
  prompt_parts << "Write a comprehensive review report into #{review_file} covering architecture, bug detection, error handling, edge cases, performance, security, and project compliance."
  prompt_parts << "If high-priority issues, bugs, architectural flaws, data races, or file-size violations (>300 lines) are found, implement fixes for them and verify all tests pass (e.g. running `#{check_cmd}`)." if check_cmd
  prompt_parts << "IMPORTANT FOR CONVERGENCE: If the codebase is already clean, fully hardened, and strictly compliant with zero actionable defects or rule violations, DO NOT make arbitrary cosmetic refactorings. Explicitly state in #{review_file} and #{fix_file} that the codebase is fully compliant and no code modifications were necessary."
  prompt_parts << guideline_str unless guideline_str.empty?
  prompt_parts << "Finally, write a fix/status summary report into #{fix_file} detailing all fixes applied (or confirming zero changes if already hardened) and test verification results."

  goal_prompt = prompt_parts.join(" ")

  escaped_cmd = %(bws gw -b #{branch_name} -- agy-run-wild --model #{options[:model]} --effort #{options[:effort]} -p '#{goal_prompt.gsub("'", "\\'")}')
  puts "--> Dispatching sandboxed agent..."

  # 3. Stream output and auto-reply to triage prompt
  triage_sent = false
  Open3.popen2e(escaped_cmd) do |stdin, stdout_err, wait_thr|
    stdout_err.each_line do |line|
      print line
      if line.include?('What would you like to do with branch') && !triage_sent
        puts "\n\e[32m--> Auto-selecting triage action [#{options[:triage]}]...\e[0m"
        stdin.puts "#{options[:triage]}\n"
        triage_sent = true
      end
    end
    status = wait_thr.value
    unless status.success?
      warn "\n\e[31m✗ Error: Sandbox agent failed or exited with status #{status.exitstatus} on iteration #{idx_str}.\e[0m"
      exit 1
    end
  end

  # 4. Run post-merge verification on host if check_cmd available
  if check_cmd
    puts "\n--> Running host verification (#{check_cmd})..."
    unless system(check_cmd)
      warn "\n\e[31m✗ Host verification failed after merging iteration #{idx_str}!\e[0m"
      exit 1
    end
    puts "\e[32m✓ Host verification passed.\e[0m"
  end

  completed_reports << { review: review_file, fix: fix_file, index: idx_str }

  # 5. Check for convergence (zero non-report file changes in the last commit)
  changed_files_out, _, _ = Open3.capture3('git diff --name-only HEAD^ HEAD')
  changed_code_files = changed_files_out.lines.map(&:strip).reject do |f|
    f.start_with?(options[:reports_dir]) || f.empty?
  end

  if changed_code_files.empty?
    puts "\n\e[1;32m★ Convergence reached! Agent made 0 code modifications (codebase confirmed hardened & compliant).\e[0m"
    if options[:early_exit]
      puts "\e[1;32m--> Stopping loop early after iteration #{idx_str} as no further code fixes are needed.\e[0m"
      break
    end
  end
end

puts "\n\e[1;32m======================================================================\e[0m"
puts "\e[1;32m✓ All review & fix cycles completed!\e[0m"
puts "\e[1;32m======================================================================\e[0m"
puts "Generated reports:"
completed_reports.each do |rep|
  puts "  • Iteration #{rep[:index]}: #{rep[:review]} & #{rep[:fix]}"
end
puts "\n"
