package robot

import "sync"

type Point struct {
	// X is the avenue index in the world, starting at 1 from the left.
	X int
	// Y is the street index in the world, starting at 1 from the bottom.
	Y int
}

// Wall represents a blocking wall between two adjacent points.
type Wall struct {
	P1 Point
	P2 Point
}

func normalizeWall(p1, p2 Point) Wall {
	if p1.X < p2.X || (p1.X == p2.X && p1.Y < p2.Y) {
		return Wall{P1: p1, P2: p2}
	}
	return Wall{P1: p2, P2: p1}
}

type World struct {
	mu sync.Mutex
	// Avenues is the width of the world (horizontal axis, 1..Avenues).
	Avenues int
	// Streets is the height of the world (vertical axis, 1..Streets).
	Streets int
	// Walls stores blocking segments between neighbouring points.
	Walls map[Wall]bool
	// Beepers stores how many beepers lie on each point.
	Beepers map[Point]int
	// update, when non-nil, is called after every visible world change.
	update func()
}

func adjacent(p1, p2 Point) bool {
	dx := p1.X - p2.X
	if dx < 0 {
		dx = -dx
	}
	dy := p1.Y - p2.Y
	if dy < 0 {
		dy = -dy
	}
	return dx+dy == 1
}

func NewWorld(avenues, streets int) *World {
	return &World{
		Avenues: avenues,
		Streets: streets,
		Walls:   make(map[Wall]bool),
		Beepers: make(map[Point]int),
	}
}

func (w *World) SetUpdateFunc(f func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.update = f
}

func (w *World) notify() {
	w.mu.Lock()
	update := w.update
	w.mu.Unlock()
	if update != nil {
		update()
	}
}

func (w *World) inBoundsLocked(p Point) bool {
	return p.X >= 1 && p.X <= w.Avenues && p.Y >= 1 && p.Y <= w.Streets
}

func (w *World) AddWall(p1, p2 Point) {
	w.mu.Lock()
	if !adjacent(p1, p2) || !w.inBoundsLocked(p1) || !w.inBoundsLocked(p2) {
		w.mu.Unlock()
		return
	}
	w.Walls[normalizeWall(p1, p2)] = true
	update := w.update
	w.mu.Unlock()
	if update != nil {
		update()
	}
}

func (w *World) RemoveWall(p1, p2 Point) {
	w.mu.Lock()
	if !adjacent(p1, p2) || !w.inBoundsLocked(p1) || !w.inBoundsLocked(p2) {
		w.mu.Unlock()
		return
	}
	delete(w.Walls, normalizeWall(p1, p2))
	update := w.update
	w.mu.Unlock()
	if update != nil {
		update()
	}
}

func (w *World) HasWall(p1, p2 Point) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !adjacent(p1, p2) {
		return false
	}
	return w.Walls[normalizeWall(p1, p2)]
}

func (w *World) ToggleWall(p1, p2 Point) {
	w.mu.Lock()
	if !adjacent(p1, p2) || !w.inBoundsLocked(p1) || !w.inBoundsLocked(p2) {
		w.mu.Unlock()
		return
	}
	wall := normalizeWall(p1, p2)
	if w.Walls[wall] {
		delete(w.Walls, wall)
	} else {
		w.Walls[wall] = true
	}
	update := w.update
	w.mu.Unlock()
	if update != nil {
		update()
	}
}

func (w *World) IsClear(p Point, dir Point) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	nextP := Point{X: p.X + dir.X, Y: p.Y + dir.Y}
	if nextP.X < 1 || nextP.X > w.Avenues || nextP.Y < 1 || nextP.Y > w.Streets {
		return false // Blocked by border
	}

	if w.Walls[normalizeWall(p, nextP)] {
		return false
	}
	return true
}

func (w *World) AddBeeper(x, y int) {
	w.mu.Lock()
	p := Point{X: x, Y: y}
	if !w.inBoundsLocked(p) {
		w.mu.Unlock()
		return
	}
	w.Beepers[p]++
	update := w.update
	w.mu.Unlock()
	if update != nil {
		update()
	}
}

func (w *World) RemoveBeeper(x, y int) {
	w.mu.Lock()
	p := Point{X: x, Y: y}
	if !w.inBoundsLocked(p) {
		w.mu.Unlock()
		return
	}
	if w.Beepers[p] > 0 {
		w.Beepers[p]--
		if w.Beepers[p] == 0 {
			delete(w.Beepers, p)
		}
		update := w.update
		w.mu.Unlock()
		if update != nil {
			update()
		}
		return
	}
	w.mu.Unlock()
}

func (w *World) OnBeeper(x, y int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Beepers[Point{X: x, Y: y}] > 0
}

func (w *World) BeeperCount(x, y int) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Beepers[Point{X: x, Y: y}]
}

func (w *World) GetAvenuesAndStreets() (int, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Avenues, w.Streets
}

func (w *World) GetSnapshot() (map[Wall]bool, map[Point]int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Create copies for safe iteration rendering
	wallsCopy := make(map[Wall]bool, len(w.Walls))
	for k, v := range w.Walls {
		wallsCopy[k] = v
	}
	beepersCopy := make(map[Point]int, len(w.Beepers))
	for k, v := range w.Beepers {
		beepersCopy[k] = v
	}
	return wallsCopy, beepersCopy
}

func (w *World) Clone() *World {
	w.mu.Lock()
	defer w.mu.Unlock()
	nw := NewWorld(w.Avenues, w.Streets)
	for k, v := range w.Walls {
		nw.Walls[k] = v
	}
	for k, v := range w.Beepers {
		nw.Beepers[k] = v
	}
	nw.update = w.update
	return nw
}

// Global state
var CurrentWorld *World
var UpdateUI func()

func CreateWorld(avenues, streets int) {
	if avenues == 0 {
		avenues = 10
	}
	if streets == 0 {
		streets = 10
	}
	CurrentWorld = NewWorld(avenues, streets)
	if UpdateUI != nil {
		CurrentWorld.SetUpdateFunc(UpdateUI)
	}
}
