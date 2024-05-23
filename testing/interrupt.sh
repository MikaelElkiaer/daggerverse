#!/bin/sh

set -e

for x in HUP INT QUIT ABRT KILL ALRM TERM; do
	trap "echo \"Caught $x\"" "$x"
done

echo -n "Sleeping"
for _ in $(seq 1 10); do
	echo -n "."
	sleep 1
done

echo "Done"
