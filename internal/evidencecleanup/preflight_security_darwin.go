// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

const darwinSystemLibrary = "/usr/lib/libSystem.B.dylib"

var darwinPreflightSecurity = sync.OnceValues(func() (darwinSecurityAPI, error) {
	return loadDarwinSecurity(purego.Dlopen, purego.Dlsym, purego.Dlclose)
})

func loadDarwinSecurity(
	open func(string, int) (uintptr, error),
	lookup func(uintptr, string) (uintptr, error),
	closeLibrary func(uintptr) error,
) (darwinSecurityAPI, error) {
	handle, err := open(darwinSystemLibrary, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return darwinSecurityAPI{}, fmt.Errorf("open libSystem: %w", err)
	}
	api, err := bindDarwinSecurity(handle, lookup)
	if err != nil {
		return darwinSecurityAPI{}, errors.Join(err, closeLibrary(handle))
	}
	// Retain this dlopen reference for the cached functions' process lifetime.
	// Closing it here could invalidate them. Initialization is lazy so a failed
	// binding refuses preflight without disabling unrelated host operations.
	return api, nil
}

func bindDarwinSecurity(
	handle uintptr, lookup func(uintptr, string) (uintptr, error),
) (darwinSecurityAPI, error) {
	getattr, err := lookup(handle, "fgetattrlist")
	if err != nil {
		return darwinSecurityAPI{}, fmt.Errorf("resolve fgetattrlist: %w", err)
	}
	return registerDarwinSecurity(getattr)
}

func registerDarwinSecurity(getattr uintptr) (darwinSecurityAPI, error) {
	if getattr == 0 {
		return darwinSecurityAPI{}, fmt.Errorf("resolve libSystem ACL API: null symbol")
	}
	// Public ABI: xnu/bsd/sys/attr.h and bsd/man/man2/getattrlist.2 declare
	// int fgetattrlist(int, struct attrlist *, void *, size_t, unsigned long).
	// Both Darwin targets use 32-bit C int and 64-bit size_t/unsigned long;
	// x/sys supplies attrlist. SyscallN captures errno in PureGo's Darwin
	// trampoline before Go runtime bookkeeping can overwrite thread-local errno.
	// Use the maintained FFI per Constitution V, not private runtime/assembly
	// stubs or unsupported direct Darwin syscalls. No struct is passed by value.
	return darwinSecurityAPI{
		read: func(fd int32, buffer *[darwinSecurityBufferSize]byte) (int32, int32) {
			if fd < 0 {
				return -1, int32(unix.EBADF)
			}
			attrs := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
				Commonattr: unix.ATTR_CMN_EXTENDED_SECURITY}
			// Convert pointers only in the SyscallN argument list, as required
			// by unsafe.Pointer rule 4. Both objects are pointer-free and the
			// synchronous libSystem call does not retain either pointer.
			result, _, errno := purego.SyscallN(getattr, uintptr(fd),
				uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(buffer)), uintptr(len(buffer)), 0)
			// fgetattrlist is synchronous and retains neither pointer. Keep the
			// pointer-free Go storage alive until libSystem has finished with it.
			runtime.KeepAlive(&attrs)
			runtime.KeepAlive(buffer)
			return darwinSecurityCallResult(result, errno)
		},
	}, nil
}
