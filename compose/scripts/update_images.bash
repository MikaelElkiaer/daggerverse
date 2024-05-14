#!/bin/bash

set -eo pipefail

DST_FILE="docker-compose.yaml"
SRC_FILE="docker-compose.yaml"

optspec="c:d:s:-:"
while getopts "$optspec" optchar; do
	case "${optchar}" in
	-)
		case "${OPTARG}" in
		destination)
			DST_FILE="${!OPTIND}"
			OPTIND=$(($OPTIND + 1))
			;;
		destination=*)
			DST_FILE=${OPTARG#*=}
			;;
		source)
			SRC_FILE="${!OPTIND}"
			OPTIND=$(($OPTIND + 1))
			;;
		source=*)
			SRC_FILE=${OPTARG#*=}
			;;
		*)
			if [ "$OPTERR" = 1 ] && [ "${optspec:0:1}" != ":" ]; then
				echo "Unknown option --${OPTARG}" >&2
			fi
			;;
		esac
		;;
	d)
		DST_FILE="${OPTARG}"
		;;
	s)
		SRC_FILE="${OPTARG}"
		;;
	*)
		if [ "$OPTERR" != 1 ] || [ "${optspec:0:1}" = ":" ]; then
			echo "Non-option argument: '-${OPTARG}'" >&2
		fi
		;;
	esac
done

SERVICES="$(yq '. | with_entries(select(.key=="services")) | .services as $s | .services = $s | map_values(. | to_entries | from_entries | map_values(pick(["image"])))' "$SRC_FILE")"

touch "$DST_FILE"
export s
for s in $(yq '.services | keys | .[]' <<<"$SERVICES"); do
	echo "-- Updating service: $s"
	read -r REG OWN REP TAG <<<"$(yq '.services[env(s)].image' <<<"$SERVICES" | sed 's,^\([^/]*\)/\([^/]*\)/\([^:]*\):\(.*\)$,\1 \2 \3 \4,')"
	[ "$TAG" == '' ] && {
		echo "No tag, skipping..."
		continue
	}
	echo "Current version: ${V}${TAG}"
	TAGS="$(skopeo list-tags "docker://$REG/$OWN/$REP" | yq -p json '.Tags[]')"
	PRE=""
	V=""
	read -r TAG V <<<"$(sed 's,^\([vV]\)\?\(.*\)$,\2 \1,' <<<"$TAG")"
	read -r TAG PRE <<<"$(sed 's,^\([^-]*\)\(-.*\)\?$,\1 \2,' <<<"$TAG")"
	FILTERED="$(echo "$TAGS" | xargs semver --range "^${TAG}${PRE}" || echo '')"
	LATEST="${V}$(echo "$FILTERED" | tail -1 || echo '')${PRE}"
	[ "$LATEST" == '' ] && {
		echo "No latest tag, skipping..."
		continue
	}
	echo "Latest version: $LATEST"
	[ "$LATEST" = "${V}${TAG}${PRE}" ] && {
		echo "Up-to-date, skipping..."
		continue
	}
	export REG OWN REP TAG LATEST
	yq --inplace '.services[env(s)].image = env(REG)+"/"+env(OWN)+"/"+env(REP)+":"+env(LATEST)' "$DST_FILE"
done
