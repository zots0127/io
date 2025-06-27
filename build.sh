#!/bin/bash

# Build and commit script for io project
# Usage: ./build.sh [platform|commit] [commit-message]

set -e

VERSION=${VERSION:-"dev"}
LDFLAGS="-s -w -X main.Version=$VERSION"

# Function to commit changes
commit_changes() {
    local commit_message="$1"
    
    if [ -z "$commit_message" ]; then
        echo "Error: Commit message is required"
        echo "Usage: $0 commit \"your commit message\""
        exit 1
    fi
    
    echo "Checking git status..."
    if ! git diff --quiet || ! git diff --cached --quiet; then
        echo "Changes detected, committing..."
        
        # Add all changes
        git add .
        
        # Create commit
        git commit -m "$commit_message"
        
        echo "Changes committed successfully!"
        echo "Current status:"
        git log --oneline -1
    else
        echo "No changes to commit"
    fi
}

# Handle commit command
if [ "$1" = "commit" ]; then
    commit_changes "$2"
    exit 0
fi

# Build for current platform by default
if [ -z "$1" ]; then
    echo "Building for current platform..."
    go build -ldflags="$LDFLAGS" -o io .
    echo "Built: ./io"
    exit 0
fi

# Build for specific platform
case "$1" in
    "all")
        echo "Building for all platforms..."
        # Linux
        GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-linux-amd64 .
        GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o io-linux-arm64 .
        GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="$LDFLAGS" -o io-linux-armv7 .
        
        # macOS
        GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-darwin-amd64 .
        GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o io-darwin-arm64 .
        
        # Windows
        GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-windows-amd64.exe .
        GOOS=windows GOARCH=arm64 go build -ldflags="$LDFLAGS" -o io-windows-arm64.exe .
        
        # FreeBSD
        GOOS=freebsd GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-freebsd-amd64 .
        GOOS=freebsd GOARCH=arm64 go build -ldflags="$LDFLAGS" -o io-freebsd-arm64 .
        
        echo "All builds completed!"
        ;;
    "linux")
        echo "Building for Linux amd64..."
        GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-linux-amd64 .
        ;;
    "darwin")
        echo "Building for macOS arm64..."
        GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o io-darwin-arm64 .
        ;;
    "windows")
        echo "Building for Windows amd64..."
        GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-windows-amd64.exe .
        ;;
    *)
        echo "Usage: $0 [all|linux|darwin|windows|commit]"
        echo ""
        echo "Build commands:"
        echo "  $0           - Build for current platform"
        echo "  $0 all       - Build for all platforms"
        echo "  $0 linux     - Build for Linux amd64"
        echo "  $0 darwin    - Build for macOS arm64"
        echo "  $0 windows   - Build for Windows amd64"
        echo ""
        echo "Git commands:"
        echo "  $0 commit \"message\" - Add all changes and commit with message"
        echo ""
        echo "Examples:"
        echo "  $0 commit \"Add new feature\""
        echo "  $0 commit \"Fix bug in API\""
        exit 1
        ;;
esac