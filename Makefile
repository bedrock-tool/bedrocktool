GC = go build -ldflags "-s -w"
TAG = $(shell git describe --tags)

NAME = bedrocktool-${TAG}
SRCS = $(wildcard *.go)



HAVE_PACKS = false
ifeq ($(shell head -c 7 resourcepack-ace.go),package)
HAVE_PACKS = true
endif

$(info pack support: ${HAVE_PACKS})

ifneq ($(HAVE_PACKS),true)
GC += -overlay overlay.json
endif


BUILDS=\
	windows_amd64.exe\
	windows_arm64.exe\
	windows_arm.exe\
	darwin_amd64\
	darwin_arm64\
	linux_amd64\
	linux_arm64\
	linux_arm\
	linux_mips\
	linux_mips64\
	linux_mips64le\
	linux_ppc64\
	linux_ppc64le\
	linux_riscv64\
	linux_s390x


DISTS=$(BUILDS:%=$(NAME)_%)

all: $(DISTS)

.PHONY: all clean $(DISTS)

$(DISTS): OS = $(word 2,$(subst _, ,$@))
$(DISTS): ARCH = $(word 1,$(subst ., ,$(word 3,$(subst _, ,$@))))

$(DISTS): $(SRCS)
	@echo "building: $@"
	GOOS=$(OS) GOARCH=$(ARCH) $(GC) -o $@

clean:
	rm $(NAME).exe $(NAME)-linux $(NAME)-mac