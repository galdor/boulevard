prefix = /usr/local
bindir = $(DESTDIR)$(prefix)/bin
sharedir = $(DESTDIR)$(prefix)/share
docdir = $(DESTDIR)$(prefix)/share/doc

BUILD_ID = $(shell git describe --tags HEAD)
ifndef BUILD_ID
$(error Cannot generate build id from Git repository)
endif

GO_PKGS = \
  go.n16f.net/boulevard/cmd/boulevard \
  go.n16f.net/boulevard/cmd/boulevard-cli \
  go.n16f.net/boulevard/cmd/fastcgi

DELIVERED_BINARIES = \
  boulevard \
  boulevard-cli \

define go_make1
CGO_ENABLED=0 \
go build -o bin -ldflags="-X 'main.buildId=$(BUILD_ID)'" $1
endef

define go_make
$(foreach dir,$(GO_PKGS),$(call go_make1,$(dir))
)
endef

DOC_DIR = $(CURDIR)/doc
DOC_MANUAL = $(DOC_DIR)/manual/manual.texi
DOC_MANUAL_HTML = $(DOC_DIR)/manual/html
TEXI_FILES = $(wildcard $(DOC_DIR)/*.texi)

all: build

build: FORCE
	$(call go_make)

check: vet

vet:
	go vet $(CURDIR)/...

test:
	go test -race -count 1 -failfast $(CURDIR)/...

doc: $(TEXI_FILES)
	texi2any --html -o $(DOC_MANUAL_HTML) $(DOC_MANUAL)

install:
	@if [ -z "$(DESTDIR)" ]; then echo "DESTDIR not set" >&2; exit 1; fi
	mkdir -p $(bindir)
	cp $(addprefix bin/,$(DELIVERED_BINARIES)) $(bindir)
	mkdir -p $(sharedir)/licenses
	cp LICENSE $(sharedir)/licenses/boulevard
	mkdir -p $(docdir)/boulevard/html
	cp -r $(DOC_MANUAL_HTML)/* $(docdir)/boulevard/html

install-flat:
	@if [ -z "$(DESTDIR)" ]; then echo "DESTDIR not set" >&2; exit 1; fi
	mkdir -p $(DESTDIR)
	cp $(addprefix bin/,$(DELIVERED_BINARIES)) $(DESTDIR)
	cp LICENSE $(DESTDIR)
	mkdir -p $(DESTDIR)/doc/html
	cp -r $(DOC_MANUAL_HTML)/* $(DESTDIR)/doc/html

clean:
	$(RM) $(wildcard bin/*) $(DOC_MANUAL_HTML)/*

FORCE:

.PHONY: all build check vet test doc install install-flat clean
