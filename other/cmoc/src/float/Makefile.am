# $Id: Makefile.am,v 1.9 2020/08/05 00:18:40 sarrazip Exp $
#
# ecb = Extended Color Basic.
# usim = usim 6809 simulator.

cmoclibdir = $(pkgdatadir)/lib
cmoclib_DATA = \
	libcmoc-float-ecb.a

ASM = \
	addFPA0FPA1.asm \
	addIntSingle.asm \
	addSignedDWordSingle.asm \
	addSingleInt.asm \
	addSingleSignedDWord.asm \
	addSingleSingle.asm \
	addSingleUnsignedDWord.asm \
	addSingleUnsignedInt.asm \
	addUnsignedDWordSingle.asm \
	addUnsignedIntSingle.asm \
	binOpSingleSingle.asm \
	cmpSignedIntSingle.asm \
	cmpSingleSignedInt.asm \
	cmpSingleSingle.asm \
	cmpSingleUnsignedInt.asm \
	cmpUnsignedIntSingle.asm \
	copySingle.asm \
	decrementSingle.asm \
	divByZeroSingle.asm \
	divIntSingle.asm \
	divSignedDWordSingle.asm \
	divSingleInt.asm \
	divSingleSignedDWord.asm \
	divSingleSingle.asm \
	divSingleUnsignedDWord.asm \
	divSingleUnsignedInt.asm \
	divUnsignedDWordSingle.asm \
	divUnsignedIntSingle.asm \
	ftoa.asm \
	incrementSingle.asm \
	initDWordFromSingle.asm \
	initSignedByteFromSingle.asm \
	initSignedWordFromSingle.asm \
	initSingleFromDWord.asm \
	initSingleFromSignedWord.asm \
	initSingleFromUnsignedWord.asm \
	initUnsignedByteFromSingle.asm \
	initUnsignedWordFromSingle.asm \
	isDWordZeroSpecial.asm \
	isSingleZero.asm \
	loadSignedDWordInFPA0.asm \
	loadUnsignedDInFPA0.asm \
	loadUnsignedDWordInFPA0.asm \
	mulIntSingle.asm \
	mulSignedDWordSingle.asm \
	mulSingleInt.asm \
	mulSingleSignedDWord.asm \
	mulSingleSingle.asm \
	mulSingleUnsignedDWord.asm \
	mulSingleUnsignedInt.asm \
	mulUnsignedDWordSingle.asm \
	mulUnsignedIntSingle.asm \
	negateSingle.asm \
	strtof.asm \
	subIntSingle.asm \
	subSignedDWordSingle.asm \
	subSingle_common.asm \
	subSingleInt.asm \
	subSingleSignedDWord.asm \
	subSingleSingle.asm \
	subSingleUnsignedDWord.asm \
	subSingleUnsignedInt.asm \
	subUnsignedDWordSingle.asm \
	subUnsignedIntSingle.asm \
	unpackXToFPA0AndDiv.asm \
	unpackXtoFPA0AndPrep.asm \
	unpackXToFPA1AndMul.asm

# Also part of 'float' library, but only under Color Basic:
C_COCO_BASIC = \
	atoff.c

ECB_OBJ  = $(ASM:asm=ecb_o) $(C_COCO_BASIC:.c=.ecb_o)

USIM_OBJ =

libcmoc-float-ecb.a: $(ECB_OBJ)
	$(LWAR) -c $@ $(ECB_OBJ)

libcmoc-float-usim.a: $(USIM_OBJ)
	$(LWAR) -c $@ $(USIM_OBJ)

.asm.ecb_o:
	$(LWASM) -fobj -D_COCO_BASIC_ --output=$@ $<

.asm.usim_o:
	$(LWASM) -fobj -DUSIM --output=$@ $<

CMOC_LIB_FLAGS = -O2 -DNDEBUG -nostdinc -I $(srcdir)/../stdlib -I $(srcdir)

.c.ecb_o:
	../cmoc -c $(CMOC_LIB_FLAGS) -o $@ $<

.c.usim_o:
	../cmoc -c $(CMOC_LIB_FLAGS) -o $@ $<

# Checks if some files contain non-breakable spaces (character 160).
# The statement in the 'continue' clause resets the line counter ($.)
# at the end of each processed file.
#
check:
	$(PERL) -e 'while (<>) { if (/\xA0/) { print "Non-breakable space at $$ARGV:$$.\n"; exit 1 } } continue { close ARGV if eof }' \
		$(ASM) $(C_COCO_BASIC) Makefile.am

CLEANFILES = $(cmoclib_DATA) \
	$(ECB_OBJ) $(ECB_OBJ:.ecb_o=.s) $(ECB_OBJ:.ecb_o=.lst) \
	$(USIM_OBJ) $(USIM_OBJ:.usim_o=.s) $(USIM_OBJ:.usim_o=.lst)

EXTRA_DIST = float.inc $(ASM) $(C_COCO_BASIC)
