package pools

const defaultPolicySize = 1024 * 1024 * 8 // 8k size default

var DefaultPolicy = Policy{
	Size: defaultPolicySize,
}

type Policy struct {
	Size  uint64
	Count int
}

func (pl Policy) IsConstrainded() bool {
	return pl.Size > 0 || pl.Count > 0
}
