package utils

import "time"

func GetTimeMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
