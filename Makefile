GC = go build -ldflags "-s -w"
NAME = bedrocktool
SRCS = $(wildcard *.go)

all: windows linux


$(NAME).exe: $(SRCS)
	GOOS=windows $(GC) -o $@
	upx -9 $@ # ignore if fails

$(NAME)-linux: $(SRCS)
	GOOS=linux $(GC) -o $@
	upx -9 $@ # ignore if fails

$(NAME)-mac: $(SRCS) # possibly broken
	GOOS=darwin $(GC) -o $@
	upx -9 $@ # ignore if fails

.PHONY: clean windows linux mac

windows: $(NAME).exe
linux: $(NAME)-linux
mac: $(NAME)-mac

clean:
	rm $(NAME).exe $(NAME)-linux $(NAME)-mac