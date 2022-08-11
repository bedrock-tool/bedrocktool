GC = go build -ldflags "-s -w"
TAG := $(shell git describe --tags)

NAME = bedrocktool_${TAG}
SRCS = $(wildcard *.go)

all: windows linux

.PHONY: all clean windows linux mac

windows: $(NAME).exe
linux: $(NAME)-linux
mac: $(NAME)-mac


$(NAME).exe: $(SRCS)
	GOOS=windows $(GC) -o $@

$(NAME)-linux: $(SRCS)
	GOOS=linux $(GC) -o $@

$(NAME)-mac: $(SRCS) # possibly broken
	GOOS=darwin $(GC) -o $@




clean:
	rm $(NAME).exe $(NAME)-linux $(NAME)-mac