// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package evidencecleanup

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestDarwinSecurityAPI_ReadSecurity(t *testing.T) {
	for _, test := range []struct {
		name   string
		fd     int
		result int32
		errno  int32
		nilErr bool
		want   error
		reason string
	}{
		{name: "success ignores stale errno", fd: 42, errno: int32(unix.EACCES)},
		{name: "negative descriptor", fd: -1, want: unix.EBADF},
		{name: "descriptor overflow", fd: math.MaxInt32 + 1, want: unix.EBADF},
		{name: "access denied", fd: 42, result: -1, errno: int32(unix.EACCES), want: unix.EACCES},
		{name: "unsupported", fd: 42, result: -1, errno: int32(unix.ENOTSUP), want: unix.ENOTSUP},
		{name: "bad descriptor", fd: math.MaxInt32, result: -1,
			errno: int32(unix.EBADF), want: unix.EBADF},
		{name: "missing errno storage", fd: 42, nilErr: true, reason: "unavailable errno storage"},
		{name: "missing error code", fd: 42, result: -1, reason: "invalid libSystem result"},
		{name: "negative error code", fd: 42, result: -1, errno: -1, reason: "invalid libSystem result"},
		{name: "invalid result", fd: 42, result: 1, errno: int32(unix.EACCES),
			reason: "invalid libSystem result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls, errnoCalls int
			api := darwinSecurityAPI{
				errno: func() *int32 {
					errnoCalls++
					if test.nilErr {
						return nil
					}
					return &test.errno
				},
				read: func(fd int32, buffer *[darwinSecurityBufferSize]byte) int32 {
					calls++
					assert.EqualValues(t, test.fd, fd)
					assert.Equal(t, 1, errnoCalls, "errno address must be fetched before the call")
					assert.Equal(t, [darwinSecurityBufferSize]byte{}, *buffer)
					binary.LittleEndian.PutUint32(buffer[:4], 12)
					return test.result
				},
			}
			buffer, err := api.readSecurity(test.fd)
			switch {
			case test.want != nil:
				require.ErrorIs(t, err, test.want)
			case test.reason != "":
				require.ErrorContains(t, err, test.reason)
			default:
				require.NoError(t, err)
				require.NoError(t, checkDarwinSecurityBuffer(buffer))
			}
			if test.fd < 0 || test.fd > math.MaxInt32 {
				assert.Zero(t, errnoCalls)
				assert.Zero(t, calls)
			} else if test.nilErr {
				assert.Equal(t, 1, errnoCalls)
				assert.Zero(t, calls)
			} else {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestCheckDarwinSecurityBuffer_RefusesACLAndInvalidResponses(t *testing.T) {
	for _, test := range []struct {
		name   string
		length uint32
		acl    uint32
		reason string
	}{
		{"no ACL", 12, 0, ""},
		{"empty response", 0, 0, "invalid extended-security"},
		{"short reference", 11, 0, "invalid extended-security"},
		{"truncated response", darwinSecurityBufferSize + 1, 0, "invalid extended-security"},
		{"ACL present", 64, 52, "ACL requires retention"},
		{"oversized ACL", 12, math.MaxUint32, "ACL requires retention"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var buffer [darwinSecurityBufferSize]byte
			binary.LittleEndian.PutUint32(buffer[:4], test.length)
			binary.LittleEndian.PutUint32(buffer[8:12], test.acl)
			err := checkDarwinSecurityBuffer(buffer)
			if test.reason == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.reason)
			}
		})
	}
}
