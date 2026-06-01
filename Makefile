BINARY  := dockflux
PREFIX  ?= /usr/local
BINDIR  := $(PREFIX)/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/jconder44/dockflux/cmd.Version=$(VERSION)"

.PHONY: build install uninstall clean

build:
	go build $(LDFLAGS) -o $(BINARY) .

install: build
	install -m 0755 $(BINARY) $(BINDIR)/$(BINARY)
	@echo "Installed $(BINDIR)/$(BINARY)"
	@echo "Next: sudo dockflux service install"

uninstall:
	rm -f $(BINDIR)/$(BINARY)
	@echo "Removed $(BINDIR)/$(BINARY)"

clean:
	rm -f $(BINARY)
