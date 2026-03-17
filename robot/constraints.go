package robot

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const constraintErrorPrefix = "ConstraintReached:"

// Constraints describes optional runtime limits enforced during robot actions.
type Constraints struct {
	// MaxMoves caps successful Move actions.
	MaxMoves int
	// MaxActions caps all visible robot actions (move/turn/pick/drop).
	MaxActions int
	// MaxTurns caps TurnLeft + TurnRight actions.
	MaxTurns int
	// MaxPicks caps PickBeeper actions.
	MaxPicks int
	// TimeLimitMs caps elapsed run time in milliseconds.
	TimeLimitMs int
}

// GoalType identifies a high-level learning objective.
type GoalType string

const (
	GoalNone                  GoalType = "none"
	GoalCollectAllBeepers     GoalType = "collect_all_beepers"
	GoalCollectAtLeastBeepers GoalType = "collect_at_least_beepers"
	GoalVisitAllCells         GoalType = "visit_all_cells"
	GoalVisitAtLeastCells     GoalType = "visit_at_least_cells"
)

// Goal configures which objective should be evaluated after a run.
type Goal struct {
	Type   GoalType
	Target int
}

// RunStats tracks what happened during the most recent run.
type RunStats struct {
	Started bool

	Actions int
	Moves   int
	Turns   int
	Picks   int
	Drops   int

	VisitedCells int
	WorldCells   int

	InitialBeepers   int
	RemainingBeepers int
	CollectedBeepers int
	// PotentialBeeperUpperBound is a broad upper bound based on hard limits.
	PotentialBeeperUpperBound int
	// PotentialBeeperGreedy is a distance-aware greedy estimate.
	PotentialBeeperGreedy int

	ElapsedMs int64
}

// GoalEvaluation contains the outcome of evaluating the configured goal.
type GoalEvaluation struct {
	Goal     Goal
	Achieved bool
	Progress float64
	Message  string
	Details  string
}

var runStateMu sync.Mutex
var runConstraints Constraints
var runGoal = Goal{Type: GoalNone}
var runStats RunStats
var runStartedAt time.Time
var runVisited = map[Point]bool{}
var runInitialBeeperPiles = map[Point]int{}
var runStartPoint Point
var runHasStartPoint bool

func sanitizeConstraints(c Constraints) Constraints {
	if c.MaxMoves < 0 {
		c.MaxMoves = 0
	}
	if c.MaxActions < 0 {
		c.MaxActions = 0
	}
	if c.MaxTurns < 0 {
		c.MaxTurns = 0
	}
	if c.MaxPicks < 0 {
		c.MaxPicks = 0
	}
	if c.TimeLimitMs < 0 {
		c.TimeLimitMs = 0
	}
	return c
}

func normalizeGoal(g Goal) Goal {
	switch g.Type {
	case GoalCollectAllBeepers, GoalCollectAtLeastBeepers, GoalVisitAllCells, GoalVisitAtLeastCells:
		// keep as-is
	default:
		g.Type = GoalNone
	}
	if g.Target < 0 {
		g.Target = 0
	}
	return g
}

// SetConstraints updates global runtime limits used by the next run.
func SetConstraints(c Constraints) {
	runStateMu.Lock()
	runConstraints = sanitizeConstraints(c)
	runStateMu.Unlock()
}

// GetConstraints returns the currently configured runtime limits.
func GetConstraints() Constraints {
	runStateMu.Lock()
	defer runStateMu.Unlock()
	return runConstraints
}

// ClearConstraints disables all runtime limits.
func ClearConstraints() {
	SetConstraints(Constraints{})
}

// SetGoal sets the currently configured learning goal.
func SetGoal(g Goal) {
	runStateMu.Lock()
	runGoal = normalizeGoal(g)
	runStateMu.Unlock()
}

// GetGoal returns the currently configured learning goal.
func GetGoal() Goal {
	runStateMu.Lock()
	defer runStateMu.Unlock()
	return runGoal
}

// ClearGoal removes any configured learning goal.
func ClearGoal() {
	SetGoal(Goal{Type: GoalNone})
}

func snapshotWorldNumbers() (int, int) {
	if CurrentWorld == nil {
		return 0, 0
	}
	av, st := CurrentWorld.GetAvenuesAndStreets()
	_, beepers := CurrentWorld.GetSnapshot()
	total := 0
	for _, count := range beepers {
		total += count
	}
	return av * st, total
}

func snapshotRobotPoints() []Point {
	RegistryMu.Lock()
	robots := make([]*Robot, len(Registry))
	copy(robots, Registry)
	RegistryMu.Unlock()

	out := make([]Point, 0, len(robots))
	for _, r := range robots {
		if r == nil {
			continue
		}
		r.mu.Lock()
		out = append(out, Point{X: r.X, Y: r.Y})
		r.mu.Unlock()
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func copyPointIntMap(in map[Point]int) map[Point]int {
	out := make(map[Point]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func estimateBeeperPotential(stats RunStats, constraints Constraints, initialPiles map[Point]int, start Point, hasStart bool) (int, int) {
	if stats.InitialBeepers <= 0 {
		return 0, 0
	}

	upper := stats.InitialBeepers
	if constraints.MaxPicks > 0 {
		upper = minInt(upper, constraints.MaxPicks)
	}
	if constraints.MaxActions > 0 {
		upper = minInt(upper, constraints.MaxActions)
	}

	if !hasStart {
		return upper, upper
	}

	movesLeft := 1 << 30
	if constraints.MaxMoves > 0 {
		movesLeft = constraints.MaxMoves
	}
	actionsLeft := 1 << 30
	if constraints.MaxActions > 0 {
		actionsLeft = constraints.MaxActions
	}
	picksLeft := 1 << 30
	if constraints.MaxPicks > 0 {
		picksLeft = constraints.MaxPicks
	}

	if actionsLeft <= 0 || picksLeft <= 0 {
		return upper, 0
	}

	piles := copyPointIntMap(initialPiles)
	current := start
	collected := 0

	for {
		if actionsLeft <= 0 || picksLeft <= 0 {
			break
		}

		if pile := piles[current]; pile > 0 {
			take := pile
			if take > picksLeft {
				take = picksLeft
			}
			if take > actionsLeft {
				take = actionsLeft
			}
			if take > 0 {
				collected += take
				actionsLeft -= take
				picksLeft -= take
				pile -= take
				if pile > 0 {
					piles[current] = pile
				} else {
					delete(piles, current)
				}
				continue
			}
		}

		best := Point{}
		bestFound := false
		bestScoreNum := -1
		bestScoreDen := 1
		bestDist := 0

		for p, count := range piles {
			if count <= 0 {
				continue
			}
			dx := p.X - current.X
			if dx < 0 {
				dx = -dx
			}
			dy := p.Y - current.Y
			if dy < 0 {
				dy = -dy
			}
			dist := dx + dy
			if dist > movesLeft || dist > actionsLeft {
				continue
			}

			num := count
			den := dist + 1
			if !bestFound || num*bestScoreDen > bestScoreNum*den || (num*bestScoreDen == bestScoreNum*den && dist < bestDist) {
				bestFound = true
				best = p
				bestScoreNum = num
				bestScoreDen = den
				bestDist = dist
			}
		}

		if !bestFound {
			break
		}

		movesLeft -= bestDist
		actionsLeft -= bestDist
		current = best
	}

	if collected > upper {
		collected = upper
	}
	return upper, collected
}

// BeginRun resets run statistics and starts timing for a new execution.
func BeginRun() {
	worldCells, initialBeepers := snapshotWorldNumbers()
	robotPoints := snapshotRobotPoints()
	initialPiles := map[Point]int{}
	if CurrentWorld != nil {
		_, beepers := CurrentWorld.GetSnapshot()
		initialPiles = beepers
	}

	runStateMu.Lock()
	runStartedAt = time.Now()
	runVisited = map[Point]bool{}
	for _, p := range robotPoints {
		runVisited[p] = true
	}
	runStats = RunStats{
		Started:          true,
		WorldCells:       worldCells,
		InitialBeepers:   initialBeepers,
		RemainingBeepers: initialBeepers,
		CollectedBeepers: 0,
		VisitedCells:     len(runVisited),
		ElapsedMs:        0,
	}
	runInitialBeeperPiles = copyPointIntMap(initialPiles)
	runHasStartPoint = len(robotPoints) > 0
	if runHasStartPoint {
		runStartPoint = robotPoints[0]
	} else {
		runStartPoint = Point{}
	}
	runStateMu.Unlock()
}

// ResetRunTracking clears run statistics without changing configured limits/goals.
func ResetRunTracking() {
	runStateMu.Lock()
	runStartedAt = time.Time{}
	runVisited = map[Point]bool{}
	runInitialBeeperPiles = map[Point]int{}
	runStartPoint = Point{}
	runHasStartPoint = false
	runStats = RunStats{}
	runStateMu.Unlock()
}

func syncWorldBeeperCounts() {
	_, remaining := snapshotWorldNumbers()

	runStateMu.Lock()
	if !runStats.Started {
		runStateMu.Unlock()
		return
	}
	runStats.RemainingBeepers = remaining
	if runStats.InitialBeepers >= remaining {
		runStats.CollectedBeepers = runStats.InitialBeepers - remaining
	} else {
		runStats.CollectedBeepers = 0
	}
	runStateMu.Unlock()
}

func recordRobotSpawn(r *Robot) {
	if r == nil {
		return
	}

	r.mu.Lock()
	p := Point{X: r.X, Y: r.Y}
	r.mu.Unlock()

	runStateMu.Lock()
	if !runStats.Started {
		runStateMu.Unlock()
		return
	}
	runVisited[p] = true
	runStats.VisitedCells = len(runVisited)
	runStateMu.Unlock()
}

func recordRunAction(action string, r *Robot) {
	runStateMu.Lock()
	if !runStats.Started {
		runStateMu.Unlock()
		BeginRun()
		runStateMu.Lock()
	}

	runStats.Actions++
	switch action {
	case "Move":
		runStats.Moves++
		if r != nil {
			r.mu.Lock()
			p := Point{X: r.X, Y: r.Y}
			r.mu.Unlock()
			runVisited[p] = true
			runStats.VisitedCells = len(runVisited)
		}
	case "TurnLeft", "TurnRight":
		runStats.Turns++
	case "PickBeeper":
		runStats.Picks++
	case "DropBeeper":
		runStats.Drops++
	}

	if !runStartedAt.IsZero() {
		runStats.ElapsedMs = time.Since(runStartedAt).Milliseconds()
	}
	runStateMu.Unlock()

	syncWorldBeeperCounts()
}

func checkRunConstraintsNext(action string) error {
	constraints := GetConstraints()
	if constraints == (Constraints{}) {
		return nil
	}

	stats := CurrentRunStats()
	if !stats.Started {
		return nil
	}

	if constraints.TimeLimitMs > 0 && stats.ElapsedMs >= int64(constraints.TimeLimitMs) {
		return fmt.Errorf("%s time limit (%d ms) reached", constraintErrorPrefix, constraints.TimeLimitMs)
	}
	if constraints.MaxActions > 0 && stats.Actions >= constraints.MaxActions {
		return fmt.Errorf("%s action limit (%d) reached", constraintErrorPrefix, constraints.MaxActions)
	}

	switch action {
	case "Move":
		if constraints.MaxMoves > 0 && stats.Moves >= constraints.MaxMoves {
			return fmt.Errorf("%s move limit (%d) reached", constraintErrorPrefix, constraints.MaxMoves)
		}
	case "TurnLeft", "TurnRight":
		if constraints.MaxTurns > 0 && stats.Turns >= constraints.MaxTurns {
			return fmt.Errorf("%s turn limit (%d) reached", constraintErrorPrefix, constraints.MaxTurns)
		}
	case "PickBeeper":
		if constraints.MaxPicks > 0 && stats.Picks >= constraints.MaxPicks {
			return fmt.Errorf("%s pick limit (%d) reached", constraintErrorPrefix, constraints.MaxPicks)
		}
	}

	return nil
}

// IsConstraintReachedError reports whether err came from constraint enforcement.
func IsConstraintReachedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), constraintErrorPrefix)
}

// CurrentRunStats returns a fresh snapshot of run metrics.
func CurrentRunStats() RunStats {
	worldCells, remaining := snapshotWorldNumbers()
	constraints := GetConstraints()

	runStateMu.Lock()
	stats := runStats
	startedAt := runStartedAt
	initialPiles := copyPointIntMap(runInitialBeeperPiles)
	startPoint := runStartPoint
	hasStart := runHasStartPoint
	runStateMu.Unlock()

	if !stats.Started {
		return stats
	}

	if !startedAt.IsZero() {
		stats.ElapsedMs = time.Since(startedAt).Milliseconds()
	}
	if worldCells > 0 {
		stats.WorldCells = worldCells
	}
	stats.RemainingBeepers = remaining
	if stats.InitialBeepers >= remaining {
		stats.CollectedBeepers = stats.InitialBeepers - remaining
	} else {
		stats.CollectedBeepers = 0
	}

	stats.PotentialBeeperUpperBound, stats.PotentialBeeperGreedy = estimateBeeperPotential(stats, constraints, initialPiles, startPoint, hasStart)

	return stats
}

func clampProgress(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func progressMessage(achieved bool, progress float64) string {
	if achieved {
		return "Great job! You achieved this goal."
	}
	if progress >= 0.9 {
		return "Too bad, you did not achieve this yet, but you were very close."
	}
	if progress >= 0.6 {
		return "Too bad, you did not achieve this yet, but you made solid progress."
	}
	return "Too bad, you did not achieve this goal yet."
}

// EvaluateGoal computes goal status and feedback from current run metrics.
func EvaluateGoal() GoalEvaluation {
	goal := GetGoal()
	stats := CurrentRunStats()

	if goal.Type == GoalNone {
		msg := fmt.Sprintf(
			"Run summary: %d actions, %d moves, %d/%d beepers collected, %d/%d cells visited.",
			stats.Actions,
			stats.Moves,
			stats.CollectedBeepers,
			stats.InitialBeepers,
			stats.VisitedCells,
			stats.WorldCells,
		)
		return GoalEvaluation{
			Goal:     goal,
			Achieved: true,
			Progress: 1,
			Message:  msg,
			Details:  "No goal was configured.",
		}
	}

	evaluation := GoalEvaluation{Goal: goal}
	var target int
	var current int

	switch goal.Type {
	case GoalCollectAllBeepers:
		target = stats.InitialBeepers
		current = stats.CollectedBeepers
		if target <= 0 {
			evaluation.Achieved = true
			evaluation.Progress = 1
			evaluation.Message = "Great job! There were no beepers to collect in this world."
			evaluation.Details = "Goal collect_all_beepers is trivially satisfied."
			return evaluation
		}
		evaluation.Achieved = stats.RemainingBeepers == 0
		evaluation.Progress = clampProgress(float64(current) / float64(target))
		evaluation.Message = progressMessage(evaluation.Achieved, evaluation.Progress)
		evaluation.Details = fmt.Sprintf(
			"Collected %d/%d beepers (%d remaining). Potential under limits: up to %d (greedy estimate %d).",
			current,
			target,
			stats.RemainingBeepers,
			stats.PotentialBeeperUpperBound,
			stats.PotentialBeeperGreedy,
		)
	case GoalCollectAtLeastBeepers:
		target = goal.Target
		if target <= 0 {
			target = 1
		}
		current = stats.CollectedBeepers
		evaluation.Achieved = current >= target
		evaluation.Progress = clampProgress(float64(current) / float64(target))
		evaluation.Message = progressMessage(evaluation.Achieved, evaluation.Progress)
		evaluation.Details = fmt.Sprintf(
			"Collected %d/%d required beepers. Potential under limits: up to %d (greedy estimate %d).",
			current,
			target,
			stats.PotentialBeeperUpperBound,
			stats.PotentialBeeperGreedy,
		)
	case GoalVisitAllCells:
		target = stats.WorldCells
		current = stats.VisitedCells
		if target <= 0 {
			evaluation.Achieved = true
			evaluation.Progress = 1
			evaluation.Message = "Great job! There were no world cells to visit."
			evaluation.Details = "Goal visit_all_cells is trivially satisfied."
			return evaluation
		}
		evaluation.Achieved = current >= target
		evaluation.Progress = clampProgress(float64(current) / float64(target))
		evaluation.Message = progressMessage(evaluation.Achieved, evaluation.Progress)
		evaluation.Details = fmt.Sprintf("Visited %d/%d cells.", current, target)
	case GoalVisitAtLeastCells:
		target = goal.Target
		if target <= 0 {
			target = 1
		}
		current = stats.VisitedCells
		evaluation.Achieved = current >= target
		evaluation.Progress = clampProgress(float64(current) / float64(target))
		evaluation.Message = progressMessage(evaluation.Achieved, evaluation.Progress)
		evaluation.Details = fmt.Sprintf("Visited %d/%d target cells.", current, target)
	default:
		evaluation.Achieved = true
		evaluation.Progress = 1
		evaluation.Message = "Run completed."
		evaluation.Details = "Unknown goal type; nothing to evaluate."
		return evaluation
	}

	if !evaluation.Achieved {
		switch goal.Type {
		case GoalCollectAllBeepers, GoalCollectAtLeastBeepers:
			evaluation.Details += " Tip: prioritize high-value beeper piles early when limits are tight."
		case GoalVisitAllCells, GoalVisitAtLeastCells:
			evaluation.Details += " Tip: use DFS/BFS frontier expansion to improve coverage."
		}
	}

	return evaluation
}
