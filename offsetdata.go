package pools

import (
	"unsafe"
)

type offsetData[T any] struct {
	data   []T
	offset int
}

// Length returns the number of elements in the data
func (d offsetData[T]) Length() int {
	return len(d.data)
}

// LengthFrom returns the number of elements in the data following (and including) the element at the given offset
// If given offset is less than the current pool offset, or greater than the pool offset plus its Length, zero is returned.
func (d offsetData[T]) LengthFrom(offset int) int {
	i := d.IndexOf(offset)
	if i < 0 { // offset out of range
		return 0
	}
	return len(d.data) - i
}

// Size returns the byte size of the data
func (d offsetData[T]) Size() uint64 {
	l := len(d.data)
	if l == 0 {
		return 0
	}
	sz := d.elementSize()
	return sz * uint64(l)
}

func (d offsetData[T]) Offset() int {
	return d.offset
}

func (d *offsetData[T]) Append(t ...T) {
	d.data = append(d.data, t...)
}

func (d *offsetData[T]) SliceFrom(offset int) []T {
	i := d.IndexOf(offset)
	if i < 0 {
		i = 0
	}
	return d.data[i:]
}

func (d *offsetData[T]) IndexOf(offset int) int {
	if offset < d.offset {
		return -1
	}
	i := offset - d.offset
	if i >= len(d.data) {
		return -1
	}
	return i
}

func (d *offsetData[T]) TrimToLength(count int) {
	if count >= len(d.data) {
		return
	}
	if count < 0 {
		count = 0
	}
	cut := len(d.data) - count
	d.offset += cut
	d.data = d.data[cut:]
}

func (d *offsetData[T]) TrimToSize(size uint64) {
	dsize := d.Size()
	esize := d.elementSize()
	if dsize == 0 || esize == 0 || size >= dsize {
		return
	}
	d.TrimToLength(int(size / esize))
}

func (d offsetData[T]) elementSize() uint64 {
	if len(d.data) == 0 {
		return 0
	}
	return uint64(unsafe.Sizeof(d.data[0]))
}
