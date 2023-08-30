//go:build windows

package main

/*
BEWARE:
crimes against memory safety ahead
*/

import (
	"time"
	"unsafe"

	_ "unsafe"

	"github.com/beevik/ntp"
	"github.com/sirupsen/logrus"
	"github.com/undefinedlabs/go-mpatch"
)

var realTimeOffset *ntp.Response

func init() {
	var err error
	realTimeOffset, err = ntp.Query("time.windows.com")
	if err != nil {
		logrus.Fatal(err)
	}
	if realTimeOffset.ClockOffset > time.Minute*30 {
		logrus.Warnf("Your Clock is off by: %s\n", realTimeOffset.ClockOffset.String())
	}

	_, err = mpatch.PatchMethod(time.Now, timeNow)
	if err != nil {
		logrus.Fatal(err)
	}
}

//go:linkname now time.now
func now() (sec int64, nsec int32, mono int64)

//go:linkname startNano time.startNano
var startNano int64

const (
	secondsPerMinute       = 60
	secondsPerHour         = 60 * secondsPerMinute
	secondsPerDay          = 24 * secondsPerHour
	unixToInternal   int64 = (1969*365 + 1969/4 - 1969/100 + 1969/400) * secondsPerDay
	wallToInternal   int64 = (1884*365 + 1884/4 - 1884/100 + 1884/400) * secondsPerDay
	minWall                = wallToInternal
	nsecShift              = 30
	hasMonotonic           = 1 << 63
)

type timet struct {
	// wall and ext encode the wall time seconds, wall time nanoseconds,
	// and optional monotonic clock reading in nanoseconds.
	//
	// From high to low bit position, wall encodes a 1-bit flag (hasMonotonic),
	// a 33-bit seconds field, and a 30-bit wall time nanoseconds field.
	// The nanoseconds field is in the range [0, 999999999].
	// If the hasMonotonic bit is 0, then the 33-bit field must be zero
	// and the full signed 64-bit wall seconds since Jan 1 year 1 is stored in ext.
	// If the hasMonotonic bit is 1, then the 33-bit field holds a 33-bit
	// unsigned wall seconds since Jan 1 year 1885, and ext holds a
	// signed 64-bit monotonic clock reading, nanoseconds since process start.
	wall uint64
	ext  int64

	// loc specifies the Location that should be used to
	// determine the minute, hour, month, day, and year
	// that correspond to this Time.
	// The nil location means UTC.
	// All UTC times are represented with loc==nil, never loc==&utcLoc.
	loc *time.Location
}

//go:noinline
func timeNow() time.Time {
	sec, nsec, mono := now()
	mono -= startNano
	sec += unixToInternal - minWall

	t := &timet{hasMonotonic | uint64(sec)<<nsecShift | uint64(nsec), mono, time.Local}
	tt := *(*time.Time)(unsafe.Pointer(t))
	return tt.Add(realTimeOffset.ClockOffset)
}
