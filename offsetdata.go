package pools

type offsetData[T any] struct {
	data   []T
	offset int
}

func (d offsetData[T]) Length() int {
	return len(d.data)
}

func (d offsetData[T]) LengthFrom(offset int) int {
	i := offset - d.offset
	if i < 0 {
		i = 0
	}
	if i > len(d.data) {
		i = len(d.data)
	}
	return len(d.data) - i
}

func (d offsetData[T]) Offset() int {
	return d.offset
}

func (d *offsetData[T]) Append(t ...T) {
	d.data = append(d.data, t...)
}

func (d *offsetData[T]) SliceFrom(offset int) []T {
	i := offset - d.offset
	if i < 0 {
		i = 0
	}
	return d.data[i:]
}

func (d *offsetData[T]) Trim(size int) {
	if size >= len(d.data) {
		return
	}
	if size < 0 {
		size = 0
	}
	cut := len(d.data) - size
	d.offset += cut
	d.data = d.data[cut:]
}
