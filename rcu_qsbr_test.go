package gorcu

import (
	"math/rand"
	"os"
	"os/signal"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	count := uint32(3)
	rq := NewRCUQsbr(count)
	for i := uint32(0); i < count; i++ {
		id := i
		go func() {
			for {
				err := rq.Register(id)
				if err != nil {
					t.Error(err)
				}
				r := rand.Intn(10)
				time.Sleep(time.Duration(r) * time.Microsecond)
				err = rq.Unregister(id)
				if err != nil {
					t.Error(err)
				}
				r = rand.Intn(10)
				time.Sleep(time.Duration(r) * time.Microsecond)
			}
		}()
	}

	sgCh := make(chan os.Signal, 1)
	signal.Notify(sgCh, os.Interrupt)
	<-sgCh
}
