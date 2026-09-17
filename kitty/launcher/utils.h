/*
 * utils.h
 * Copyright (C) 2025 Kovid Goyal <kovid at kovidgoyal.net>
 *
 * Distributed under terms of the GPL3 license.
 */

#pragma once
#include <stddef.h>
#include <stdlib.h>
#include <stdbool.h>
#include <ctype.h>
#ifdef _WIN32
#include "../win32-compat.h"
#else
#include <pwd.h>
#endif
#include <unistd.h>
#include <stdio.h>
#include <string.h>
#include <limits.h>
#include <errno.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <limits.h>


static const char *home = NULL;

static void
ensure_home_path(void) {
    if (home) return;
    home = getenv("HOME");
#ifdef _WIN32
    if (!home || !home[0]) home = getenv("USERPROFILE");
#else
    if (!home || !home[0]) {
        struct passwd *pw = getpwuid(geteuid());
        if (pw) home = pw->pw_dir;
    }
#endif
    if (!home || !home[0]) {
        fprintf(stderr, "Fatal error: Cannot determine home directory\n");
        exit(1);
    }
}

#define safe_snprintf(buf, sz, fmt, ...)                                                                        \
    {                                                                                                           \
        int n = snprintf(buf, sz, fmt, __VA_ARGS__);                                                            \
        if (n < 0 || (size_t)n >= sz) {                                                                         \
            fprintf(stderr, "Out of buffer space calling sprintf for format: %s at line: %d\n", fmt, __LINE__); \
            exit(1);                                                                                            \
        }                                                                                                       \
    }

static const char *
home_path_for(const char *username) {
#ifdef _WIN32
    (void)username;
#else
    struct passwd *pw = getpwnam(username);
    if (pw) return pw->pw_dir;
#endif
    return NULL;
}

static void
expand_tilde(const char *path, char *ans, size_t ans_sz) {
    if (path[0] != '~') {
        safe_snprintf(ans, ans_sz, "%s", path);
        return;
    }
    const char *prefix = NULL, *sep = "";
    if (path[1] == '/' || path[1] == '\0') {
        // If the path is "~" or "~/something", get the current user's home directory
        ensure_home_path();
        prefix = home;
    } else {
        // If the path is "~user/something", get the specified user's home directory
        char *slash = (char *)strchr(path, '/'); // path is temporarily modified here and then restored
        if (slash) {
            *slash = 0;
            prefix = home_path_for(path + 1);
            *slash = '/';
        } else prefix = home_path_for(path + 1);
        if (prefix) path = slash ? slash - 1 : "a";
        else {
            prefix = "";
            path--;
        }
    }
    // Construct the expanded path
    safe_snprintf(ans, ans_sz, "%s%s%s", prefix, sep, path + 1);
}

// Length of the root prefix that path components must not be popped past:
// "C:/" -> 3, "//" -> 2 (UNC), "/" or relative -> 0
static size_t
path_root_len(const char *path) {
#ifdef _WIN32
#define is_sep(c) ((c) == '/' || (c) == '\\')
    if (isalpha((unsigned char)path[0]) && path[1] == ':') return is_sep(path[2]) ? 3 : 2;
    if (is_sep(path[0]) && is_sep(path[1])) return 2;
#undef is_sep
#else
    (void)path;
#endif
    return 0;
}

static size_t
clean_path(char *path) {
#ifdef _WIN32
    for (char *p = path; *p; p++)
        if (*p == '\\') *p = '/';
#endif
    char *root = path + path_root_len(path);
    // start on the root's trailing slash so the first component is handled like any other
    char *start = root > path && root[-1] == '/' ? root - 1 : root;
    char *write_ptr = start;
    char *read_ptr = start;
    while (*read_ptr) {
        if (read_ptr[0] != '/') {
            *write_ptr++ = *read_ptr++;
            continue;
        }
        // we have /
        if (read_ptr[1] == '/') {
            read_ptr++;
            continue;
        } // skip one slash of double slash
        if (read_ptr[1] != '.') {
            *write_ptr++ = *read_ptr++;
            continue;
        }
        // we have /.
        if (read_ptr[2] == '/' || !read_ptr[2]) {
            read_ptr += 2;
            continue;
        } // skip /./
        if (read_ptr[2] != '.') {
            *write_ptr++ = *read_ptr++;
            continue;
        }
        // we have /..
        if (read_ptr[3] == '/' || !read_ptr[3]) {
            read_ptr += 3;
            while (write_ptr > start) {
                write_ptr--;
                if (*write_ptr == '/') break;
            }
        } else *write_ptr++ = *read_ptr++;
    }
    if (write_ptr < root) write_ptr = root;
    // remove trailing slashes
    char *min_end = root > path ? root : path + 1;
    while (write_ptr > min_end && *(write_ptr - 1) == '/') write_ptr--;
    // Null-terminate the normalized path
    *write_ptr++ = '\0';
    return write_ptr - path - 1;
}

static size_t
lexical_absolute_path(const char *relative, char *output, size_t outsz) {
    size_t rlen = strlen(relative);
    char *limit = output + outsz;
    char *write_ptr = output; // Points to the location to write normalized characters
#define _ensure_space(n)                                                                                      \
    if (write_ptr + n + 1 >= limit) {                                                                         \
        fprintf(stderr, "Out of buffer space making absolute path for: %s with cwd: %s\n", relative, output); \
        exit(1);                                                                                              \
    }
#ifdef _WIN32
    bool is_absolute = path_root_len(relative) > 0;
    // A leading slash without a drive is relative to the drive of the cwd
    bool is_drive_relative = !is_absolute && (relative[0] == '/' || relative[0] == '\\');
#else
    bool is_absolute = relative[0] == '/';
    bool is_drive_relative = false;
#endif
    if (!is_absolute) {
        if (!getcwd(output, outsz)) {
            perror("Getting the current working directory failed with error");
            exit(1);
        }
        size_t cwdlen = strlen(output);
        if (is_drive_relative) {
            cwdlen = isalpha((unsigned char)output[0]) && output[1] == ':' ? 2 : 0;
            output[cwdlen] = 0;
        }
        write_ptr = output + cwdlen;
        _ensure_space(cwdlen + rlen + 2);
        if (rlen && cwdlen && !is_drive_relative && *(write_ptr - 1) != '/' && *(write_ptr - 1) != '\\') *(write_ptr++) = '/';
    } else {
        _ensure_space(rlen + 2);
    }
#undef _ensure_space
    // Append the relative path
    memcpy(write_ptr, relative, rlen);
    *(write_ptr + rlen) = 0;
    size_t ans = clean_path(output);
    // Ensure the path is not empty
    if (output[0] == '\0') {
        output[0] = '/';
        output[1] = 0;
        ans = 1;
    }
    return ans;
}

static bool
makedirs_cleaned(char *path, int mode, struct stat *buffer) {
    if (stat(path, buffer) == 0) {
        if (S_ISDIR(buffer->st_mode)) return true;
        errno = ENOTDIR;
        return false;
    }
    if (errno == ENOTDIR) return false;
    char *p = strrchr(path, '/');
    if (p && p > path) {
        p[0] = 0;
        bool parent_created = makedirs_cleaned(path, mode, buffer);
        p[0] = '/';
        if (!parent_created) return false;
    }
    // Now parent exists
    return mkdir(path, mode) == 0;
}

static bool
makedirs(const char *path, int mode) {
    struct stat buffer;
    char pbuf[PATH_MAX];
    lexical_absolute_path(path, pbuf, sizeof(pbuf));
    return makedirs_cleaned(pbuf, mode, &buffer);
}

static bool
is_dir_ok_for_config(char *q) {
    size_t len = strlen(q);
    memcpy(q + len, "/kitty", sizeof("/kitty"));
    len += sizeof("/kitty") - 1;
    memcpy(q + len, "/kitty.conf", sizeof("/kitty.conf"));
    if (access(q, F_OK) != 0) return false;
    q[len] = 0;
    return access(q, W_OK) == 0;
}

static bool
get_config_dir(char *output, size_t outputsz) {
    const char *q;
    char buf1[PATH_MAX], buf2[PATH_MAX];
#define expand(x, dest, sz)                    \
    {                                          \
        expand_tilde(x, buf1, sizeof(buf1));   \
        lexical_absolute_path(buf1, dest, sz); \
    }
    q = getenv("KITTY_CONFIG_DIRECTORY");
    if (q && q[0]) {
        expand(q, output, outputsz);
        return true;
    }
#define check_and_ret(x)                               \
    if (x && x[0]) {                                   \
        expand(x, output, outputsz);                   \
        if (is_dir_ok_for_config(output)) return true; \
    }
    q = getenv("XDG_CONFIG_HOME");
    check_and_ret(q);
    check_and_ret("~/.config");
#ifdef __APPLE__
    check_and_ret("~/Library/Preferences");
#endif
    q = getenv("XDG_CONFIG_DIRS");
    if (q && q[0]) {
        safe_snprintf(buf2, sizeof(buf2), "%s", q);
        char *s, *token = strtok_r(buf2, ":", &s);
        while (token) {
            check_and_ret(token);
            token = strtok_r(NULL, ":", &s);
        }
    }
    q = getenv("XDG_CONFIG_HOME");
    if (!q || !q[0]) q = "~/.config";
    expand(q, buf2, sizeof(buf2));
    safe_snprintf(output, outputsz, "%s/kitty", buf2);
    if (makedirs(output, 0755)) return true;
    return false;
#undef expand
#undef check_and_ret
}


static ssize_t
safe_read_stream(void *ptr, size_t size, FILE *stream) {
    errno = 0;
    ssize_t total = 0, bytes_to_read = size;
    while (total < bytes_to_read) {
        size_t n = fread((char *)ptr + total, 1, bytes_to_read - total, stream);
        if (n > 0) total += n;
        else {
            if (!ferror(stream)) break; // eof
            if (errno != EINTR) return -1;
            clearerr(stream);
        }
    }
    return total;
}

static char *
read_full_file(const char *filename, size_t *sz) {
    FILE *file = NULL;
    errno = EINTR;
    while (file == NULL && errno == EINTR) file = fopen(filename, "rb");
    if (!file) return NULL;
    fseek(file, 0, SEEK_END);
    unsigned long file_size = ftell(file);
    rewind(file);
    char *buffer = (char *)malloc(file_size + 1); // +1 for the null terminator
    if (!buffer) {
        errno = EINTR;
        while (errno == EINTR && fclose(file) != 0);
        errno = ENOMEM;
        return NULL;
    }
    ssize_t q = safe_read_stream(buffer, file_size, file);
    int saved = errno;
    errno = EINTR;
    while (errno == EINTR && fclose(file) != 0);
    errno = saved;
    if (q < 0) {
        free(buffer);
        buffer = NULL;
        if (sz) *sz = 0;
    } else {
        if (sz) { *sz = q; }
        buffer[q] = 0;
    }
    return buffer;
}
