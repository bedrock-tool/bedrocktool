GC = go build -ldflags "-s -w"
TAG = $(shell git describe --tags)

NAME = bedrocktool_${TAG}
SRCS = $(wildcard *.go)



HAVE_PACKS = false
ifeq ($(shell head -c 7 resourcepack-ace.go),package)
HAVE_PACKS = true
endif

$(info pack support: ${HAVE_PACKS})

ifneq ($(HAVE_PACKS),true)
GC += -overlay overlay.json
endif



all: windows linux

.PHONY: all clean windows linux mac

windows: $(NAME).exe
linux: $(NAME)-linux
mac: $(NAME)-mac


$(NAME).exe: $(SRCS)
	echo TAG: ${TAG}
	GOOS=windows $(GC) -o $@

$(NAME)-linux: $(SRCS)
	GOOS=linux $(GC) -o $@

$(NAME)-mac: $(SRCS)
	GOOS=darwin $(GC) -o $@


clean:
	rm $(NAME).exe $(NAME)-linux $(NAME)-mac