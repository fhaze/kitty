#!/bin/sh
#
# Install the Linux kitten binary shipped with the Windows kitty package into
# the current WSL distribution and put it on the PATH of the user's login shell.
#
# Usage: install-kitten.sh [--uninstall] [--no-path] SOURCE_DIR
#
# SOURCE_DIR is the directory containing the kitten-linux-* binaries, either as
# a Windows path (C:\Program Files\kitty\lib\kitty\windows\wsl) or a WSL path.
# Runs under any POSIX sh (dash, busybox ash, bash, ...) and is idempotent.

set -u

marker="kitty-wsl-kitten"
dest_dir="${KITTY_WSL_BIN_DIR:-$HOME/.local/bin}"
uninstall=0
update_path=1
src=""

die() {
    printf '%s\n' "install-kitten.sh: $*" >&2
    exit 1
}

while [ $# -gt 0 ]; do
    case "$1" in
        --uninstall) uninstall=1 ;;
        --no-path) update_path=0 ;;
        --) shift; break ;;
        -*) die "unknown option: $1" ;;
        *) src="$1" ;;
    esac
    shift
done
[ $# -gt 0 ] && src="$1"

to_wsl_path() {
    case "$1" in
        /*) printf '%s' "$1" ;;
        *)
            if command -v wslpath >/dev/null 2>&1; then
                command wslpath -u "$1"
            else
                printf '%s' "$1"
            fi
            ;;
    esac
}

kitten_arch() {
    case "$(uname -m)" in
        x86_64 | amd64) echo amd64 ;;
        aarch64 | arm64) echo arm64 ;;
        armv7l | armv6l | armhf) echo arm ;;
        i?86) echo 386 ;;
        *) return 1 ;;
    esac
}

login_shell() {
    shell=""
    if command -v getent >/dev/null 2>&1; then
        shell=$(command getent passwd "$(id -un)" 2>/dev/null | command awk -F: 'NR == 1 { print $NF }')
    fi
    [ -n "$shell" ] || shell=$(command awk -F: -v u="$(id -un)" '$1 == u { print $NF; exit }' /etc/passwd 2>/dev/null)
    [ -n "$shell" ] || shell="${SHELL:-/bin/sh}"
    printf '%s' "${shell##*/}"
}

posix_path_block() {
    printf '\n# >>> %s >>>\ncase ":$PATH:" in\n    *":%s:"*) ;;\n    *) export PATH="%s:$PATH" ;;\nesac\n# <<< %s <<<\n' "$marker" "$1" "$1" "$marker"
}

fish_path_block() {
    printf '# >>> %s >>>\nif not contains -- "%s" $PATH\n    set -gx PATH "%s" $PATH\nend\n# <<< %s <<<\n' "$marker" "$1" "$1" "$marker"
}

csh_path_block() {
    printf '\n# >>> %s >>>\nif ( ":${PATH}:" !~ *":%s:"* ) setenv PATH "%s:${PATH}"\n# <<< %s <<<\n' "$marker" "$1" "$1" "$marker"
}

# Config files the login shell reads for interactive sessions, first one wins
# for adding, all of them are cleaned on uninstall.
rc_files_for() {
    case "$1" in
        bash) echo "$HOME/.bashrc" ;;
        zsh) echo "${ZDOTDIR:-$HOME}/.zshrc" ;;
        fish) echo "${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/$marker.fish" ;;
        tcsh) echo "$HOME/.tcshrc" ;;
        csh) echo "$HOME/.cshrc" ;;
        *) echo "$HOME/.profile" ;;
    esac
}

# Print the shell config file contents without the block we added, and
# without the blank line that was inserted before it.
strip_block() {
    command awk -v m="$marker" '
        index($0, "# >>> " m " >>>") { skip = 1; held_blank = 0; next }
        index($0, "# <<< " m " <<<") { skip = 0; next }
        skip { next }
        { if (held_blank) print ""; held_blank = 0 }
        /^$/ { held_blank = 1; next }
        { print }
        END { if (held_blank) print "" }
    ' "$1"
}

remove_path_block() {
    rc="$1"
    [ -f "$rc" ] || return 0
    command grep -q "$marker" "$rc" 2>/dev/null || return 0
    case "$rc" in
        *"/conf.d/$marker.fish") command rm -f -- "$rc" && echo "Removed $rc" ;;
        *)
            tmp="$rc.$marker.tmp"
            strip_block "$rc" > "$tmp" || die "failed to rewrite $rc"
            if [ -z "$(command tr -d '[:space:]' < "$tmp")" ]; then
                # the file only ever held our block
                command rm -f -- "$tmp" "$rc" || die "failed to remove $rc"
                echo "Removed $rc"
            else
                command mv -f -- "$tmp" "$rc" || die "failed to rewrite $rc"
                echo "Removed PATH entry from $rc"
            fi
            ;;
    esac
}

add_path_block() {
    shell="$1"
    rc=$(rc_files_for "$shell")
    if [ -f "$rc" ] && command grep -q "$marker" "$rc" 2>/dev/null; then
        echo "PATH entry already present in $rc"
        return 0
    fi
    command mkdir -p "$(dirname "$rc")" || die "failed to create $(dirname "$rc")"
    case "$shell" in
        fish) fish_path_block "$dest_dir" > "$rc" ;;
        tcsh | csh) csh_path_block "$dest_dir" >> "$rc" ;;
        *) posix_path_block "$dest_dir" >> "$rc" ;;
    esac || die "failed to write $rc"
    echo "Added $dest_dir to PATH in $rc"
}

if [ "$uninstall" = 1 ]; then
    if [ -e "$dest_dir/kitten" ]; then
        command rm -f -- "$dest_dir/kitten" || die "failed to remove $dest_dir/kitten"
        echo "Removed $dest_dir/kitten"
    fi
    for shell in bash zsh fish tcsh csh sh; do
        remove_path_block "$(rc_files_for "$shell")"
    done
    exit 0
fi

[ -n "$src" ] || die "the directory containing the kitten binaries must be specified"
src_dir=$(to_wsl_path "$src")
[ -d "$src_dir" ] || die "$src_dir does not exist, is the Windows drive mounted in this distribution?"
arch=$(kitten_arch) || die "unsupported architecture: $(uname -m)"
binary="$src_dir/kitten-linux-$arch"
[ -f "$binary" ] || die "$binary not found"

command mkdir -p "$dest_dir" || die "failed to create $dest_dir"
# copy then rename so a running kitten is replaced atomically
tmp="$dest_dir/.kitten.$$"
command cp -f -- "$binary" "$tmp" && command chmod 755 "$tmp" && command mv -f -- "$tmp" "$dest_dir/kitten" || {
    command rm -f -- "$tmp"
    die "failed to install $dest_dir/kitten"
}
ver=$("$dest_dir/kitten" --version 2>/dev/null) || die "$dest_dir/kitten does not run on this system"
echo "Installed $ver to $dest_dir/kitten"

if [ "$update_path" = 1 ]; then
    add_path_block "$(login_shell)"
fi
