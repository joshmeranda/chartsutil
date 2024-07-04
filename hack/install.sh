#!/usr/bin/env bash

VERSION_FILE=scripts/version
BINDIR=${BINDIR:-./bin}

if [ -z "$VERSION" ]; then
	if [ -z "$CHARTS_BUILD_SCRIPT_VERSION" ]; then
		echo "CHARTS_BUILD_SCRIPT_VERSION is not set, checking '$VERSION_FILE'"
		source $VERSION_FILE
	fi

	echo "Using charts-build-script version ${CHARTS_BUILD_SCRIPT_VERSION:?}"

	case "$CHARTS_BUILD_SCRIPT_VERSION" in
		v0.9.2 )
			VERSION=v0.0.0
			;;
		* )
			echo "Unsupported version $CHARTS_BUILD_SCRIPT_VERSION"
			exit 1
			;;
	esac
fi

echo "Installing chartsutils into ${BINDIR}"
GOBIN="${BINDIR}" go install "github.com/joshmeranda/chartsutil@${VERSION}"

${BINDIR}/chartsutil --version