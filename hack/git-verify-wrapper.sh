#!/bin/sh
# Simple wrapper around gpg verify-commit so we can capture stderr output
COMMITS="$*"
IFS=''
OUTPUT=$(git verify-commit $COMMITS 2>&1)
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
