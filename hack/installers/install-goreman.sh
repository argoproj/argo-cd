#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

BASHRC="$HOME/.bashrc"
GOPATH_LINE='export GOPATH=$HOME/go'
GOROOT_PATH_LINE='export PATH=$PATH:$GOROOT/bin:$GOPATH/bin'
GOPATH_PATH_LINE='export PATH=$GOPATH/bin:$PATH'

# Install goreman
echo "Installing goreman..."
go install github.com/mattn/goreman@latest

# Function to check if a line exists in ~/.bashrc
check_and_add() {
    local line="$1"
    local file="$2"
    if ! grep -Fxq "$line" "$file"; then
        echo "$line" >> "$file"
        echo "Added: $line"
    else
        echo "Already present: $line"
    fi
}

# Ensure required lines are in ~/.bashrc
check_and_add "$GOPATH_LINE" "$BASHRC"
check_and_add "$GOROOT_PATH_LINE" "$BASHRC"
check_and_add "$GOPATH_PATH_LINE" "$BASHRC"

# Reload ~/.bashrc
. "$BASHRC"

# Install goreman
if ! command -v goreman &> /dev/null; then
    echo "Installing goreman..."
    go install github.com/mattn/goreman@latest
    echo "Goreman installed successfully."
else
    echo "Goreman is already installed."
fi
