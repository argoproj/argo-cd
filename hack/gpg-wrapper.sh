#!/bin/sh
# Simple wrapper around gpg to prevent exit code != 0
ARGS=$*
OUTPUT=$(gpg $ARGS 2>&1)
IFS=''
RET=$?
case "$RET" in
0)
	echo $OUTPUT
	;;
1)
	echo $OUTPUT
	RET=0
	;;
*)
	echo $OUTPUT >&2
	;;
esac
exit $RET
