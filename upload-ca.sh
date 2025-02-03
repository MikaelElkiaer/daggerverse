#!/usr/bin/env sh

set -e

CERT_PATH="$(mktemp)"
trap 'rm --force "$CERT_PATH"' INT QUIT TERM EXIT

cat "${1:--}" >"$CERT_PATH"

if ! [ -s "$CERT_PATH" ]; then
	echo "Empty file or stdin"
	exit 1
fi

ID="$(docker ps --quiet --filter name=dagger-engine)"
docker exec "$ID" rm -f /usr/local/share/ca-certificates/custom_root.crt
docker cp "$CERT_PATH" "$ID:/usr/local/share/ca-certificates/custom_root.crt"
docker exec "$ID" update-ca-certificates
docker restart "$ID"
