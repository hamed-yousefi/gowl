/**
 * Copyright © 2019 Hamed Yousefi <hdyousefi@gmail.com.com>.
 *
 * Use of this source code is governed by an MIT-style
 * license that can be found in the LICENSE file.
 *
 * Created by IntelliJ IDEA.
 * User: Hamed Yousefi
 * Email: hdyousefi@gmail.com
 * Date: 4/12/21
 * Time: 12:45 PM
 *
 * Description:
 *
 */

package gowl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hamed-yousefi/gowl/status/pool"
	"github.com/hamed-yousefi/gowl/status/process"
)

type (
	pTestFunc   func(ctx context.Context, pid PID, duration time.Duration) error
	mockProcess struct {
		name      string
		pid       PID
		sleepTime time.Duration
		pFunc     pTestFunc
	}
)

func (t mockProcess) Start(ctx context.Context) error {
	return t.pFunc(ctx, t.pid, t.sleepTime)
}

func (t mockProcess) Name() string {
	return t.name
}

func (t mockProcess) PID() PID {
	return t.pid
}

func newTestProcess(name string, id int, duration time.Duration, f pTestFunc) Process {
	return mockProcess{
		name:      name,
		pid:       PID("p-" + strconv.Itoa(id)),
		sleepTime: duration,
		pFunc:     f,
	}
}

var errCancelled = errors.New("task was cancelled")

// Close pool before adding all processes to the queue
func TestNewPool(t *testing.T) {
	a := assert.New(t)
	wp := NewPool(2)

	a.Equal(pool.Created, wp.Monitor().PoolStatus())
	wp.Register(createProcess(10, 1, 300*time.Millisecond, processFunc)...)
	err := wp.Start()
	a.NoError(err)
	a.Equal(pool.Running, wp.Monitor().PoolStatus())
	time.Sleep(500 * time.Millisecond)
	err = wp.Close()
	a.NoError(err)
	a.Equal(pool.Closed, wp.Monitor().PoolStatus())
}

// Four different goroutine will publish processes to the queue
func TestNewPoolMultiPublisher(t *testing.T) {
	a := assert.New(t)
	wp := NewPool(2)
	a.Equal(pool.Created, wp.Monitor().PoolStatus())
	err := wp.Start()
	a.NoError(err)
	a.Equal(pool.Running, wp.Monitor().PoolStatus())
	wp.Register(createProcess(10, 1, 300*time.Millisecond, processFunc)...)
	wp.Register(createProcess(10, 2, 200*time.Millisecond, processFunc)...)
	wp.Register(createProcess(10, 3, 100*time.Millisecond, processFunc)...)
	wp.Register(createProcess(10, 4, 500*time.Millisecond, processFunc)...)

	time.Sleep(10 * time.Second)
	err = wp.Close()
	a.NoError(err)
	a.Equal(pool.Closed, wp.Monitor().PoolStatus())
}

// Kill a processFunc before it starts
func TestWorkerPool_Kill(t *testing.T) {
	a := assert.New(t)
	wp := NewPool(5)
	a.Equal(pool.Created, wp.Monitor().PoolStatus())
	err := wp.Start()
	a.NoError(err)
	a.Equal(pool.Running, wp.Monitor().PoolStatus())
	wp.Register(createProcess(10, 1, 3*time.Second, processFunc)...)
	wp.Kill("p-18")
	time.Sleep(7 * time.Second)
	err = wp.Close()
	a.NoError(err)
	a.Equal(pool.Closed, wp.Monitor().PoolStatus())
	a.Equal(process.Killed, wp.Monitor().ProcessStats("p-18").Status)
}

// Kill a processFunc after it started
func TestWorkerPoolStarted_Kill(t *testing.T) {
	a := assert.New(t)
	wp := NewPool(3)
	a.Equal(pool.Created, wp.Monitor().PoolStatus())
	err := wp.Start()
	a.NoError(err)
	a.Equal(pool.Running, wp.Monitor().PoolStatus())
	wp.Register(createProcess(3, 1, 3*time.Second, processFunc)...)
	time.Sleep(2 * time.Second)
	wp.Kill("p-12")
	err = wp.Close()
	a.NoError(err)
	a.Equal(pool.Closed, wp.Monitor().PoolStatus())
	a.Equal(process.Killed, wp.Monitor().ProcessStats("p-12").Status)
	a.Error(wp.Monitor().Error("p-12"))
	a.Equal("task was cancelled", wp.Monitor().Error("p-12").Error())
}

// Process returns error and monitor should cache it
func TestMonitor_Error(t *testing.T) {
	a := assert.New(t)
	wp := NewPool(5)
	a.Equal(pool.Created, wp.Monitor().PoolStatus())
	err := wp.Start()
	a.NoError(err)
	a.Equal(pool.Running, wp.Monitor().PoolStatus())
	wp.Register(createProcess(1, 1, 1*time.Second, processFuncWithError)...)
	time.Sleep(2 * time.Second)
	err = wp.Close()
	a.NoError(err)
	a.Equal(pool.Closed, wp.Monitor().PoolStatus())
	a.Equal(process.Failed, wp.Monitor().ProcessStats("p-11").Status)
	a.Error(wp.Monitor().Error("p-11"))
	a.Equal("unable to start processFunc with id: p-11", wp.Monitor().Error("p-11").Error())
}

// Close a created pool should return error
func TestWorkerPool_Close(t *testing.T) {
	a := assert.New(t)
	wp := NewPool(3)
	a.Equal(pool.Created, wp.Monitor().PoolStatus())
	err := wp.Close()
	a.Error(err)
	a.Equal("pool is not running, status "+wp.Monitor().PoolStatus().String(), err.Error())
	err = wp.Start()
	a.NoError(err)
	a.Equal(pool.Running, wp.Monitor().PoolStatus())
	wp.Register(createProcess(1, 1, 100*time.Millisecond, processFunc)...)
	time.Sleep(1 * time.Second)
	err = wp.Close()
	a.NoError(err)
	a.Equal(pool.Closed, wp.Monitor().PoolStatus())
}

// Get worker list and check their status
func TestWorkerPool_WorkerList(t *testing.T) {
	a := assert.New(t)
	wp := NewPool(3)
	a.Equal(pool.Created, wp.Monitor().PoolStatus())
	err := wp.Close()
	a.Error(err)
	a.Equal("pool is not running, status "+wp.Monitor().PoolStatus().String(), err.Error())
	err = wp.Start()
	a.NoError(err)
	a.Equal(pool.Running, wp.Monitor().PoolStatus())
	wp.Register(createProcess(5, 1, 700*time.Millisecond, processFunc)...)
	time.Sleep(1 * time.Second)
	err = wp.Start()
	a.Error(err)
	a.Equal("unable to start the pool, status: "+pool.Running.String(), err.Error())
	wList := wp.Monitor().WorkerList()
	for _, wn := range wList {
		fmt.Println(wp.Monitor().WorkerStatus(wn))
	}
	err = wp.Close()
	a.NoError(err)
	a.Equal(pool.Closed, wp.Monitor().PoolStatus())
}

func createProcess(n int, g int, d time.Duration, f pTestFunc) []Process {
	pList := make([]Process, 0)
	for i := 1; i <= n; i++ {
		pList = append(pList, newTestProcess("p-"+strconv.Itoa(i), (g*10)+i, d, f))
	}
	return pList
}

func processFunc(ctx context.Context, pid PID, d time.Duration) error {
	fmt.Printf("process with id %v has been started.\n", pid)
	select {
	case <-time.After(d):
	case <-ctx.Done():
		return errCancelled
	}
	return nil
}

func processFuncWithError(ctx context.Context, pid PID, d time.Duration) error {
	return errors.New("unable to start processFunc with id: " + pid.String())
}
