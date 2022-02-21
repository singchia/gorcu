package gorcu

import (
	"math/bits"
	"runtime"
	"sync/atomic"
)

const (
	qsbrVersionInit uint64 = 1

	qsbrGoroutineIndexShift = 6
	qsbrGoroutineMask       = 0x3f
	qsbrGoroutineInvalid    = 0xffffffff
	qsbrVersionMax          = ^uint64(0)
	qsbrVersionOffline      = uint64(0)
)

type RCUQsbr struct {
	version       uint64
	ackedVersion  uint64
	numElems      uint32 // number of elements needed
	numGoroutines uint32 // number of goroutines currently
	maxGoroutines uint32 // maximum number of goroutines
	ctxs          []rcuQsbrCtx
	bitmap        []uint64
}

type rcuQsbrCtx struct {
	version uint64
}

func NewRCUQsbr(maxGoroutines uint32) *RCUQsbr {
	rq := &RCUQsbr{}
	rq.maxGoroutines = maxGoroutines
	rq.numElems = ((maxGoroutines + 64 - 1) / 64) * 64 // ceil division
	rq.version = qsbrVersionInit
	rq.ackedVersion = qsbrVersionInit - 1
	rq.ctxs = make([]rcuQsbrCtx, maxGoroutines)
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

	oldBmap := atomic.LoadUint64(&rq.bitmap[index])
	if oldBmap&(1<<offset) == (1 << offset) {
		return nil
	}

	swapped := false
	for !swapped {
		newBmap := oldBmap | (1 << offset)
		swapped = atomic.CompareAndSwapUint64(&rq.bitmap[index], oldBmap, newBmap)
		if swapped {
			atomic.AddUint32(&rq.numGoroutines, 1)
		} else {
			oldBmap = atomic.LoadUint64(&rq.bitmap[index])
			if oldBmap&(1<<offset) == 1<<offset {
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
	oldBmap := atomic.LoadUint64(&rq.bitmap[index])
	if !(oldBmap&(1<<offset) == 1<<offset) {
		return nil
	}

	swapped := false
	for !swapped {
		newBmap := oldBmap & ^(1 << offset)
		swapped = atomic.CompareAndSwapUint64(&rq.bitmap[index], oldBmap, newBmap)
		if swapped {
			sub := int32(-1)
			usub := uint32(sub)
			atomic.AddUint32(&rq.numGoroutines, usub)
		} else {
			oldBmap = atomic.LoadUint64(&rq.bitmap[index])
			if !(oldBmap&(1<<offset) == 1<<offset) {
				return nil
			}
		}
	}
	return nil
}

func (rq *RCUQsbr) Online(id uint32) {
	if id >= rq.maxGoroutines {
		return
	}
	v := atomic.LoadUint64(&rq.version)
	atomic.StoreUint64(&rq.ctxs[id].version, v)
}

func (rq *RCUQsbr) Offline(id uint32) {
	if id >= rq.maxGoroutines {
		return
	}
	atomic.StoreUint64(&rq.ctxs[id].version, qsbrVersionOffline)
}

func (rq *RCUQsbr) Quiescent(id uint32) {
	if id >= rq.maxGoroutines {
		return
	}
	v := atomic.LoadUint64(&rq.version)
	vId := atomic.LoadUint64(&rq.ctxs[id].version)
	if v != vId {
		atomic.StoreUint64(&rq.ctxs[id].version, v)
	}
}

func (rq *RCUQsbr) Update() uint64 {
	return atomic.AddUint64(&rq.version, 1) - 1
}

func (rq *RCUQsbr) Check(version uint64, wait bool) bool {
	if version <= rq.ackedVersion {
		return true
	}
	if rq.numGoroutines == rq.maxGoroutines {
		return rq.checkAll(version, wait)
	} else {
		return rq.checkPartial(version, wait)
	}
}

/*
 * check the quiescent state for all goroutines
 */
func (rq *RCUQsbr) checkAll(version uint64, wait bool) bool {
	ackedVersion := qsbrVersionMax
	v := qsbrVersionOffline
	for i := uint32(0); i < rq.maxGoroutines; i++ {
		for {
			v = atomic.LoadUint64(&rq.ctxs[i].version)
			if v == qsbrVersionOffline || v >= version {
				break
			}
			if !wait {
				return false
			}
			// yeild the processor
			runtime.Gosched()
		}
		if v != qsbrVersionOffline && ackedVersion > v {
			ackedVersion = v
		}
	}
	if ackedVersion != qsbrVersionMax {
		atomic.StoreUint64(&rq.ackedVersion, ackedVersion)
	}
	return true
}

func (rq *RCUQsbr) checkPartial(version uint64, wait bool) bool {
	ackedVersion := qsbrVersionMax
	for i := uint32(0); i < rq.numElems; i++ {
		bmap := atomic.LoadUint64(&rq.bitmap[i])
		// original id offset
		id := i << qsbrGoroutineIndexShift

		for bmap > 0 {
			offset := uint32(bits.TrailingZeros64(bmap))
			v := atomic.LoadUint64(&rq.ctxs[id+offset].version)
			if v != qsbrVersionOffline && v < version {
				// still not entered quiescent state
				if !wait {
					return false
				}
				// yeild the processor
				runtime.Gosched()
				bmap = atomic.LoadUint64(&rq.bitmap[i])
				continue
			}
			if v != qsbrVersionOffline && ackedVersion > v {
				ackedVersion = v
			}

			bmap &= ^(1 << offset)
		}
	}
	if ackedVersion != qsbrVersionMax {
		atomic.StoreUint64(&rq.ackedVersion, ackedVersion)
	}
	return true
}

func (rq *RCUQsbr) NumGoroutines() uint32 {
	return atomic.LoadUint32(&rq.numGoroutines)
}
