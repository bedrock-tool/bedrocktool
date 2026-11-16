#!/bin/sh
# Rebuilds bedrocktool_android.jar from the .java files here.
# Run after editing one, and commit the jar with it.
# Needs a JDK and the Android SDK (ANDROID_HOME) with a platform installed.
set -e
cd "$(dirname "$0")"

sdk="${ANDROID_HOME:-$ANDROID_SDK_ROOT}"
if [ -z "$sdk" ]; then
	echo "set ANDROID_HOME to the Android SDK" >&2
	exit 1
fi
androidjar=$(ls -d "$sdk"/platforms/android-*/android.jar | sort -V | tail -1)

out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
# gogio's d8 step only takes Java 8 class files
javac -source 1.8 -target 1.8 -Xlint:-options -bootclasspath "$androidjar" -d "$out" *.java
jar cf bedrocktool_android.jar -C "$out" .
echo "wrote bedrocktool_android.jar"
