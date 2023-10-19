package pools

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// Pool represents an active slice of data which can be read and appended to by multiple, concurrent users.
// It uses a non-blocking model, such that no client reader or writer can block the process of any other process.
// e.g. if a Reader blocks its receiving channel, the other Readers will still continue to read.
// The content data's life cycle is governed by a 'Policy' which defines both how long to keep items and/or how many to keep.
// The contents are zero based indexed, like a reqular slice, however, as the Pool Policy dictates, early indexes
// may be removed and become unavailable to any future readers.
// A Reader must ask for the starting index and if that index is no longer in the pool,
type Pool[T any] interface {
	Policy() Policy
	Feed(ctx context.Context, ch <-chan T) <-chan struct{}
	Read(ctx context.Context, offset int) <-chan T
	WaitForClose()
}

type pool[T any] struct {
	feed chan T
	done chan struct{}

	requests      chan request[T]
	waitLock      chan struct{}
	waitLockMutex sync.Mutex

	policy Policy
}

func (p pool[T]) WaitForClose() {
	<-p.done
}

func (p pool[T]) Policy() Policy {
	return p.policy
}

func (p pool[T]) Feed(ctx context.Context, ch <-chan T) <-chan struct{} {
	go func(ch <-chan T) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.done:
				return
			case t, ok := <-ch:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-p.done:
					return
				case p.feed <- t:
				}
			}
		}
	}(ch)
	return p.done
}

func (p pool[T]) Read(ctx context.Context, offset int) <-chan T {
	ch := make(chan T)
	go func(out chan<- T) {
		defer close(out)

		errs := make(chan error)
		rq := newRequest(ctx, ch, errs, offset)
		p.submitRequest(rq)
		select {
		case <-ctx.Done():
			return
		case err := <-errs:
			log.Println(err)
			return
		}
	}(ch)
	return ch
}

func (p pool[T]) submitRequest(rq request[T]) {
	select {
	case <-rq.Context().Done():
		return
	case <-p.done:
		log.Println("submit aborted, pool closed")
		rq.PostError() <- fmt.Errorf("request aborted as Pool has shutdown")
		return
	case p.requests <- rq:
		return
	}
}

func (p pool[T]) runPool(ctx context.Context, data *offsetData[T]) {
	log.Println("pool is starting...")
	defer close(p.requests)
	defer close(p.feed)
	defer close(p.done) // done should be first to close, which shuts down all Readers / Waiters, avoiding attempts to write to feed after its closed.

	defer func(data *offsetData[T]) {
		log.Printf("Pool shutting down with %d elements in data\n", data.Length())
	}(data)

	for {
		select {
		case <-ctx.Done():
			return

		case t := <-p.feed:
			data.Append(t)
			p.releaseWaitLock()

		case rq := <-p.requests:
			rqOff := rq.Offset()
			if rqOff < 0 {
				rqOff = data.Offset()
				// request with neg offset treated as requesting first available.
				rq.ResetOffset(rqOff)
			}

			if data.LengthFrom(rqOff) == 0 {
				// nothing to give, wait for new data
				go p.waitAndResubmit(rq, p.getWaitLock())
			} else {
				go p.postAndResubmit(rq, data.SliceFrom(rqOff))
			}
		}
	}
}

func (p *pool[T]) applyPolicy(data *offsetData[T]) {
	if p.policy.Size > 0 && p.policy.Size < data.Size() {
		data.TrimToSize(p.policy.Size)
	}
	if p.policy.Count > 0 && p.policy.Count < data.Length() {
		data.TrimToLength(p.policy.Count)
	}
}

func (p *pool[T]) getWaitLock() chan struct{} {
	p.waitLockMutex.Lock()
	defer p.waitLockMutex.Unlock()

	if p.waitLock == nil {
		p.waitLock = make(chan struct{})
	}
	return p.waitLock
}

func (p *pool[T]) releaseWaitLock() {
	p.waitLockMutex.Lock()
	defer p.waitLockMutex.Unlock()
	if p.waitLock != nil {
		close(p.waitLock)
		p.waitLock = nil
	}
}

// methods below are called outside the main pool thread.
func (p *pool[T]) waitAndResubmit(rq request[T], waitLock chan struct{}) {
	select {
	case <-rq.Context().Done():
		return
	case <-p.done:
		rq.PostError() <- fmt.Errorf("waiting request aborted, pool has shutdown")
		return
	case <-waitLock:
		p.submitRequest(rq)
	}
}

func (p pool[T]) postAndResubmit(rq request[T], data []T) {
	rq.PostData(data)
	p.submitRequest(rq)
}

func (p *pool[T]) waitAndPurge(rq request[T], waitLock chan struct{}) {
	select {
	case <-rq.Context().Done():
		return
	case <-p.done:
		rq.PostError() <- fmt.Errorf("waiting request aborted, pool has shutdown")
		return
	case <-waitLock:
		p.submitRequest(rq)
	}
}

// NewPool creates a new Pool containing any given data.
// The Pool will be returned in an active state, ready to receive new data or Read any given data.
// It will remain active until the given context is cancelled.
func NewPool[T any](ctx context.Context, policy Policy, data ...T) Pool[T] {
	if !policy.IsConstrainded() {
		log.Fatalln("policy is unconstrained. Pool can not have unlimited memory")
	}
	p := &pool[T]{
		feed:     make(chan T),
		requests: make(chan request[T], 10),
		done:     make(chan struct{}),
		policy:   policy,
	}
	go p.runPool(ctx, &offsetData[T]{
		data:   data,
		offset: 0,
	})
	return p
}
