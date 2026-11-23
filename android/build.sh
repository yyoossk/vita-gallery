#!/bin/bash
# Builds VitaGallery-Android.apk from this repository.
# The Go source of the app itself lives in the repository root; this
# folder only holds the Android WebView shell and this script.
#
# Requirements (paths are examples from the author's environment):
# a Go toolchain, a JDK, and the Android SDK build-tools 34.
#
# Pipeline: Go (android/arm64) -> libvitagallery.so, javac + d8 for
# the shell, aapt2 packaging, zipalign, signed with a local sideload
# key (keystore.jks next to this script, created on first run; the
# keystore is NOT part of this repository).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
TC="${ANDROID_TC:-$HOME/workspace/toolchain}"
export JAVA_HOME="${JAVA_HOME:-$TC/android/jdk}"
export PATH="$JAVA_HOME/bin:$TC/go/bin:$PATH"
BT="$TC/android/sdk/build-tools/34.0.0"
ANDROID_JAR="$TC/android/sdk/platforms/android-34/android.jar"
SRC="$(cd "$ROOT/.." && pwd)"
BUILD="$ROOT/build"
OUT="$ROOT/VitaGallery-Android.apk"

STAMP="${VITA_STAMP:-$(TZ=Asia/Tokyo date +%Y%m%d-%H%M)}"
VERSION_CODE=$(( ( $(date +%s) - 1735689600 ) / 60 ))  # minutes since 2025-01-01 UTC
echo "== build $STAMP (versionCode $VERSION_CODE)"

rm -rf "$BUILD"
mkdir -p "$BUILD/lib/arm64-v8a" "$BUILD/classes" "$BUILD/dex"

echo "-- Go server (android/arm64)"
(cd "$SRC" && GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build \
    -ldflags "-X main.buildStamp=$STAMP" -o "$BUILD/lib/arm64-v8a/libvitagallery.so" .)

echo "-- Java shell"
javac -source 11 -target 11 -cp "$ANDROID_JAR" -d "$BUILD/classes" \
    $(find "$ROOT/app/src" -name '*.java')
"$BT/d8" --lib "$ANDROID_JAR" --output "$BUILD/dex" \
    $(find "$BUILD/classes" -name '*.class')

echo "-- resources + package"
"$BT/aapt2" compile --dir "$ROOT/app/res" -o "$BUILD/res.zip"
"$BT/aapt2" link -o "$BUILD/app-raw.apk" \
    -I "$ANDROID_JAR" \
    --manifest "$ROOT/app/AndroidManifest.xml" \
    --version-code "$VERSION_CODE" \
    --version-name "$STAMP" \
    "$BUILD/res.zip"
(cd "$BUILD/dex" && zip -q ../app-raw.apk classes.dex)
(cd "$BUILD" && zip -q -r app-raw.apk lib)
"$BT/zipalign" -p -f 4 "$BUILD/app-raw.apk" "$BUILD/app-aligned.apk"

echo "-- sign"
KS="$ROOT/keystore.jks"
if [ ! -f "$KS" ]; then
    keytool -genkeypair -keystore "$KS" -alias vitagallery -keyalg RSA -keysize 2048 \
        -validity 10000 -storepass vitagallery-android -keypass vitagallery-android \
        -dname "CN=Vita Gallery, O=yyoossk" >/dev/null 2>&1
fi
"$BT/apksigner" sign --ks "$KS" --ks-pass pass:vitagallery-android \
    --key-pass pass:vitagallery-android --out "$OUT" "$BUILD/app-aligned.apk"
"$BT/apksigner" verify "$OUT"
ls -la "$OUT"
