TAG = $(shell git describe --exclude "r-*" --tags)
NAME = bedrock-skin-bot-${TAG}
SRCS = $(wildcard **/*.go)

GC = go build -ldflags "-s -w -X github.com/bedrock-tool/bedrocktool/bedrock-skin-bot/utils.Version=${TAG}"

.PHONY: dists clean updates


bedrock-skin-bot: $(SRCS)
	$(GC) -o $@ .

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
dists: $(DISTS)
$(DISTS): OS = $(word 2,$(subst _, ,$@))
$(DISTS): ARCH = $(word 1,$(subst ., ,$(word 3,$(subst _, ,$@))))
$(DISTS): BUILD = builds/$(OS)-$(ARCH)

dist builds:
	mkdir -p dist builds

$(DISTS): dist builds $(SRCS)
	$(info building: $@)
	GOOS=$(OS) GOARCH=$(ARCH) $(GC) -o $(BUILD) .
	cp $(BUILD) $@


UPDATES=$(BUILDS)
$(UPDATES): OS = $(word 1,$(subst _, ,$@))
$(UPDATES): ARCH = $(word 1,$(subst ., ,$(word 2,$(subst _, ,$@))))
updates: $(UPDATES)

$(UPDATES): $(DISTS)
	go-selfupdate -platform $(OS)-$(ARCH) builds/ $(TAG)

clean:
	rm -r dist builds public