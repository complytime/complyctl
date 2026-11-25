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
	read  func(int32, *[darwinSecurityBufferSize]byte) int32
	errno func() *int32
}

func (api darwinSecurityAPI) readSecurity(fd int) ([darwinSecurityBufferSize]byte, error) {
	var buffer [darwinSecurityBufferSize]byte
	if fd < 0 || fd > math.MaxInt32 {
		return buffer, fmt.Errorf("inspect extended security ACL: %w", unix.EBADF)
	}
	// errno is thread-local. Fetch its address and copy its value on the same
	// thread as fgetattrlist, before any other foreign call can overwrite it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	errno := api.errno()
	if errno == nil {
		return buffer, fmt.Errorf("inspect extended security ACL: unavailable errno storage")
	}
	result := api.read(int32(fd), &buffer)
	runtime.KeepAlive(&buffer)
	if result != 0 {
		return buffer, darwinSecurityCallError(result, *errno)
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
