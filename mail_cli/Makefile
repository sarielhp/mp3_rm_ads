.PHONY: all clean install

all: install

GO_FILES := $(shell find . -name "*.go")

mail_cli: $(GO_FILES) go.mod go.sum
	go build -o mail_cli .

install: mail_cli
	mkdir -p ~/misc/bin_local
	rm -f ~/misc/bin_local/mail_cli
	cp mail_cli ~/misc/bin_local/

clean:
	rm -f mail_cli
