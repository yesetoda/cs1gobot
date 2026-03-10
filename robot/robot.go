package robot

import (
	"sync"
	"time"
)

// Direction encodes one of the four compass directions a robot can face.
// The numeric values are chosen to line up with the Directions slice below.
type Direction int

const (
	// North faces toward increasing street numbers.
	North Direction = iota
	// West faces toward decreasing avenue numbers.
	West
	// South faces toward decreasing street numbers.
	South
	// East faces toward increasing avenue numbers.
	East
)

// Directions maps a Direction value to the unit step taken when the robot
// moves forward in that direction.
var Directions = []Point{
	{X: 0, Y: 1},  // N (dir 0)
	{X: -1, Y: 0}, // W (dir 1)
	{X: 0, Y: -1}, // S (dir 2)
	{X: 1, Y: 0},  // E (dir 3)
}

// StopExecution is the global stop flag checked before and after robot
// actions. External code should usually use SetStop and IsStopped instead of
// writing to this variable directly.
var StopExecution bool
var stopMu sync.Mutex

var stepMu sync.Mutex
var stepCond = sync.NewCond(&stepMu)
var stepModeEnabled bool
var stepBudget int
var stepWaiting bool
var stepCount uint64
var lastStepAction string

// DefaultDelayMs controls the pause (in milliseconds) new robots will use
// after each visible action. It can be adjusted from the UI or user code.
var DefaultDelayMs = 50

// DefaultTraceColor, when non-empty, makes newly created robots start with
// tracing enabled in the given colour name (for example, "cyan").
var DefaultTraceColor string

// MaxTracePoints bounds stored trace history per robot to keep rendering
// and snapshot copies fast during long runs. Set to <= 0 to disable capping.
var MaxTracePoints = 120

const minAnimationFrame = 16 * time.Millisecond

// StepDebugState is a snapshot of the current debugger stepping state.
type StepDebugState struct {
	// Enabled reports whether step mode is active.
	Enabled bool
	// Waiting reports whether execution is currently paused for a step token.
	Waiting bool
	// Pending is the number of queued StepOnce permits.
	Pending int
	// Steps is the number of visible robot actions executed since the last reset.
	Steps uint64
	// LastAction is the name of the most recent visible robot action.
	LastAction string
}

// SetStop enables or clears the global stop flag used by running robot code.
// When stop is true, any waiting step-mode action is released so execution can
// terminate promptly.
func SetStop(stop bool) {
	stopMu.Lock()
	StopExecution = stop
	stopMu.Unlock()

	if stop {
		stepMu.Lock()
		stepWaiting = false
		stepCond.Broadcast()
		stepMu.Unlock()
	}
}

// IsStopped reports whether the global stop flag is currently set.
func IsStopped() bool {
	stopMu.Lock()
	defer stopMu.Unlock()
	return StopExecution
}

// SetStepMode enables or disables debugger-style stepping.
// When enabled, robot actions wait for StepOnce before executing.
func SetStepMode(enabled bool) {
	stepMu.Lock()
	stepModeEnabled = enabled
	if !enabled {
		stepBudget = 0
		stepWaiting = false
	}
	stepCond.Broadcast()
	stepMu.Unlock()
}

// StepOnce allows one pending robot action to run in step mode.
func StepOnce() {
	stepMu.Lock()
	if !stepModeEnabled {
		stepModeEnabled = true
	}
	stepBudget++
	stepWaiting = false
	stepCond.Broadcast()
	stepMu.Unlock()
}

// ResetStepperState clears step counters and pending permits while keeping
// current step mode selection unchanged.
func ResetStepperState() {
	stepMu.Lock()
	stepBudget = 0
	stepWaiting = false
	stepCount = 0
	lastStepAction = ""
	stepCond.Broadcast()
	stepMu.Unlock()
}

// GetStepDebugState returns the current debugger/stepper state.
func GetStepDebugState() StepDebugState {
	stepMu.Lock()
	defer stepMu.Unlock()

	return StepDebugState{
		Enabled:    stepModeEnabled,
		Waiting:    stepWaiting,
		Pending:    stepBudget,
		Steps:      stepCount,
		LastAction: lastStepAction,
	}
}

func waitForStepPermit() {
	for {
		if IsStopped() {
			return
		}

		stepMu.Lock()
		if !stepModeEnabled {
			stepWaiting = false
			stepMu.Unlock()
			return
		}
		if stepBudget > 0 {
			stepBudget--
			stepWaiting = false
			stepMu.Unlock()
			return
		}

		stepWaiting = true
		stepCond.Wait()
		stepMu.Unlock()
	}
}

func recordStepAction(action string) {
	stepMu.Lock()
	stepCount++
	lastStepAction = action
	stepMu.Unlock()
}

// Robot represents a single robot inside the current world. All exported
// methods are safe to call from multiple goroutines.
type Robot struct {
	mu sync.Mutex

	// X and Y track the robot position in world grid coordinates (avenue,
	// street), both starting at 1.
	X int
	Y int

	// Dir is the current facing direction as an index into Directions.
	Dir int

	// BeeperBag is the number of beepers currently carried by the robot.
	BeeperBag int

	// Delay controls how long the robot pauses after each visible action.
	Delay time.Duration

	// Trace controls whether the robot leaves a visible path behind it.
	Trace     bool
	TracePath []Point

	// Color is used by the UI when choosing a sprite for this robot.
	Color string

	world *World
}

// Registry contains all currently active robots in the world.
var Registry []*Robot

// RegistryMu protects Registry while snapshots are taken for rendering or
// introspection.
var RegistryMu sync.Mutex

// New creates a new robot at avenue 1, street 1, initially facing East in
// the current world. An optional first argument may be a colour name used
// by the UI when selecting the robot sprite.
func New(opts ...interface{}) *Robot {
	color := "gray"
	if len(opts) > 0 {
		if c, ok := opts[0].(string); ok {
			color = c
		}
	}

	r := &Robot{
		X:         1,
		Y:         1,
		Dir:       int(East),
		BeeperBag: 0,
		Delay:     time.Duration(DefaultDelayMs) * time.Millisecond,
		Color:     color,
		world:     CurrentWorld,
	}

	RegistryMu.Lock()
	Registry = append(Registry, r)
	RegistryMu.Unlock()

	if r.world != nil {
		r.world.notify() // Trigger a redraw to place the robot
	}

	if DefaultTraceColor != "" {
		r.SetTrace(DefaultTraceColor)
	}
	return r
}

// NewAt creates a new robot at the given position and direction.
func NewAt(av, st int, dir Direction, color string) *Robot {
	r := &Robot{
		X:         av,
		Y:         st,
		Dir:       int(dir),
		BeeperBag: 0,
		Delay:     time.Duration(DefaultDelayMs) * time.Millisecond,
		Color:     color,
		world:     CurrentWorld,
	}

	RegistryMu.Lock()
	Registry = append(Registry, r)
	RegistryMu.Unlock()

	if r.world != nil {
		r.world.notify()
	}
	if DefaultTraceColor != "" {
		r.SetTrace(DefaultTraceColor)
	}
	return r
}

// SetPause sets the delay between visible actions for this robot in
// milliseconds.
func (r *Robot) SetPause(delayMs int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Delay = time.Duration(delayMs) * time.Millisecond
}

// SetDelay is a Go-idiomatic alias for SetPause measured in milliseconds.
func (r *Robot) SetDelay(delayMs int) {
	r.SetPause(delayMs)
}

// SetDefaultDelay changes the DefaultDelayMs used for newly created robots.
// It does not affect robots that already exist.
func SetDefaultDelay(delayMs int) {
	DefaultDelayMs = delayMs
}

// SetDefaultTraceColor changes the DefaultTraceColor used for newly created
// robots. Passing an empty string disables automatic traces.
func SetDefaultTraceColor(color string) {
	DefaultTraceColor = color
}

// SetMaxTracePoints changes the global trace history cap used by new and
// existing robots. Values <= 0 disable trimming.
func SetMaxTracePoints(max int) {
	MaxTracePoints = max
}

// SetTrace enables or disables path tracing for this robot.
//
// Any non-empty string enables tracing. The string is currently treated as a
// semantic colour hint used by the rest of the app rather than a strict trace
// renderer parameter.
func (r *Robot) SetTrace(enabled string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Trace = enabled != ""
	if r.Trace {
		// Reset the path from current position when tracing is (re)enabled.
		r.TracePath = []Point{{X: r.X, Y: r.Y}}
		return
	}
	r.TracePath = nil
}

// GetState returns the robot's current avenue, street, direction, and colour.
func (r *Robot) GetState() (int, int, int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.X, r.Y, r.Dir, r.Color
}

// GetTrace returns a copy of the robot's recorded trace path.
func (r *Robot) GetTrace() []Point {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]Point, len(r.TracePath))
	copy(res, r.TracePath)
	return res
}

func (r *Robot) checkStop() {
	if IsStopped() {
		panic("Execution stopped by user")
	}
}

func (r *Robot) beforeAction() {
	r.checkStop()
	waitForStepPermit()
	r.checkStop()
}

func (r *Robot) notifyAndPause(action string) {
	recordStepAction(action)
	r.checkStop()
	if r.world != nil {
		r.world.notify()
	}

	r.mu.Lock()
	delay := r.Delay
	r.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	} else {
		// Keep zero-delay runs visually smooth instead of bursting many
		// state changes into a single rendered frame.
		time.Sleep(minAnimationFrame)
	}
	r.checkStop()
}

// Move advances the robot forward by one cell.
//
// Move panics with a RobotError message if the path ahead is blocked by a wall
// or the world boundary.
func (r *Robot) Move() {
	r.beforeAction()
	r.mu.Lock()
	dir := Directions[r.Dir]
	if r.world == nil || !r.world.IsClear(Point{X: r.X, Y: r.Y}, dir) {
		r.mu.Unlock()
		panic("RobotError: That move really hurt! Please, make sure that there is no wall in front of me!")
	}
	r.X += dir.X
	r.Y += dir.Y
	if r.Trace {
		r.TracePath = append(r.TracePath, Point{X: r.X, Y: r.Y})
		if max := MaxTracePoints; max > 0 && len(r.TracePath) > max {
			drop := len(r.TracePath) - max
			copy(r.TracePath, r.TracePath[drop:])
			r.TracePath = r.TracePath[:max]
		}
	}
	r.mu.Unlock()
	r.notifyAndPause("Move")
}

// TurnLeft rotates the robot 90 degrees counter-clockwise.
func (r *Robot) TurnLeft() {
	r.beforeAction()
	r.mu.Lock()
	r.Dir = (r.Dir + 1) % 4
	r.mu.Unlock()
	r.notifyAndPause("TurnLeft")
}

// TurnRight turns the robot 90 degrees to the right.
func (r *Robot) TurnRight() {
	r.beforeAction()
	r.mu.Lock()
	r.Dir = (r.Dir + 3) % 4
	r.mu.Unlock()
	r.notifyAndPause("TurnRight")
}

// PickBeeper removes one beeper from the robot's current cell and places it in
// the robot's bag.
//
// PickBeeper panics with a RobotError message if the robot is not standing on a
// beeper.
func (r *Robot) PickBeeper() {
	r.beforeAction()
	r.mu.Lock()
	if r.world == nil {
		r.mu.Unlock()
		panic("RobotError: No world.")
	}
	onBeeper := r.world.OnBeeper(r.X, r.Y)
	if !onBeeper {
		r.mu.Unlock()
		panic("RobotError: I must be on a beeper to pick it up.")
	}

	targetX, targetY := r.X, r.Y
	r.BeeperBag++
	r.mu.Unlock()

	r.world.RemoveBeeper(targetX, targetY)
	r.notifyAndPause("PickBeeper")
}

// DropBeeper places one beeper from the robot's bag onto the current cell.
//
// DropBeeper panics with a RobotError message if the robot is not carrying any
// beepers.
func (r *Robot) DropBeeper() {
	r.beforeAction()
	r.mu.Lock()
	if r.BeeperBag <= 0 {
		r.mu.Unlock()
		panic("RobotError: I am not carrying any beepers.")
	}
	if r.world == nil {
		r.mu.Unlock()
		panic("RobotError: No world.")
	}

	r.BeeperBag--
	targetX, targetY := r.X, r.Y
	r.mu.Unlock()

	r.world.AddBeeper(targetX, targetY)
	r.notifyAndPause("DropBeeper")
}

// FrontIsClear reports whether there is no wall or border immediately
// in front of the robot. Prefer FrontClear in new code; this name is
// kept for compatibility with the original Python API.
func (r *Robot) FrontIsClear() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.world == nil {
		return false
	}
	return r.world.IsClear(Point{X: r.X, Y: r.Y}, Directions[r.Dir])
}

// FrontClear is the Go-idiomatic spelling for FrontIsClear.
func (r *Robot) FrontClear() bool {
	return r.FrontIsClear()
}

// LeftIsClear reports whether there is no wall or border immediately to
// the robot's left. Prefer LeftClear in new code; this name is kept for
// compatibility with the original Python API.
func (r *Robot) LeftIsClear() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.world == nil {
		return false
	}
	leftDir := (r.Dir + 1) % 4
	return r.world.IsClear(Point{X: r.X, Y: r.Y}, Directions[leftDir])
}

// LeftClear is the Go-idiomatic spelling for LeftIsClear.
func (r *Robot) LeftClear() bool {
	return r.LeftIsClear()
}

// RightIsClear reports whether there is no wall or border immediately
// to the robot's right. Prefer RightClear in new code; this name is
// kept for compatibility with the original Python API.
func (r *Robot) RightIsClear() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.world == nil {
		return false
	}
	rightDir := (r.Dir + 3) % 4
	return r.world.IsClear(Point{X: r.X, Y: r.Y}, Directions[rightDir])
}

// RightClear is the Go-idiomatic spelling for RightIsClear.
func (r *Robot) RightClear() bool {
	return r.RightIsClear()
}

// FacingNorth reports whether the robot is currently facing north.
func (r *Robot) FacingNorth() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Dir == 0
}

// CarriesBeepers reports whether the robot carries at least one beeper.
func (r *Robot) CarriesBeepers() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.BeeperBag > 0
}

// OnBeeper reports whether there are any beepers on the robot's
// current position in the world.
func (r *Robot) OnBeeper() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.world == nil {
		return false
	}
	return r.world.OnBeeper(r.X, r.Y)
}

// Reset removes all registered robots and clears step-debugger counters.
// It is typically called before starting a new script or demo run.
func Reset() {
	RegistryMu.Lock()
	Registry = nil
	RegistryMu.Unlock()
	ResetStepperState()
}
