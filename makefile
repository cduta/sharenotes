all:
	export GOPATH=~/golang/ShareNotes; \
	cp -r html/*.html bin/; \
    go build -i -o bin/shareNotes src/shareNotes.go \

clean:
	rm -rf bin/*; rm -rf pkg/*
