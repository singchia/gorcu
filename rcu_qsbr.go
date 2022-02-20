package gorcu

import (
	"sync/atomic"
	"unsafe"
)

const (
	qsbrCntInit uint64 = 1

	qsbrGoroutineIndexShift = 6
	qsbrGoroutineMask       = 0x3f
	qsbrGoroutineInvalid    = 0xffffffff
)

type RCUQsbr struct {
	version       uint64
	ackedVersion  uint64
	numElems      uint32           // number of elements needed
	numGoroutines uint32           // number of goroutines currently
	maxGoroutines uint32           // maximum number of goroutines
	cnts          []unsafe.Pointer // []*RCUQsbrCnt
	bitmap        []uint64
}

type RCUQsbrCnt struct {
	qsbrCnt uint64
}

func NewRCUQsbr(maxGoroutines uint32) *RCUQsbr {
	rq := &RCUQsbr{}
	rq.maxGoroutines = maxGoroutines
	rq.numElems = ((maxGoroutines + 64 - 1) / 64) * 64
	rq.version = qsbrCntInit
	rq.ackedToken = qsbrCntInit - 1
	rq.cnts = make([]unsafe.Pointer, maxGoroutines)
	rq.bitmap = make([]uint64, rq.numElems)
	return rq
}

/*
 * Register a goroutine to report its quiescent state
 */
func (rq *RCUQsbr) Register(id uint32) error {
	if id >= rq.maxGoroutines {
		return ErrIdExceedsMax
	}
	offset := id & qsbrGoroutineMask
	index := id >> qsbrGoroutineIndexShift

	oldMap := atomic.LoadUint64(&rq.bitmap[index])
	if oldMap&(1<<offset) == (1 << offset) {
		return nil
	}

	swapped := false
	for !swapped {
		newMap := oldMap | (1 << offset)
		swapped = atomic.CompareAndSwapUint64(&rq.bitmap[index], oldMap, newMap)
		if swapped {
			atomic.AddUint32(&rq.numGoroutines, 1)
		} else {
			oldMap = atomic.LoadUint64(&rq.bitmap[index])
			if oldMap&(1<<offset) == 1<<offset {
				return nil
			}
		}
	}
	return nil
}

/*
 * Remove a goroutine from the bitmap
 */
func (rq *RCUQsbr) UnRegister(id uint32) error {
	if id >= rq.maxGoroutines {
		return ErrIdExceedsMax
	}
	offset := id & qsbrGoroutineMask
	index := id >> qsbrGoroutineIndexShift
	oldMap := atomic.LoadUint64(&rq.bitmap[index])
	if !(oldMap&(1<<offset) == 1<<offset) {
		return nil
	}

	swapped := false
	for !swapped {
		newMap := oldMap & ^(1 << offset)
		swapped = atomic.CompareAndSwapUint64(&rq.bitmap[index], oldMap, newMap)
		if swapped {
			sub := int32(-1)
			usub := uint32(sub)
			atomic.AddUint32(&rq.numGoroutines, usub)
		} else {
			oldMap = atomic.LoadUint64(&rq.bitmap[index])
			if !(oldMap&(1<<offset) == 1<<offset) {
				return nil
			}
		}
	}
	return nil
}

func (rq *RCUQsbr) Update() uint64 {
	return atomic.AddUint64(&rq.version) - 1
}

func (rq *RCUQsbr) Quiescent() bool {
	return false
}

func (rq *RCUQsbr) WaitQuiescent() {
}

func (rq *RCUQsbr) NumGoroutines() uint32 {
	return atomic.LoadUint32(&rq.numGoroutines)
}
