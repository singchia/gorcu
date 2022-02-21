package gorcu

import (
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"testing"
	"time"
)

type Logger interface {
	Log(args ...interface{})
}

type Shared struct {
	Shared int
}

func TestSingleUSingleR(t *testing.T) {
	rcu(1, 1, 10000, 10000, t)
}

func rcu(uc, rc, ut, rt int, logger Logger) {
	wg := new(sync.WaitGroup)
	shared := &Shared{
		Shared: 0,
	}
	rq := NewRCUQsbr(uint32(rc))
	// update goroutines
	wg.Add(uc)
	go func() {
		for i := 0; i < uc; i++ {
			go func() {
				defer wg.Done()
				index := 0
				for index < ut {
					index++
					version := rq.Update()
					rq.Check(version, true)
					shared = &Shared{
						Shared: index,
					}
				}
			}()
		}
	}()
	// read goroutines
	wg.Add(rc)
	go func() {
		for i := 0; i < rc; i++ {
			go func(id uint32) {
				defer wg.Done()
				rq.Register(id)
				index := 0
				for index < rt {
					index++
					rq.Online(id)
					// TODO
					tmp := shared
					logger.Log(tmp.Shared)
					rq.Quiescent(id)
					rq.Offline(id)
				}
				rq.UnRegister(id)
			}(uint32(i))
		}
	}()
	wg.Wait()
}

func rwmux(uc, rc, ut, rt int, logger Logger) {
	rwmux := new(sync.RWMutex)
	wg := new(sync.WaitGroup)
	shared := &Shared{
		Shared: 0,
	}
	// update goroutines
	wg.Add(uc)
	go func() {
		for i := 0; i < uc; i++ {
			go func() {
				defer wg.Done()
				index := 0
				for index < ut {
					index++
					rwmux.Lock()
					shared = &Shared{
						Shared: index,
					}
					rwmux.Unlock()
				}
			}()
		}
	}()

	// read goroutines
	wg.Add(rc)
	go func() {
		for i := 0; i < rc; i++ {
			go func(id uint32) {
				defer wg.Done()
				index := 0
				for index < rt {
					index++
					rwmux.RLock()
					// TODO
					tmp := shared
					logger.Log(tmp.Shared)
					rwmux.RUnlock()
				}
			}(uint32(i))
		}
	}()
	wg.Wait()
}

func BenchmarkCompare(b *testing.B) {
	rcuShared := &Shared{
		Shared: 0,
	}
	rq := NewRCUQsbr(5)
	rcuUpdate := func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			version := rq.Update()
			rq.Check(version, true)
			time.Sleep(time.Millisecond)
			rcuShared = &Shared{
				Shared: i,
			}
		}
	}
	rcuReadFactory := func(id uint32) func(b *testing.B) {
		return func(b *testing.B) {
			rq.Register(id)
			for i := 0; i < b.N; i++ {
				rq.Online(id)
				tmp := rcuShared
				b.Log(tmp.Shared)
				rq.Quiescent(id)
				rq.Offline(id)
			}
			rq.UnRegister(id)
		}
	}
	go rcuUpdate(b)
	b.Run("rcu_read_1", rcuReadFactory(1))
	go rcuUpdate(b)
	b.Run("rcu_read_2", rcuReadFactory(2))
	go rcuUpdate(b)
	b.Run("rcu_read_3", rcuReadFactory(3))
	go rcuUpdate(b)
	b.Run("rcu_read_4", rcuReadFactory(4))
	go rcuUpdate(b)
	b.Run("rcu_read_5", rcuReadFactory(5))

	rwShared := &Shared{
		Shared: 0,
	}
	rwmux := new(sync.RWMutex)
	rwUpdate := func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rwmux.Lock()
			time.Sleep(time.Millisecond)
			rcuShared = &Shared{
				Shared: i,
			}
			rwmux.Unlock()
		}
	}
	rwRead := func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rwmux.RLock()
			b.Log(rwShared.Shared)
			rwmux.RUnlock()
		}
	}
	go rwUpdate(b)
	b.Run("rw_read_1", rwRead)
	go rwUpdate(b)
	b.Run("rw_read_2", rwRead)
	go rwUpdate(b)
	b.Run("rw_read_3", rwRead)
	go rwUpdate(b)
	b.Run("rw_read_4", rwRead)
	go rwUpdate(b)
	b.Run("rw_read_5", rwRead)
}

func BenchmarkRCU(b *testing.B) {
	rcu(2, 10, b.N, b.N, b)
}

func BenchmarkRWMux(b *testing.B) {
	rwmux(2, 10, b.N, b.N, b)
}

func TestRegister(t *testing.T) {
	count := uint32(3)
	rq := NewRCUQsbr(count)
	for i := uint32(0); i < count; i++ {
		go func(id uint32) {
			for {
				err := rq.Register(id)
				if err != nil {
					t.Error(err)
				}
				r := rand.Intn(10)
				time.Sleep(time.Duration(r) * time.Microsecond)
				err = rq.UnRegister(id)
				if err != nil {
					t.Error(err)
				}
				r = rand.Intn(10)
				time.Sleep(time.Duration(r) * time.Microsecond)
			}
		}(i)
	}

	sgCh := make(chan os.Signal, 1)
	signal.Notify(sgCh, os.Interrupt)
	<-sgCh
}
