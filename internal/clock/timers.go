package clock

// timers represents a list of sortable timers.
type Timers []*Timer

func (ts Timers) Len() int { return len(ts) }

func (ts Timers) Swap(i, j int) {
	ts[i], ts[j] = ts[j], ts[i]
}

func (ts Timers) Less(i, j int) bool {
	return ts[i].Next().Before(ts[j].Next())
}

func (ts *Timers) Push(t interface{}) {
	*ts = append(*ts, t.(*Timer))
}

func (ts *Timers) Pop() interface{} {
	t := (*ts)[len(*ts)-1]
	*ts = (*ts)[:len(*ts)-1]
	return t
}
