package utils

import (
	"sync"
	"time"
)

var (
	LastSneakTimeMu   sync.RWMutex
	LastSneakTimeMap  = make(map[int64]time.Time)

	LastProtectTimeMu  sync.RWMutex
	LastProtectTimeMap = make(map[int64]time.Time)

	LastTreatTimeMu   sync.RWMutex
	LastTreatTimeMap  = make(map[int64]time.Time)

	AttackerMu   sync.RWMutex
	AttackerMap  = make(map[int64]int64)
)

func SetLastSneakTime(userID int64, t time.Time) {
	LastSneakTimeMu.Lock()
	LastSneakTimeMap[userID] = t
	LastSneakTimeMu.Unlock()
}

func GetLastSneakTime(userID int64) time.Time {
	LastSneakTimeMu.RLock()
	defer LastSneakTimeMu.RUnlock()
	return LastSneakTimeMap[userID]
}

func SetLastProtectTime(userID int64, t time.Time) {
	LastProtectTimeMu.Lock()
	LastProtectTimeMap[userID] = t
	LastProtectTimeMu.Unlock()
}

func GetLastProtectTime(userID int64) time.Time {
	LastProtectTimeMu.RLock()
	defer LastProtectTimeMu.RUnlock()
	return LastProtectTimeMap[userID]
}

func SetLastTreatTime(userID int64, t time.Time) {
	LastTreatTimeMu.Lock()
	LastTreatTimeMap[userID] = t
	LastTreatTimeMu.Unlock()
}

func GetLastTreatTime(userID int64) time.Time {
	LastTreatTimeMu.RLock()
	defer LastTreatTimeMu.RUnlock()
	return LastTreatTimeMap[userID]
}

func SetAttackerID(userID int64, attackerID int64) {
	AttackerMu.Lock()
	AttackerMap[userID] = attackerID
	AttackerMu.Unlock()
}

func GetAttackerID(userID int64) int64 {
	AttackerMu.RLock()
	defer AttackerMu.RUnlock()
	return AttackerMap[userID]
}
