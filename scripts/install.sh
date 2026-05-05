#!/usr/bin/env sh
# MegaCLI installer for macOS and Linux
# Usage: curl -sSf https://raw.githubusercontent.com/rorikonn/MegaCLI/master/scripts/install.sh | sh
set -e

REPO="rorikonn/MegaCLI"
INSTALL_DIR="$HOME/.megacli/bin"
BINARY_NAME="megacli"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)  OS="Linux" ;;
        darwin) OS="Darwin" ;;
        *)      echo "Error: Unsupported OS: $OS"; exit 1 ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="x86_64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *)             echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
    esac
}

# Get the latest release version from GitHub
get_latest_version() {
    VERSION=$(curl -sSf "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Error: Failed to get latest version"
        exit 1
    fi
}

# Download and install
install() {
    ARCHIVE_NAME="megacli_${VERSION}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/v${VERSION}/${ARCHIVE_NAME}"

    echo "Installing MegaCLI v${VERSION} (${OS}/${ARCH})..."
    echo "  -> $DOWNLOAD_URL"

    # Create install directory
    mkdir -p "$INSTALL_DIR"

    # Download and extract
    TMP_DIR=$(mktemp -d)
    curl -sSfL "$DOWNLOAD_URL" -o "$TMP_DIR/megacli.tar.gz"
    tar xzf "$TMP_DIR/megacli.tar.gz" -C "$TMP_DIR"

    # Find and move binary
    find "$TMP_DIR" -name "$BINARY_NAME" -type f -exec cp {} "$INSTALL_DIR/$BINARY_NAME" \;
    chmod +x "$INSTALL_DIR/$BINARY_NAME"

    # Cleanup
    rm -rf "$TMP_DIR"
}

# Add to PATH in shell config
configure_path() {
    SHELL_NAME=$(basename "$SHELL")
    PROFILE=""

    case "$SHELL_NAME" in
        bash)
            if [ -f "$HOME/.bashrc" ]; then
                PROFILE="$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                PROFILE="$HOME/.bash_profile"
            fi
            ;;
        zsh)  PROFILE="$HOME/.zshrc" ;;
        fish) PROFILE="$HOME/.config/fish/config.fish" ;;
    esac

    if [ -n "$PROFILE" ]; then
        EXPORT_LINE="export PATH=\"$INSTALL_DIR:\$PATH\""
        if [ "$SHELL_NAME" = "fish" ]; then
            EXPORT_LINE="set -gx PATH $INSTALL_DIR \$PATH"
        fi

        if ! grep -q "$INSTALL_DIR" "$PROFILE" 2>/dev/null; then
            echo "" >> "$PROFILE"
            echo "# MegaCLI" >> "$PROFILE"
            echo "$EXPORT_LINE" >> "$PROFILE"
            echo "  -> Added $INSTALL_DIR to PATH in $PROFILE"
        fi
    fi
}

main() {
    detect_platform
    get_latest_version
    install
    configure_path

    echo ""
    echo "✓ MegaCLI installed to $INSTALL_DIR/$BINARY_NAME"
    echo "  Restart your terminal or run:"
    echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
    echo "  Then: megacli --help"
}

main
