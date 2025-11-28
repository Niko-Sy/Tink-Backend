package utils

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type IdGenerator struct {
}

var (
	randMutex sync.Mutex
	randGen   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// GenerateTimestamp 生成当前时间戳
func GenerateTimestamp() int64 {
	return time.Now().UnixNano()
}

// GenerateChatRoomID 生成9位数字的聊天室ID
func GenerateChatRoomID() string {
	randMutex.Lock()
	defer randMutex.Unlock()

	// 生成1-999999999之间的随机数，确保是9位数字
	id := randGen.Intn(999999999) + 1
	return fmt.Sprintf("%09d", id)
}

// GenerateMemberID 生成成员ID (M+9位随机数)
func GenerateMemberID(userID, roomID string) string {
	randMutex.Lock()
	defer randMutex.Unlock()

	// 生成1-999999999之间的随机数，确保是9位数字
	id := randGen.Intn(999999999) + 1
	return fmt.Sprintf("M%09d", id)
}

// GenerateUserID 生成用户ID (U+9位数字)
func GenerateUserID() string {
	randMutex.Lock()
	defer randMutex.Unlock()

	// 生成1-999999999之间的随机数，确保是9位数字
	id := randGen.Intn(999999999) + 1
	return fmt.Sprintf("U%09d", id)
}

// GenerateMessageID 生成12位数字的消息ID（使用时间戳）
func GenerateMessageID() string {
	// 使用纳秒时间戳，确保唯一性
	timestamp := time.Now().UnixNano()
	// 取时间戳的后12位，如果不足12位则在前面补0
	return fmt.Sprintf("%012d", timestamp%1000000000000)
}

// GenerateNotificationID 生成通知ID (N+12位时间戳)
func GenerateNotificationID() string {
	// 使用纳秒时间戳，确保唯一性
	timestamp := time.Now().UnixNano()
	// 取时间戳的后12位，如果不足12位则在前面补0
	return fmt.Sprintf("N%012d", timestamp%1000000000000)
}
