.PHONY: all check lint test build install format tidy vet staticcheck map version bump commit push ci checkpoint clean audit symbols template suggest-split

all: check

check:
	@ruby ./tools/check.rb

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

build:
	@go build -o abs .

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
	@go install .
	@if [ -d "$$HOME/bin" ]; then cp abs "$$HOME/bin/abs"; echo "Installed to $$HOME/bin/abs and $$(go env GOPATH)/bin/abs"; else echo "Installed to $$(go env GOPATH)/bin/abs"; fi

ci: check

checkpoint:
	@./tools/checkpoint.sh

clean:
	@rm -f abs
	@echo "Cleaned."