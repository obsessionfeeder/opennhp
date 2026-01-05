#!/bin/bash
# Build native NHP Agent SDKs for Flutter plugin
#
# Prerequisites:
#   - Go 1.21+
#   - gomobile: go install golang.org/x/mobile/cmd/gomobile@latest
#   - gomobile init
#   - For iOS: Xcode with iOS SDK
#   - For Android: Android SDK with NDK
#
# Usage:
#   ./scripts/build_native.sh          # Build both platforms
#   ./scripts/build_native.sh ios      # Build iOS only
#   ./scripts/build_native.sh android  # Build Android only

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(dirname "$SCRIPT_DIR")"
OPENNHP_ROOT="$(cd "$PLUGIN_DIR/../../.." && pwd)"

# Directories
IOS_SDK_SRC="$OPENNHP_ROOT/endpoints/agent/iossdk"
ANDROID_SDK_SRC="$OPENNHP_ROOT/endpoints/agent/main"
IOS_OUTPUT_DIR="$PLUGIN_DIR/ios/Frameworks"
ANDROID_OUTPUT_DIR="$PLUGIN_DIR/android/src/main/jniLibs"

echo "=== OpenNHP Flutter Plugin Native SDK Builder ==="
echo "OpenNHP root: $OPENNHP_ROOT"
echo "Plugin dir: $PLUGIN_DIR"
echo ""

# Check prerequisites
check_prerequisites() {
    echo "Checking prerequisites..."

    if ! command -v go &> /dev/null; then
        echo "ERROR: Go is not installed. Install from https://go.dev/dl/"
        exit 1
    fi
    echo "✓ Go $(go version | cut -d' ' -f3)"

    if ! command -v gomobile &> /dev/null; then
        echo "gomobile not found. Installing..."
        go install golang.org/x/mobile/cmd/gomobile@latest
        gomobile init
    fi
    echo "✓ gomobile installed"

    echo ""
}

build_ios() {
    echo "=== Building iOS SDK ==="

    if [[ "$(uname)" != "Darwin" ]]; then
        echo "WARNING: iOS SDK can only be built on macOS"
        return 1
    fi

    # Check for Xcode
    if ! xcode-select -p &> /dev/null; then
        echo "ERROR: Xcode command line tools not installed"
        echo "Run: xcode-select --install"
        exit 1
    fi

    mkdir -p "$IOS_OUTPUT_DIR"

    echo "Building NhpAgent.xcframework..."
    cd "$IOS_SDK_SRC"

    # Build xcframework
    gomobile bind \
        -target=ios \
        -o "$IOS_OUTPUT_DIR/NhpAgent.xcframework" \
        .

    if [[ -d "$IOS_OUTPUT_DIR/NhpAgent.xcframework" ]]; then
        echo "✓ iOS SDK built: $IOS_OUTPUT_DIR/NhpAgent.xcframework"
        ls -la "$IOS_OUTPUT_DIR/NhpAgent.xcframework/"
    else
        echo "ERROR: iOS SDK build failed"
        exit 1
    fi

    echo ""
}

build_android() {
    echo "=== Building Android SDK ==="

    # Check for Android SDK
    if [[ -z "$ANDROID_HOME" ]] && [[ -z "$ANDROID_SDK_ROOT" ]]; then
        echo "WARNING: ANDROID_HOME or ANDROID_SDK_ROOT not set"
        echo "Android SDK may not be found"
    fi

    mkdir -p "$ANDROID_OUTPUT_DIR"

    echo "Building nhpagent.aar..."
    cd "$ANDROID_SDK_SRC"

    # Build AAR (includes all architectures)
    CGO_ENABLED=1 gomobile bind \
        -target=android \
        -androidapi 21 \
        -o "$PLUGIN_DIR/android/src/main/libs/nhpagent.aar" \
        .

    if [[ -f "$PLUGIN_DIR/android/src/main/libs/nhpagent.aar" ]]; then
        echo "✓ Android SDK built: $PLUGIN_DIR/android/src/main/libs/nhpagent.aar"
        ls -la "$PLUGIN_DIR/android/src/main/libs/"
    else
        echo "ERROR: Android SDK build failed"
        exit 1
    fi

    echo ""
}

# Main
check_prerequisites

case "${1:-all}" in
    ios)
        build_ios
        ;;
    android)
        build_android
        ;;
    all)
        build_ios || echo "iOS build skipped"
        build_android || echo "Android build skipped"
        ;;
    *)
        echo "Usage: $0 [ios|android|all]"
        exit 1
        ;;
esac

echo "=== Build Complete ==="
echo ""
echo "Next steps:"
echo "1. For iOS: The xcframework is now in ios/Frameworks/"
echo "2. For Android: The AAR is now in android/src/main/libs/"
echo "3. Uncomment the native library imports in the plugin code"
echo "4. Run 'flutter pub get' in your Flutter project"
