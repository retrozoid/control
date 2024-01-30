package control

import (
	"sync"

	"github.com/ecwid/control/protocol/common"
)

type syncFrames struct {
	sync.RWMutex
	value map[common.FrameId]string
}

func (fr *syncFrames) Get(frameID common.FrameId) string {
	fr.RLock()
	defer fr.RUnlock()
	return fr.value[frameID]
}

func (fr *syncFrames) Set(frameID common.FrameId, contextID string) {
	fr.Lock()
	fr.value[frameID] = contextID
	fr.Unlock()
}

func (fr *syncFrames) Remove(frameID common.FrameId) {
	fr.Lock()
	delete(fr.value, frameID)
	fr.Unlock()
}
