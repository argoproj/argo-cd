#!/bin/sh

# If we're started as PID 1, we should wrap command execution through tini to
# prevent leakage of orphaned processes ("zombies").
if test "$$" = "1"; then
	exec tini -- $@
else
	exec "$@"
fi
