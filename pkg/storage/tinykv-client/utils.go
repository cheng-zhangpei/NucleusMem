package tinykv_client

import "time"

func getCommitTS() uint64 {
	// todo(Cheng) owing to the scale of the MVP,We do not support the TSO to get the time
	return uint64(time.Now().UnixNano())
}
