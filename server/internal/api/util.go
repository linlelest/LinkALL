// 共用 helper
package api

import "time"

func nowUnix() int64 {
	return time.Now().Unix()
}
