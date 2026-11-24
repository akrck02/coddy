package navigation

// Spot is a place the person was, to go back to: a file, by its path on disk as projects may be
// opened, closed and sorted again meanwhile, and a line of it, from 1.
type Spot struct {
	Path string
	Line int
}

// trailLength is how many places back the trail keeps.
const trailLength = 100

// Trail is where following names has been: the places to go back to, and forward again.
// A step of -1 goes back, of 1 forward.
type Trail struct{ back, forward []Spot }

// Leave keeps the spot the person leaves to go back to, when it is a place of a file, and forgets
// the way forward, as a browser does.
func (trail *Trail) Leave(from Spot) {
	if from.Line > 0 {
		trail.back = append(trail.back, from)
		if len(trail.back) > trailLength {
			trail.back = trail.back[1:]
		}
	}
	trail.forward = nil
}

// Next is the spot a step leads to; found is false when there is none that way.
func (trail *Trail) Next(step int) (spot Spot, found bool) {
	from, _ := trail.ways(step)
	if len(*from) == 0 {
		return Spot{}, false
	}
	return (*from)[len(*from)-1], true
}

// Drop forgets the spot a step leads to, as it cannot be reached: its project was closed.
func (trail *Trail) Drop(step int) {
	from, _ := trail.ways(step)
	if len(*from) > 0 {
		*from = (*from)[:len(*from)-1]
	}
}

// Take makes the step: the spot it leads to is left behind, and the place the person was, when
// a file was open, is kept to come back to the other way.
func (trail *Trail) Take(step int, current Spot, open bool) {
	trail.Drop(step)
	if !open {
		return
	}
	_, to := trail.ways(step)
	*to = append(*to, current)
}

// Behind is how many places back the trail keeps.
func (trail *Trail) Behind() int { return len(trail.back) }

// ways are the list a step takes its spot from, and the one it keeps the place left in.
func (trail *Trail) ways(step int) (from, to *[]Spot) {
	if step > 0 {
		return &trail.forward, &trail.back
	}
	return &trail.back, &trail.forward
}
