package utils

import (
	"encoding/hex"
	"fmt"
	"sync"
)

var (
	userLocks sync.Map // 键: string ("group_user"), 值: *sync.Mutex
)

// GetLock 获取指定 key 的并发互斥锁
// GetLock 获取指定 key 的并发互斥锁
func GetLock(key string) *sync.Mutex {
	actual, _ := userLocks.LoadOrStore(key, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// GetUserLockKey 生成针对具体群组内具体用户的锁标识
func GetUserLockKey(groupID, userID int64) string {
	return fmt.Sprintf("%d_%d", groupID, userID)
}

// Emoji 将十六进制字符串转为 UTF-8 emoji 字符
func Emoji(hexStr string) string {
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return ""
	}
	return string(b)
}

