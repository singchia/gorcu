package gorcu

import "unsafe"

type RCUQsbr struct {
	token, ackedToken uint64
	numGoroutines     int
	cnts              []unsafe.Pointer
}

type RCUQsbrCnt struct {
	qsbrCnt uint64
}

func RcuQsbrInit(rq *RCUQsbr, maxGoroutines int) error {
	return nil
}
