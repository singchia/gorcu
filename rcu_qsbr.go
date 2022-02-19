package gorcu

import (
	"unsafe"
)

const (
	qsbrCntInit uint64 = 1
)

type RCUQsbr struct {
	token         uint64
	ackedToken    uint64
	numElems      uint32           // number of elements needed
	numGoroutines uint32           // number of goroutines currently
	maxGoroutines uint32           // maximum number of goroutines
	cnts          []unsafe.Pointer // []*RCUQsbrCnt
	bitmap        []uint64
}

type RCUQsbrCnt struct {
	qsbrCnt uint64
}

func NewRcuQsbr(maxGoroutines uint32) *RCUQsbr {
	rq := &RCUQsbr{}
	rq.maxGoroutines = maxGoroutines
	rq.numElems = ((maxGoroutines + 64 - 1) / 64) * 64
	rq.token = qsbrCntInit
	rq.ackedToken = qsbrCntInit - 1
	rq.cnts = make([]unsafe.Pointer, maxGoroutines)
	rq.bitmap = make([]uint64, rq.numElems)
	return rq
}

func (rq *RCUQsbr) Register(id int) error {
	return nil
}

func (rq *RCUQsbr) Unregister(id int) error {
	return nil
}
