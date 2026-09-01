#!/usr/bin/env ruby
lines = File.readlines('transcribe.go')
File.write('transcribe_wav.go', "package main\n\n" + lines[17..54].join)
lines.slice!(17..54)
File.write('transcribe.go', lines.join)
