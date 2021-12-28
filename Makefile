all: _other_ bin/undisk bin/os9defs
install: all

_other_:
	: There should be no command dependencies outside /usr/bin and /bin.
	: But if you have your unix C compiler in a strange place, fix this.
	cd other && PATH='/usr/bin:/bin' make all

bin/undisk:
	go build -x -o bin/undisk v2/undisk/*.go
bin/os9defs:
	go build -x -o bin/os9defs v2/os9defs/*.go

clean:
	cd other && make clean
	rm -rf bin lib share
