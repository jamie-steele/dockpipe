package main

import "time"

func timeNow() time.Time {
	return time.Now()
}

func timeNowUnixNano() int64 {
	return timeNow().UnixNano()
}
