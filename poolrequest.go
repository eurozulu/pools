package pools

import "context"

type request[T any] interface {
	Context() context.Context
	Offset() int
	ResetOffset(offset int)
	PostError() chan<- error
	PostData(data []T)
}

type requestImpl[T any] struct {
	ctx       context.Context
	ch        chan<- T
	err       chan<- error
	offset    int
	additions int
}

func (rq requestImpl[T]) Context() context.Context {
	return rq.ctx
}

func (rq requestImpl[T]) Offset() int {
	return rq.offset + rq.additions
}

func (rq requestImpl[T]) ReadCount() int {
	return rq.additions
}

func (rq *requestImpl[T]) ResetOffset(offset int) {
	rq.offset = offset
	rq.additions = 0
}

func (rq requestImpl[T]) PostError() chan<- error {
	return rq.err
}

func (rq *requestImpl[T]) PostData(data []T) {
	var count int
	for _, t := range data {
		select {
		case <-rq.Context().Done():
			return
		case rq.ch <- t:
			count++
		}
	}
	rq.additions += count
}

func (rq requestImpl[T]) IsOffsetValid() bool {
	return rq.offset >= 0
}

func newRequest[T any](ctx context.Context, out chan<- T, err chan<- error, offset int) request[T] {
	return &requestImpl[T]{
		ctx:    ctx,
		ch:     out,
		err:    err,
		offset: offset,
	}
}
