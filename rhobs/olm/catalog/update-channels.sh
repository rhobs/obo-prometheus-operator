#!/usr/bin/env bash
set -e -u -o pipefail

#usage $0 channel1[,channel2,...] bundle

to_upper() {
	echo "$@" | tr '[:lower:]' '[:upper:]'
}

err() {
	echo "ERROR: $*"
}

update_channel() {
	local index_file="$1"
	shift
	local channel="$1"
	shift
	local bundle="$1"
	shift

	echo "updating index-file: $index_file | channel: $channel | bundle: $bundle"
	local marker
	marker="### $(to_upper "$channel")_CHANNEL_MARKER ###"

	if ! grep -q "$marker" "$index_file"; then
		err "No marker '$marker' found in $index_file"
		return 1
	fi

	# find the entry before that
	local previous_entry
	previous_entry=$(grep "$marker" -B2 "$index_file" | grep 'name:' | cut -f2 -d: | tr -d ' ')
	echo " -> found previous entry: $previous_entry"

	#
	### handle first entry and first entry is when the previous entry
	# is the same as the channel passed based on the yaml index file
	# name: stable
	# entries:
	# ### STABLE_CHANNEL_MARKER ###
	if [[ "$previous_entry" == "$channel" ]]; then
		echo " -> adding first entry to $bundle to $channel channel"
		sed -e \
			"s|^\($marker\)|  - name: $bundle\n\1|" \
			-i "$index_file"
		return 0
	fi

	echo " -> adding $bundle replaces $previous_entry to $channel channel"
	sed -e \
		"s|^\($marker\)|  - name: $bundle\n    replaces: $previous_entry\n\1|" \
		-i "$index_file"

}

main() {
	cd "$(git rev-parse --show-toplevel)"
	local index_file="$1"
	shift
	local channels="$1"
	shift
	local bundle="$1"
	shift

	echo "index-file: $index_file | channels: $channels | bundle: $bundle"

	# convert comma seperated list to an array
	local -a channel_list
	readarray -t -d, channel_list <<<"$channels,"
	# remove the last one to get rid of the trailing \n
	channel_list=("${channel_list[@]::${#channel_list[@]}-1}")

	for ch in "${channel_list[@]}"; do
		update_channel "$index_file" "$ch" "$bundle"
	done

	return $?
}

main "$@"
