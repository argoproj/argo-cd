#!/bin/sh
set -ex

if test "${TARGET_FILE}" = ""; then
	echo "Need to define \$TARGET_FILE" >&2
	exit 1
fi

CHKSUM_FILE=$(cd "$(dirname "$0")" && pwd)/checksums/${TARGET_FILE}.sha256

cd "${DOWNLOADS}" || (
	echo "Can't change directory to ${DOWNLOAD}" >&2
	exit 1
)

if ! test -f "${TARGET_FILE}"; then
	echo "Archive to be checked (${TARGET_FILE}) does not exist" >&2
	exit 1
fi

if ! grep -q "${TARGET_FILE}" ${CHKSUM_FILE}; then
	echo "No checksum for ${TARGET_FILE} in ${CHKSUM_FILE}" >&2
	exit 1
fi

shasum -a 256 -c ${CHKSUM_FILE}
