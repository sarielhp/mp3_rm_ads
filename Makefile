.PHONY: all check lint test build format tidy vet staticcheck map version bump commit push ci checkpoint clean audit symbols template

all: check

check:
	@./tools/check.sh

lint:
	@./tools/lint.sh

audit:
	@ruby ./tools/audit_lines.rb

symbols:
	@ruby ./tools/outline_symbols.rb $(ARGS)

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
	@./tools/bump-version.sh

commit:
	@./tools/commit.sh $(ARGS)

push: bump

ci: check

checkpoint:
	@./tools/checkpoint.sh

clean:
	@rm -f abs
	@echo "Cleaned."