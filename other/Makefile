all: _lwtools_ _cmoc_
install: all

_lwtools_:
	P=$$(cd ..; pwd) ; cd lwtools && make PREFIX=$$P all install
	:
	: Make lwasm verbose so it prints all its arguments to stderr.
	cd ../bin && mv lwasm lwasm-jerbo-orig
	(echo '#!/bin/sh -x'; echo ' "$$0-jerbo-orig" "$$@" ') > ../bin/lwasm
	chmod +x ../bin/lwasm
	:
	: Make lwlink verbose so it prints all its arguments to stderr.
	cd ../bin && mv lwlink lwlink-jerbo-orig
	(echo '#!/bin/sh -x'; echo ' "$$0-jerbo-orig" "$$@" ') > ../bin/lwlink
	chmod +x ../bin/lwlink

_cmoc_:
	P=$$(cd ..; pwd) ; cd cmoc && PATH="$$P/bin:$$PATH" ./configure --prefix=$$P
	P=$$(cd ..; pwd) ; cd cmoc && PATH="$$P/bin:$$PATH" make all install
	:
	: Make cmoc verbose so it prints all its arguments to stderr.
	cd ../bin && mv cmoc cmoc-jerbo-orig
	(echo '#!/bin/sh -x'; echo ' "$$0-jerbo-orig" "$$@" ') > ../bin/cmoc
	chmod +x ../bin/cmoc

clean:
	cd cmoc && make clean
	cd lwtools && make clean
