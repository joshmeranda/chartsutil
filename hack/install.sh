#!/usr/bin/env bash

VERSION_FILE=scripts/version
BINDIR=${BINDIR:-$(realpath ./bin)}

if [ -z "$VERSION" ]; then
	if [ -z "$CHARTS_BUILD_SCRIPT_VERSION" ]; then
		echo "CHARTS_BUILD_SCRIPT_VERSION is not set, checking '$VERSION_FILE'"
		source $VERSION_FILE
	fi

	echo "Using charts-build-script version ${CHARTS_BUILD_SCRIPT_VERSION:?}"

	case "$CHARTS_BUILD_SCRIPT_VERSION" in
		v0.9.2 )
			VERSION=v0.1.0
			;;
		v0.9.3 )
			VERSION=v0.1.1
			;;
		v0.9.4 )
			VERSION=v0.1.2
			;;
		v0.9.5 )
			VERSION=v0.1.8
			;;
		v0.9.6 )
			VERSION=v0.1.9
			;;
		v0.9.7 )
			VERSION=v0.1.10
			;;
		v0.9.8 )
			VERSION=v0.1.11
			;;
		v0.9.9 )
			VERSION=v0.1.12
			;;
		v1.0.0 )
			VERSION=v0.1.14
			;;
		* )
			echo "Unsupported version $CHARTS_BUILD_SCRIPT_VERSION"
			exit 1
			;;
	esac
fi

echo "Installing chartsutils into ${BINDIR}"
git clone --branch "${VERSION}" git@github.com:joshmeranda/chartsutil.git .chartsutil-src &>/dev/null
cd .chartsutil-src
GOBIN="${BINDIR}" go install .
cd ..
rm --recursive --force .chartsutil-src

${BINDIR}/chartsutil --version