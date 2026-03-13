package handlers

import (
	"context"
	"log"
	"time"
)

type capabilityDispatchRequest struct {
	ctx      context.Context
	interval time.Duration
	ch       chan struct{}
}

type CapabilityTestDispatcher struct {
	queue chan capabilityDispatchRequest
}

var capabilityTestDispatcher = newCapabilityTestDispatcher()

func newCapabilityTestDispatcher() *CapabilityTestDispatcher {
	d := &CapabilityTestDispatcher{
		queue: make(chan capabilityDispatchRequest, 4096),
	}
	go d.run()
	return d
}

func GetCapabilityTestDispatcher() *CapabilityTestDispatcher {
	return capabilityTestDispatcher
}

func (d *CapabilityTestDispatcher) AcquireSendSlot(ctx context.Context, interval time.Duration) error {
	readyCh := make(chan struct{}, 1)
	request := capabilityDispatchRequest{ctx: ctx, interval: interval, ch: readyCh}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case d.queue <- request:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-readyCh:
		return nil
	}
}

func (d *CapabilityTestDispatcher) run() {
	nextAvailable := time.Now()

	for {
		request := <-d.queue
		for request.ctx.Err() != nil {
			request = <-d.queue
		}

		now := time.Now()
		if wait := nextAvailable.Sub(now); wait > 0 {
			time.Sleep(wait)
		}

		request.ch <- struct{}{}

		interval := request.interval
		if interval <= 0 {
			interval = time.Minute / 10
		}
		log.Printf("[CapabilityTest-Dispatch] 放行一个能力测试请求，间隔=%s", interval)
		nextAvailable = time.Now().Add(interval)
	}
}
