TAG = $(shell git describe --tags)
NAME = bedrocktool-${TAG}
SRCS = $(wildcard *.go)

GC = go build -ldflags "-s -w -X main.version=${TAG}"

# check if packs are supported
HAVE_PACKS = false
ifeq ($(shell head -c 7 cmd/bedrocktool/utils/resourcepack-ace.go.ignore),package)
HAVE_PACKS = true
endif

$(info pack support: ${HAVE_PACKS})
ifeq ($(HAVE_PACKS),true)
GC += -overlay overlay.json
endif

BUILDS=\
	windows_386.exe\
	windows_amd64.exe\
	windows_arm64.exe\
	windows_arm.exe\
	darwin_amd64\
	darwin_arm64\
	linux_386\
	linux_amd64\
	linux_arm64\
	linux_arm


DISTS=$(BUILDS:%=dist/$(NAME)_%)

all: $(DISTS)

.PHONY: all clean

$(DISTS): OS = $(word 2,$(subst _, ,$@))
$(DISTS): ARCH = $(word 1,$(subst ., ,$(word 3,$(subst _, ,$@))))

dist:
	mkdir -p dist

$(DISTS): dist $(SRCS)
	@echo "building: $@"
	GOOS=$(OS) GOARCH=$(ARCH) $(GC) -o $@ ./cmd/bedrocktool

clean:
	rm -r dist