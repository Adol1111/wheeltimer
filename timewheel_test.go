package wheeltimer

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type timerTask struct {
	id      int
	runTime time.Time
}

func TestTimerWheel(t *testing.T) {
	wheel, err := NewWheelTimer(time.Millisecond*1, 1024, WithMaxPendingTimeouts(1024))
	assert.NoError(t, err)

	err = wheel.Start()
	assert.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)

		r := rand.Intn(1500)
		delay := time.Millisecond * time.Duration(r+500)
		now := time.Now()

		fmt.Printf("id: %d ,add time: %v, delay: %v \n", i, now, delay.String())
		_, err := wheel.NewTimeout(NewDataTimerTask(timerTask{id: i, runTime: now.Add(delay)}, func(timeout Timeout, task timerTask) error {
			defer wg.Done()

			fmt.Printf("id: %d, error: %v\n", task.id, time.Since(task.runTime))
			return nil
		}), delay)
		assert.NoError(t, err)
	}

	wg.Wait()
}
