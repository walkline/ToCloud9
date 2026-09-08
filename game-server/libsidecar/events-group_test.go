package main

import (
	"testing"
	"unsafe"

	"github.com/walkline/ToCloud9/shared/events"
)

// TestWriteMemberGUIDs checks that every member lands in its own slot and that
// nothing is written past the array. The previous implementation advanced the
// destination pointer cumulatively, writing at 0, 1, 3, 6, 10... — so a group of
// three or more shipped an uninitialized slot and wrote out of bounds.
func TestWriteMemberGUIDs(t *testing.T) {
	const guard = 0xDEAD

	for _, count := range []int{1, 2, 3, 5} {
		members := make([]events.GroupMember, count)
		for i := range members {
			members[i].MemberGUID = uint64(100 + i)
		}

		// The array the C side receives, plus guard slots: a write outside the
		// members range shows up as a changed guard.
		buf := make([]uint64, count+8)
		for i := range buf {
			buf[i] = guard
		}

		writeMemberGUIDs(unsafe.Pointer(&buf[0]), members)

		for i := 0; i < count; i++ {
			if got, want := buf[i], uint64(100+i); got != want {
				t.Fatalf("%d members: slot %d holds %d, expected %d", count, i, got, want)
			}
		}

		for i := count; i < len(buf); i++ {
			if buf[i] != guard {
				t.Fatalf("%d members: wrote past the array at slot %d", count, i)
			}
		}
	}
}
