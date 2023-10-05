package pools

import (
	"context"
	"fmt"
	"time"
)

type Pool[T any] interface {
	Feed(ctx context.Context, ch <-chan T)
	Drain(ctx context.Context, offset int) <-chan T
	Policy() Policy
}

const requestBufferSize = 5

type Policy struct {
	TTL time.Duration
}

type request[T any] struct {
	offset int
	ch     chan T
	ctx    context.Context
}

type pool[T any] struct {
	feed     chan T
	requests chan *request[T]
	waitLock chan struct{}
	policy   Policy
}

func (p pool[T]) Policy() Policy {
	return p.policy
}

func (p pool[T]) Feed(ctx context.Context, ch <-chan T) {
	go func(ch <-chan T) {
		for {
			select {
			case <-ctx.Done():
				return
			case t, ok := <-ch:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case p.feed <- t:
				}
			}
		}
	}(ch)
}

func (p pool[T]) Drain(ctx context.Context, offset int) <-chan T {
	ch := make(chan T)
	go func(out chan<- T) {
		defer close(ch)
		rq := &request[T]{
			ctx:    ctx,
			offset: offset,
		}
		p.submitRequest(rq)

		for {
			select {
			case <-ctx.Done():
				return
			case t, ok := <-rq.ch:
				if ok {
					select {
					case <-ctx.Done():
						return
					case out <- t:
						rq.offset++
					}
					continue
				}
				// request has completed, resubmit with a new channel
				rq.ch = nil
				if err := p.submitRequest(rq); err != nil {
					return
				}
			}
		}
	}(ch)
	return ch
}

func (p pool[T]) run(ctx context.Context, data *offsetData[T]) {
	defer close(p.feed)
	defer close(p.requests)
	defer func(data *offsetData[T]) {
		fmt.Printf("data has %d elements\n", data.Length())
	}(data)
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-p.feed:
			data.Append(t)
			p.releaseWaitLock()
		case rq := <-p.requests:
			if data.LengthFrom(rq.offset) == 0 {
				// nothing to send, place in waiting until something arrives.
				go p.waitAndResubmit(rq, p.getWaitLock())
				continue
			}
			go p.serviceRequest(rq, data.SliceFrom(rq.offset))
		}
	}
}

func (p pool[T]) submitRequest(r *request[T]) error {
	if r.ch == nil {
		r.ch = make(chan T, requestBufferSize)
	}
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case p.requests <- r:
		return nil
	}
}

func (p pool[T]) waitAndResubmit(r *request[T], done <-chan struct{}) error {
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case <-done:
		return p.submitRequest(r)
	case <-time.After(time.Minute):
		return p.submitRequest(r)
	}
}

func (p *pool[T]) getWaitLock() chan struct{} {
	if p.waitLock == nil {
		p.waitLock = make(chan struct{})
	}
	return p.waitLock
}

func (p *pool[T]) releaseWaitLock() {
	if p.waitLock != nil {
		close(p.waitLock)
		p.waitLock = nil
	}
}

func (p pool[T]) serviceRequest(dr *request[T], data []T) {
	defer close(dr.ch)
	for _, t := range data {
		select {
		case <-dr.ctx.Done():
			return
		case dr.ch <- t:
		}
	}
}

func NewPool[T any](ctx context.Context, data ...T) Pool[T] {
	p := &pool[T]{
		feed:     make(chan T),
		requests: make(chan *request[T]),
	}
	go p.run(ctx, &offsetData[T]{
		data:   data,
		offset: 0,
	})
	return p
}
