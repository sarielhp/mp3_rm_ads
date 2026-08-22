#!/usr/bin/env ruby
# frozen_string_literal: true

require 'open3'
require 'fileutils'

dir = File.expand_path(__dir__)
binary = File.join(dir, 'image_show')
image_path = '/home/mp3/mp3/rock/wham/make_it_big/cover.jpg'

system('go', 'build', '-o', binary, '.', chdir: dir) || abort("Build failed")

# Execute binary directly replacing the current process so stdout goes straight to terminal
exec(binary, image_path)
