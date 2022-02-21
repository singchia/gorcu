package main

import (
	"fmt"
	"sync"

	"github.com/singchia/gorcu"
)

func main() {
	rcu(1, 2, 100, 1000, b)
}

func rcu(uc, rc, ut, rt int) {
	wg := new(sync.WaitGroup)
	shared := &Shared{
		Shared: 0,
	}
	rq := gorcu.NewRCUQsbr(uint32(rc))
	// update goroutines
	go func() {
		for i := 0; i < uc; i++ {
			wg.Add(1)
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
			fmt.Println("update finished")
		}
	}()
	// read goroutines
	go func() {
		for i := 0; i < rc; i++ {
			wg.Add(1)
			go func(id uint32) {
				defer wg.Done()
				rq.Register(id)
				index := 0
				for index < rt {
					index++
					rq.Online(id)
					// TODO
					tmp := shared
					rq.Quiescent(id)
					rq.Offline(id)
				}
				rq.UnRegister(id)
			}(uint32(i))
			fmt.Println("read finished")
		}
	}()
	wg.Wait()
}
