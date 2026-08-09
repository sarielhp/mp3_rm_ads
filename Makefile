.PHONY: all check lint test build format tidy vet staticcheck map version bump commit push ci checkpoint clean

all: check

check:
	@./scripts/check.sh

lint:
	@./scripts/lint.sh

test:
	@go test -timeout 30s ./...

build:
	@go build -o mp3_rm_ads .

format:
	@./scripts/format.sh

tidy:
	@go mod tidy

vet:
	@go vet ./...

staticcheck:
	@staticcheck -checks '-SA2001' ./...

map:
	@./scripts/map.sh

version:
	@./scripts/version.sh

bump:
	@./scripts/bump-version.sh

commit:
	@./scripts/commit.sh $(ARGS)

push: bump

ci: check

checkpoint:
	@./scripts/checkpoint.sh

clean:
	@rm -f mp3_rm_ads
	@echo "Cleaned."