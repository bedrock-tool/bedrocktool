GC = go build -ldflags "-s -w"
NAME = resourcedumper
SRCS = $(wildcard *.go)

all: ${NAME}.exe ${NAME}-linux ${NAME}-mac


$(NAME).exe: $(SRCS)
	GOOS=windows $(GC) -o $@

$(NAME)-linux: $(SRCS)
	GOOS=linux $(GC) -o $@

$(NAME)-mac: $(SRCS)
	GOOS=darwin $(GC) -o $@


.PHONY: clean windows linux mac

windows: $(NAME).exe
linux: $(NAME)-linux
mac: $(NAME)-mac

clean:
	rm $(NAME).exe $(NAME)-linux $(NAME)-mac