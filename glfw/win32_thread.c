//========================================================================
// GLFW 3.4 Win32 - www.glfw.org
//------------------------------------------------------------------------
// Copyright (c) 2002-2006 Marcus Geelnard
// Copyright (c) 2006-2017 Camilla Löwy <elmindreda@glfw.org>
//
// This software is provided 'as-is', without any express or implied
// warranty. In no event will the authors be held liable for any damages
// arising from the use of this software.
//
// Permission is granted to anyone to use this software for any purpose,
// including commercial applications, and to alter it and redistribute it
// freely, subject to the following restrictions:
//
// 1. The origin of this software must not be misrepresented; you must not
//    claim that you wrote the original software. If you use this software
//    in a product, an acknowledgment in the product documentation would
//    be appreciated but is not required.
//
// 2. Altered source versions must be plainly marked as such, and must not
//    be misrepresented as being the original software.
//
// 3. This notice may not be removed or altered from any source
//    distribution.
//
//========================================================================
// Please use C89 style variable declarations in this file because VS 2010
//========================================================================

#include "internal.h"

#include <assert.h>


//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

bool
_glfwPlatformCreateTls(_GLFWtls *tls) {
    assert(tls->win32.allocated == false);

    tls->win32.index = TlsAlloc();
    if (tls->win32.index == TLS_OUT_OF_INDEXES) {
        _glfwInputErrorWin32(GLFW_PLATFORM_ERROR, "Win32: Failed to allocate TLS index");
        return false;
    }

    tls->win32.allocated = true;
    return true;
}

void
_glfwPlatformDestroyTls(_GLFWtls *tls) {
    if (tls->win32.allocated) TlsFree(tls->win32.index);
    memset(tls, 0, sizeof(_GLFWtls));
}

void *
_glfwPlatformGetTls(_GLFWtls *tls) {
    assert(tls->win32.allocated == true);
    return TlsGetValue(tls->win32.index);
}

void
_glfwPlatformSetTls(_GLFWtls *tls, void *value) {
    assert(tls->win32.allocated == true);
    TlsSetValue(tls->win32.index, value);
}

bool
_glfwPlatformCreateMutex(_GLFWmutex *mutex) {
    assert(mutex->win32.allocated == false);
    InitializeCriticalSection(&mutex->win32.section);
    return mutex->win32.allocated = true;
}

void
_glfwPlatformDestroyMutex(_GLFWmutex *mutex) {
    if (mutex->win32.allocated) DeleteCriticalSection(&mutex->win32.section);
    memset(mutex, 0, sizeof(_GLFWmutex));
}

void
_glfwPlatformLockMutex(_GLFWmutex *mutex) {
    assert(mutex->win32.allocated == true);
    EnterCriticalSection(&mutex->win32.section);
}

void
_glfwPlatformUnlockMutex(_GLFWmutex *mutex) {
    assert(mutex->win32.allocated == true);
    LeaveCriticalSection(&mutex->win32.section);
}
