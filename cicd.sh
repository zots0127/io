#!/bin/bash

# CI/CD Script for io project
# Interactive script for build, commit, push, and release operations

set -e

VERSION=${VERSION:-"dev"}
LDFLAGS="-s -w -X main.Version=$VERSION"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored text
print_color() {
    printf "${1}${2}${NC}\n"
}

# Show banner
show_banner() {
    print_color $BLUE "================================"
    print_color $BLUE "    IO Project CI/CD Script     "
    print_color $BLUE "================================"
    echo ""
}

# Check git status
check_git_status() {
    if ! git diff --quiet || ! git diff --cached --quiet; then
        print_color $YELLOW "📝 Changes detected in working directory"
        git status --short
        echo ""
        return 1
    else
        print_color $GREEN "✅ Working directory is clean"
        return 0
    fi
}

# Build function
build_project() {
    local platform="$1"
    
    case "$platform" in
        "current")
            print_color $BLUE "🔨 Building for current platform..."
            go build -ldflags="$LDFLAGS" -o io .
            print_color $GREEN "✅ Built: ./io"
            ;;
        "linux")
            print_color $BLUE "🔨 Building for Linux amd64..."
            GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-linux-amd64 .
            print_color $GREEN "✅ Built: ./io-linux-amd64"
            ;;
        "darwin")
            print_color $BLUE "🔨 Building for macOS arm64..."
            GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o io-darwin-arm64 .
            print_color $GREEN "✅ Built: ./io-darwin-arm64"
            ;;
        "windows")
            print_color $BLUE "🔨 Building for Windows amd64..."
            GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o io-windows-amd64.exe .
            print_color $GREEN "✅ Built: ./io-windows-amd64.exe"
            ;;
        "all")
            print_color $BLUE "🔨 Building for all platforms..."
            
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
            
            print_color $GREEN "✅ All builds completed!"
            ;;
        *)
            print_color $RED "❌ Unknown platform: $platform"
            return 1
            ;;
    esac
}

# Commit changes
commit_changes() {
    local commit_message="$1"
    
    if [ -z "$commit_message" ]; then
        echo -n "📝 Enter commit message: "
        read commit_message
        
        if [ -z "$commit_message" ]; then
            print_color $RED "❌ Commit message cannot be empty"
            return 1
        fi
    fi
    
    print_color $BLUE "📦 Adding all changes..."
    git add .
    
    print_color $BLUE "📝 Committing changes..."
    git commit -m "$commit_message"
    
    print_color $GREEN "✅ Changes committed successfully!"
    print_color $BLUE "Latest commit:"
    git log --oneline -1
}

# Push to remote
push_to_remote() {
    local branch=$(git branch --show-current)
    
    print_color $BLUE "🚀 Pushing to origin/$branch..."
    git push origin "$branch"
    
    print_color $GREEN "✅ Pushed to origin/$branch successfully!"
}

# Create and push tag
create_release() {
    local tag_name="$1"
    
    if [ -z "$tag_name" ]; then
        echo -n "🏷️  Enter tag name (e.g., v1.0.0): "
        read tag_name
        
        if [ -z "$tag_name" ]; then
            print_color $RED "❌ Tag name cannot be empty"
            return 1
        fi
    fi
    
    # Validate tag format
    if [[ ! "$tag_name" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        print_color $YELLOW "⚠️  Tag format should be vX.Y.Z (e.g., v1.0.0)"
        echo -n "Continue anyway? (y/N): "
        read confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            print_color $BLUE "❌ Cancelled"
            return 1
        fi
    fi
    
    print_color $BLUE "🏷️  Creating tag $tag_name..."
    git tag "$tag_name"
    
    print_color $BLUE "🚀 Pushing tag to remote..."
    git push origin "$tag_name"
    
    print_color $GREEN "✅ Tag $tag_name created and pushed!"
    print_color $BLUE "🎉 GitHub Actions will automatically build and create a release"
}

# Interactive menu
show_menu() {
    echo ""
    print_color $BLUE "Choose an action:"
    echo "1) 🔍 Check status"
    echo "2) 🔨 Build project"
    echo "3) 📝 Commit changes"
    echo "4) 🚀 Push to remote"
    echo "5) 🏷️  Create release tag"
    echo "6) 🔄 Full workflow (commit + push)"
    echo "7) 🎯 Release workflow (commit + push + tag)"
    echo "0) 🚪 Exit"
    echo ""
}

# Build menu
show_build_menu() {
    echo ""
    print_color $BLUE "Choose build target:"
    echo "1) Current platform"
    echo "2) Linux (amd64)"
    echo "3) macOS (arm64)" 
    echo "4) Windows (amd64)"
    echo "5) All platforms"
    echo "0) Back to main menu"
    echo ""
}

# Main workflow
full_workflow() {
    print_color $BLUE "🔄 Starting full workflow..."
    
    if ! check_git_status; then
        commit_changes ""
        echo ""
    fi
    
    push_to_remote
    print_color $GREEN "✅ Full workflow completed!"
}

# Release workflow
release_workflow() {
    print_color $BLUE "🎯 Starting release workflow..."
    
    if ! check_git_status; then
        commit_changes ""
        echo ""
    fi
    
    push_to_remote
    echo ""
    create_release ""
    print_color $GREEN "🎉 Release workflow completed!"
}

# Main execution
main() {
    show_banner
    
    # Check if git repository
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        print_color $RED "❌ Not a git repository"
        exit 1
    fi
    
    # Initial status check
    check_git_status
    
    while true; do
        show_menu
        echo -n "Enter your choice: "
        read choice
        
        case $choice in
            1)
                echo ""
                check_git_status
                ;;
            2)
                show_build_menu
                echo -n "Enter build choice: "
                read build_choice
                
                case $build_choice in
                    1) build_project "current" ;;
                    2) build_project "linux" ;;
                    3) build_project "darwin" ;;
                    4) build_project "windows" ;;
                    5) build_project "all" ;;
                    0) continue ;;
                    *) print_color $RED "❌ Invalid choice" ;;
                esac
                ;;
            3)
                echo ""
                if check_git_status; then
                    print_color $YELLOW "⚠️  No changes to commit"
                else
                    commit_changes ""
                fi
                ;;
            4)
                echo ""
                push_to_remote
                ;;
            5)
                echo ""
                create_release ""
                ;;
            6)
                echo ""
                full_workflow
                ;;
            7)
                echo ""
                release_workflow
                ;;
            0)
                print_color $GREEN "👋 Goodbye!"
                exit 0
                ;;
            *)
                print_color $RED "❌ Invalid choice"
                ;;
        esac
        
        echo ""
        echo "Press Enter to continue..."
        read
    done
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi