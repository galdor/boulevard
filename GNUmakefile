BUILD_ID = $(shell git describe --tags HEAD)
ifndef BUILD_ID
$(error Cannot generate build id from Git repository)
endif

BIN_DIR = $(CURDIR)/bin

GO_PKGS = \
  go.n16f.net/boulevard/cmd/boulevard \
  go.n16f.net/boulevard/cmd/boulevard-cli \
  go.n16f.net/boulevard/cmd/fastcgi

define go_make1
CGO_ENABLED=0 \
go build -o $(BIN_DIR) \
  -ldflags="-X 'main.buildId=$(BUILD_ID)'" \
  $1
endef

define go_make
$(foreach dir,$(GO_PKGS),$(call go_make1,$(dir))
)
endef

DOC_DIR = $(CURDIR)/doc
DOC_MANUAL = $(DOC_DIR)/manual/manual.texi
DOC_MANUAL_HTML = $(DOC_DIR)/manual/html/
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

clean:
	$(RM) $(wildcard bin/*)

FORCE:

.PHONY: all build check vet test doc clean
