/*
 * win32_process_info.c
 * Copyright (C) 2026 Kovid Goyal <kovid at kovidgoyal.net>
 *
 * Distributed under terms of the GPL3 license.
 */

#ifdef _WIN32
// Only Python.h is included here (not data-types.h) as kitty's typedefs
// collide with <windows.h>
#define PY_SSIZE_T_CLEAN
#include <Python.h>
#define WIN32_LEAN_AND_MEAN
#define NOMINMAX
#include <windows.h>
#include <tlhelp32.h>
#include <psapi.h>
#include <shellapi.h>
#include <winternl.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#define UNUSED __attribute__((unused))

static bool
pid_from_arg(PyObject *pid_, DWORD *pid) {
    if (!PyLong_Check(pid_)) {
        PyErr_SetString(PyExc_TypeError, "pid must be an int");
        return false;
    }
    long p = PyLong_AsLong(pid_);
    if (p < 0) {
        PyErr_SetString(PyExc_TypeError, "pid cannot be negative");
        return false;
    }
    *pid = (DWORD)p;
    return true;
}

static PyObject*
set_error_from_last_error(void) {
    return PyErr_SetFromWindowsErr(GetLastError());
}

static HANDLE
open_process(DWORD pid, DWORD access) {
    HANDLE h = OpenProcess(access, FALSE, pid);
    if (!h) set_error_from_last_error();
    return h;
}

static PyObject*
abspath_of_process(PyObject *self UNUSED, PyObject *pid_) {
    DWORD pid;
    if (!pid_from_arg(pid_, &pid)) return NULL;
    HANDLE h = open_process(pid, PROCESS_QUERY_LIMITED_INFORMATION);
    if (!h) return NULL;
    wchar_t buf[32768];
    DWORD sz = sizeof(buf) / sizeof(buf[0]);
    BOOL ok = QueryFullProcessImageNameW(h, 0, buf, &sz);
    CloseHandle(h);
    if (!ok) return set_error_from_last_error();
    return PyUnicode_FromWideChar(buf, sz);
}

// Reading the PEB of another process {{{
typedef NTSTATUS (NTAPI *NtQueryInformationProcess_func)(HANDLE, PROCESSINFOCLASS, PVOID, ULONG, PULONG);

static NtQueryInformationProcess_func
nt_query_information_process(void) {
    static NtQueryInformationProcess_func f = NULL;
    if (!f) {
        HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
        if (ntdll) f = (NtQueryInformationProcess_func)(void*)GetProcAddress(ntdll, "NtQueryInformationProcess");
    }
    return f;
}

// The documented RTL_USER_PROCESS_PARAMETERS in winternl.h omits most fields,
// the layout below follows the well known 64-bit structure.
typedef struct {
    ULONG MaximumLength, Length, Flags, DebugFlags;
    HANDLE ConsoleHandle;
    ULONG ConsoleFlags;
    HANDLE StandardInput, StandardOutput, StandardError;
    UNICODE_STRING CurrentDirectoryPath;
    HANDLE CurrentDirectoryHandle;
    UNICODE_STRING DllPath, ImagePathName, CommandLine;
} KittyUserProcessParameters;

static bool
read_remote(HANDLE h, const void *addr, void *dest, SIZE_T sz) {
    SIZE_T n = 0;
    if (!ReadProcessMemory(h, addr, dest, sz, &n) || n != sz) {
        set_error_from_last_error();
        return false;
    }
    return true;
}

static bool
read_process_parameters(HANDLE h, KittyUserProcessParameters *params) {
    NtQueryInformationProcess_func f = nt_query_information_process();
    if (!f) {
        PyErr_SetString(PyExc_OSError, "NtQueryInformationProcess not available");
        return false;
    }
    PROCESS_BASIC_INFORMATION pbi;
    ULONG len = 0;
    NTSTATUS status = f(h, ProcessBasicInformation, &pbi, sizeof(pbi), &len);
    if (status != 0) {
        PyErr_Format(PyExc_OSError, "NtQueryInformationProcess failed with status: 0x%lx", (unsigned long)status);
        return false;
    }
    PEB peb;
    if (!read_remote(h, pbi.PebBaseAddress, &peb, sizeof(peb))) return false;
    return read_remote(h, peb.ProcessParameters, params, sizeof(*params));
}

static PyObject*
read_remote_unicode_string(HANDLE h, const UNICODE_STRING *s) {
    if (!s->Buffer || !s->Length) return PyUnicode_FromString("");
    wchar_t *buf = malloc(s->Length + sizeof(wchar_t));
    if (!buf) return PyErr_NoMemory();
    if (!read_remote(h, s->Buffer, buf, s->Length)) {
        free(buf);
        return NULL;
    }
    PyObject *ans = PyUnicode_FromWideChar(buf, s->Length / sizeof(wchar_t));
    free(buf);
    return ans;
}

static PyObject*
process_parameter_string(PyObject *pid_, bool want_cwd) {
    DWORD pid;
    if (!pid_from_arg(pid_, &pid)) return NULL;
    HANDLE h = open_process(pid, PROCESS_QUERY_INFORMATION | PROCESS_VM_READ);
    if (!h) return NULL;
    KittyUserProcessParameters params;
    PyObject *ans = NULL;
    if (read_process_parameters(h, &params)) {
        ans = read_remote_unicode_string(h, want_cwd ? &params.CurrentDirectoryPath : &params.CommandLine);
    }
    CloseHandle(h);
    return ans;
}
// }}}

static PyObject*
cwd_of_process(PyObject *self UNUSED, PyObject *pid_) {
    PyObject *ans = process_parameter_string(pid_, true);
    if (!ans) return NULL;
    // The PEB stores the cwd with a trailing backslash (except for drive roots)
    Py_ssize_t len = PyUnicode_GET_LENGTH(ans);
    if (len > 3 && PyUnicode_READ_CHAR(ans, len - 1) == L'\\') {
        PyObject *stripped = PyUnicode_Substring(ans, 0, len - 1);
        Py_DECREF(ans);
        ans = stripped;
    }
    return ans;
}

static PyObject*
cmdline_of_process(PyObject *self UNUSED, PyObject *pid_) {
    PyObject *cmdline = process_parameter_string(pid_, false);
    if (!cmdline) return NULL;
    PyObject *ans = PyList_New(0);
    if (!ans) {
        Py_DECREF(cmdline);
        return NULL;
    }
    if (PyUnicode_GET_LENGTH(cmdline) > 0) {
        wchar_t *w = PyUnicode_AsWideCharString(cmdline, NULL);
        if (!w) {
            Py_DECREF(cmdline);
            Py_DECREF(ans);
            return NULL;
        }
        int argc = 0;
        wchar_t **argv = CommandLineToArgvW(w, &argc);
        PyMem_Free(w);
        if (!argv) {
            Py_DECREF(cmdline);
            Py_DECREF(ans);
            return set_error_from_last_error();
        }
        for (int i = 0; i < argc; i++) {
            PyObject *arg = PyUnicode_FromWideChar(argv[i], -1);
            if (!arg || PyList_Append(ans, arg) != 0) {
                Py_XDECREF(arg);
                Py_CLEAR(ans);
                break;
            }
            Py_DECREF(arg);
        }
        LocalFree(argv);
    }
    Py_DECREF(cmdline);
    return ans;
}

// Returns a tuple of (pid, parent_pid) for all processes on the system
static PyObject*
process_group_map(PyObject *self UNUSED, PyObject *args UNUSED) {
    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) return set_error_from_last_error();
    PyObject *ans = PyList_New(0);
    if (!ans) {
        CloseHandle(snap);
        return NULL;
    }
    PROCESSENTRY32W pe = {.dwSize = sizeof(PROCESSENTRY32W)};
    for (BOOL ok = Process32FirstW(snap, &pe); ok; ok = Process32NextW(snap, &pe)) {
        PyObject *item = Py_BuildValue("kk", (unsigned long)pe.th32ProcessID, (unsigned long)pe.th32ParentProcessID);
        if (!item || PyList_Append(ans, item) != 0) {
            Py_XDECREF(item);
            Py_CLEAR(ans);
            break;
        }
        Py_DECREF(item);
    }
    CloseHandle(snap);
    if (!ans) return NULL;
    PyObject *ret = PyList_AsTuple(ans);
    Py_DECREF(ans);
    return ret;
}

static unsigned long long
memory_of_pid(DWORD pid) {
    HANDLE h = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!h) return 0;
    PROCESS_MEMORY_COUNTERS_EX pmc = {.cb = sizeof(pmc)};
    unsigned long long ans = 0;
    if (GetProcessMemoryInfo(h, (PROCESS_MEMORY_COUNTERS*)&pmc, sizeof(pmc))) ans = pmc.PrivateUsage;
    CloseHandle(h);
    return ans;
}

// Memory used by the process and all its descendants
static PyObject*
memory_of_process_tree(PyObject *self UNUSED, PyObject *pid_) {
    DWORD root;
    if (!pid_from_arg(pid_, &root)) return NULL;
    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) return set_error_from_last_error();
    size_t count = 0, cap = 256;
    DWORD (*procs)[2] = malloc(cap * sizeof(*procs));
    if (!procs) {
        CloseHandle(snap);
        return PyErr_NoMemory();
    }
    PROCESSENTRY32W pe = {.dwSize = sizeof(PROCESSENTRY32W)};
    for (BOOL ok = Process32FirstW(snap, &pe); ok; ok = Process32NextW(snap, &pe)) {
        if (count >= cap) {
            cap *= 2;
            DWORD (*n)[2] = realloc(procs, cap * sizeof(*procs));
            if (!n) {
                free(procs);
                CloseHandle(snap);
                return PyErr_NoMemory();
            }
            procs = n;
        }
        procs[count][0] = pe.th32ProcessID;
        procs[count][1] = pe.th32ParentProcessID;
        count++;
    }
    CloseHandle(snap);
    bool *in_tree = calloc(count, sizeof(bool));
    if (!in_tree) {
        free(procs);
        return PyErr_NoMemory();
    }
    bool found_root = false;
    for (size_t i = 0; i < count; i++) if (procs[i][0] == root) { in_tree[i] = true; found_root = true; }
    if (!found_root) {
        free(procs);
        free(in_tree);
        PyErr_Format(PyExc_ProcessLookupError, "No process with pid: %lu", (unsigned long)root);
        return NULL;
    }
    // Snapshot ordering is arbitrary so iterate until no new descendants are found
    for (bool changed = true; changed;) {
        changed = false;
        for (size_t i = 0; i < count; i++) {
            if (in_tree[i] || procs[i][0] == 0) continue;
            for (size_t j = 0; j < count; j++) {
                if (in_tree[j] && procs[i][1] == procs[j][0] && procs[i][0] != procs[j][0]) {
                    in_tree[i] = true;
                    changed = true;
                    break;
                }
            }
        }
    }
    unsigned long long total = 0;
    for (size_t i = 0; i < count; i++) if (in_tree[i]) total += memory_of_pid(procs[i][0]);
    free(procs);
    free(in_tree);
    return PyLong_FromUnsignedLongLong(total);
}

static PyMethodDef module_methods[] = {
    {"cwd_of_process", cwd_of_process, METH_O, ""},
    {"abspath_of_process", abspath_of_process, METH_O, ""},
    {"cmdline_of_process", cmdline_of_process, METH_O, ""},
    {"process_group_map", process_group_map, METH_NOARGS, ""},
    {"memory_of_process_tree", memory_of_process_tree, METH_O, ""},
    {NULL, NULL, 0, NULL}
};

bool
init_win32_process_info(PyObject *module) {
    if (PyModule_AddFunctions(module, module_methods) != 0) return false;
    return true;
}
#endif
