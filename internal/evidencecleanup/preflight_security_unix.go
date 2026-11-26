// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"

	"golang.org/x/sys/unix"
)

const darwinSecurityBufferSize = 4096

// Keep the call/errno protocol independent of the loader (Dependency Inversion)
// so failure paths and response validation can also be tested off Darwin.
type darwinSecurityAPI struct {
	read func(int32, *[darwinSecurityBufferSize]byte) (int32, int32)
}

func (api darwinSecurityAPI) readSecurity(fd int) ([darwinSecurityBufferSize]byte, error) {
	var buffer [darwinSecurityBufferSize]byte
	if fd < 0 || fd > math.MaxInt32 {
		return buffer, fmt.Errorf("inspect extended security ACL: %w", unix.EBADF)
	}
	// The binding captures errno before returning from the foreign call.
	// Reading thread-local storage later is unsafe even with LockOSThread:
	// runtime or FFI bookkeeping can overwrite errno on that same thread.
	result, errno := api.read(int32(fd), &buffer)
	runtime.KeepAlive(&buffer)
	if result != 0 {
		return buffer, darwinSecurityCallError(result, errno)
	}
	return buffer, nil
}

func darwinSecurityCallError(result, errno int32) error {
	if result != -1 || errno <= 0 {
		return fmt.Errorf("inspect extended security ACL: invalid libSystem result %d (errno %d)",
			result, errno)
	}
	return fmt.Errorf("inspect extended security ACL: %w", unix.Errno(errno))
}

// libSystem returns C ints; upper register bits are not part of either value.
// Preserve the documented 0/-1 result and reject unexpected values safely.
func darwinSecurityCallResult(result, errno uintptr) (int32, int32) {
	result &= math.MaxUint32
	errno &= math.MaxUint32
	if errno > math.MaxInt32 {
		return 1, 0
	}
	if result == math.MaxUint32 {
		return -1, int32(errno)
	}
	if result > math.MaxInt32 {
		return 1, int32(errno)
	}
	return int32(result), int32(errno)
}

func checkDarwinSecurityBuffer(buffer [darwinSecurityBufferSize]byte) error {
	// sys/attr.h defines uint32 length followed by attrreference_t (int32
	// offset, uint32 length). XNU's vfs_attrlist.c packs a zero-length value
	// when no ACL exists. Any nonempty security attribute requires retention;
	// do not infer ACL absence from ordinary extended attributes.
	const referenceEnd = 12
	length := binary.LittleEndian.Uint32(buffer[:4])
	if length < referenceEnd || length > uint32(len(buffer)) {
		return fmt.Errorf("invalid extended-security attribute response")
	}
	if binary.LittleEndian.Uint32(buffer[8:referenceEnd]) != 0 {
		return fmt.Errorf("extended access ACL requires retention")
	}
	return nil
}
