all:
	mkdir bin/; \
	mkdir pkg/; \
	export GOPATH=~/golang/sharenotes; \
	cp -r html/*.html bin/; \
    go build -i -o bin/shareNotes src/shareNotes.go \

clean:
	rm -rf bin/*; rm -rf pkg/*
