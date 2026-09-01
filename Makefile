.PHONY: all check lint test race vuln build install format tidy vet staticcheck map version bump commit push ci checkpoint clean audit symbols template suggest-split visual verify-queue review-loop

all: check

verify-queue:
	@ruby ./tools/verify_remote_queue.rb $(ARGS)

visual:
	@ruby ./tools/visual_audit.rb

check:
	@ruby ./tools/check.rb

ci:
	@ruby ./tools/check.rb --full

lint:
	@ruby ./tools/lint.rb

audit:
	@ruby ./tools/audit_lines.rb

symbols:
	@ruby ./tools/outline_symbols.rb $(ARGS)

suggest-split:
	@ruby ./tools/suggest_split.rb $(ARGS)

template:
	@ruby ./tools/generate_config_template.rb

test:
	@go test -timeout 30s ./...

race:
	@go test -race -timeout 30s ./...

vuln:
	@$$HOME/.go/bin/govulncheck ./...

build:
	@ruby ./tools/build_local.rb

format:
	@./tools/format.sh

tidy:
	@go mod tidy

vet:
	@go vet ./...

staticcheck:
	@staticcheck -checks '-SA2001' ./...

map:
	@./tools/map.sh

version:
	@./tools/version.sh

bump:
	@ruby ./tools/bump-version.rb

commit:
	@ruby ./tools/commit.rb $(ARGS)

push: bump

install: build
	@install -d "$$HOME/bin"
	@install -m 755 abs "$$HOME/bin/abs"
	@echo "Installed to $$HOME/bin/abs"

ci: check

checkpoint:
	@./tools/checkpoint.sh

review-loop:
	@ruby ./tools/review_loop.rb $(ARGS)

clean:
	@rm -f abs
	@echo "Cleaned."