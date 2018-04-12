package logmgr

import (
	"time"
)

var globalLogMgr *LogMgr

func InitGlobalLogMgr(logBaseDir string, idleDuration time.Duration) error {
	lm, err := NewLogMgr(logBaseDir, idleDuration)
	if err != nil {
		return err
	}

	globalLogMgr = lm
	return nil
}

func L() *LogMgr {
	return globalLogMgr
}
