#!/bin/sh
# Wrapper script to perform GPG signature validation on git commit SHAs and
# annotated tags.
#
# We capture stderr to stdout, so we can have the output in the logs. Also,
# we ignore error codes that are emitted if signature verification failed.
#
if test "$1" = ""; then
	echo "Wrong usage of git-verify-wrapper.sh" >&2
	exit 1
fi

REVISION="$1"
TYPE=

# Figure out we have an annotated tag or a commit SHA
if git describe --exact-match "${REVISION}" >/dev/null 2>&1; then
	IFS=''
	TYPE=tag
	OUTPUT=$(git verify-tag "$REVISION" 2>&1)
	RET=$?
else
	IFS=''
	TYPE=commit
	OUTPUT=$(git verify-commit "$REVISION" 2>&1)
	RET=$?
fi

case "$RET" in
0)
	echo "$OUTPUT"
	;;
1)
	# git verify-tag emits error messages if no signature is found on tag,
	# which we don't want in the output.
	if test "$TYPE" = "tag" -a "${OUTPUT%%:*}" = "error"; then
		OUTPUT=""
	fi
	echo "$OUTPUT"
	RET=0
	;;
*)
	echo "$OUTPUT" >&2
	;;
esac
exit $RET
