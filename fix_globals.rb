#!/usr/bin/env ruby

content = File.read("docker.go")
content = "var dockerMu syncMutex\n\n" + content
File.write("docker.go", content)

content = File.read("openrouter.go")
content = content.sub("var openRouterModelsCache []OpenRouterModel", "var openRouterModelsCache []OpenRouterModel\nvar openRouterCacheMu syncMutex")
File.write("openrouter.go", content)
