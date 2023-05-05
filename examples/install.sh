#!/bin/bash

# USAGE:
#   ./install.sh [ chisel-cut-OPTIONS ] <slices..>
# Installs the slices, using
#   chisel cut "$@"
# Generates a status file similar to /var/lib/dpkg/status at the CHISEL_DPKG_STATUS_FILE path
# env vars:
#   CHISEL_BIN                (optional) the path of chisel binary
#   CHISEL_CACHE_DIR          (optional) the path which chisel will use to store it's cache
#   CHISEL_DPKG_STATUS_FILE   (required) the path where the dpkg status file should be generated

set -e

CHISEL_BIN="${CHISEL_BIN:-"./chisel"}"
CHISEL_CACHE_DIR="${CHISEL_CACHE_DIR:-".chisel-cache"}"
CHISEL_DPKG_STATUS_FILE="${CHISEL_DPKG_STATUS_FILE}"

_print_error() {
	echo "Error: $@" >> /dev/stderr
}

_cleanup_cache() {
	if [ -d "${CHISEL_CACHE_DIR}" ]; then
		rm -rf "${CHISEL_CACHE_DIR}"
	fi
}

_install_slices() {
	XDG_CACHE_HOME="${CHISEL_CACHE_DIR}" ${CHISEL_BIN} cut "$@"
}

_prepare_dpkg_status() {
	local dir="${CHISEL_CACHE_DIR}/chisel/sha256"
	if [ ! -d "$dir" ]; then
		_print_error "could not find the chisel cache at ${dir}"
		exit 1
	fi

	if [ -f "$CHISEL_DPKG_STATUS_FILE" ]; then
		rm -f "$CHISEL_DPKG_STATUS_FILE"
	fi

	for f in "$dir"/*; do
		local is_deb="$(file "$f" | grep "Debian binary package" | cat)"
		if [ -z "$is_deb" ]; then
			continue
		fi
		dpkg-deb -f "$f" >> "${CHISEL_DPKG_STATUS_FILE}"
		echo "" >> "${CHISEL_DPKG_STATUS_FILE}"
	done
}

if [ -z "$CHISEL_DPKG_STATUS_FILE" ] ; then
	_print_error "please specify the desired path of dpkg status in CHISEL_DPKG_STATUS_FILE"
	exit 1
fi

_cleanup_cache
_install_slices "$@"
_prepare_dpkg_status
_cleanup_cache
