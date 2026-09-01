#!/usr/bin/env ruby
content = File.read("docker.go")
content = content.sub("var dockerMu syncMutex\\n\\npackage main", "package main\\n\\nvar dockerMu syncMutex")
File.write("docker.go", content)
