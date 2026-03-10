package demos

import (
	"sync"

	"github.com/yesetoda/cs1gobot/robot"
)

// Action describes one named demo entry that can be launched from the UI.
type Action struct {
	// Label is the user-visible name of the demo.
	Label string
	// Run starts the demo when invoked.
	Run func()
}

// Actions returns the demos exposed by the application UI.
func Actions() []Action {
	return []Action{
		{Label: "Square", Run: StartSquareDemo},
		{Label: "Hurdles", Run: StartHurdlesDemo},
		{Label: "Harvest", Run: StartHarvestDemo},
		{Label: "Maze", Run: StartMazeDemo},
		{Label: "Goroutine race", Run: StartGoroutineRaceDemo},
	}
}

// StartSquareDemo launches a simple square-walking demonstration.
func StartSquareDemo() {
	robot.CreateWorld(10, 10)
	robot.SetStop(false)
	go runSquareDemo()
}

func runSquareDemo() {
	hubo := robot.New("blue")
	hubo.SetDelay(120)
	if robot.DefaultTraceColor != "" {
		hubo.SetTrace(robot.DefaultTraceColor)
	}

	for i := 0; i < 4; i++ {
		for step := 0; step < 4; step++ {
			hubo.Move()
		}
		hubo.TurnLeft()
	}
}

// StartHurdlesDemo launches a demo that jumps over a row of small hurdles.
func StartHurdlesDemo() {
	robot.CreateWorld(14, 6)
	for _, x := range []int{3, 6, 9, 12} {
		robot.CurrentWorld.AddWall(robot.Point{X: x, Y: 1}, robot.Point{X: x + 1, Y: 1})
	}
	robot.SetStop(false)
	go runHurdlesDemo()
}

func runHurdlesDemo() {
	hubo := robot.New("green")
	hubo.SetDelay(110)
	if robot.DefaultTraceColor != "" {
		hubo.SetTrace(robot.DefaultTraceColor)
	}

	for step := 0; step < 12; step++ {
		if hubo.FrontClear() {
			hubo.Move()
			continue
		}
		jumpSmallHurdle(hubo)
	}
}

func jumpSmallHurdle(hubo *robot.Robot) {
	hubo.TurnLeft()
	hubo.Move()
	hubo.TurnRight()
	hubo.Move()
	hubo.TurnRight()
	hubo.Move()
	hubo.TurnLeft()
}

// StartHarvestDemo launches a demo that collects beepers, returns home, and
// drops them back into the world.
func StartHarvestDemo() {
	robot.CreateWorld(10, 6)
	for x := 2; x <= 8; x++ {
		robot.CurrentWorld.AddBeeper(x, 1)
	}
	robot.SetStop(false)
	go runHarvestDemo()
}

func runHarvestDemo() {
	hubo := robot.New("yellow")
	hubo.SetDelay(100)
	if robot.DefaultTraceColor != "" {
		hubo.SetTrace(robot.DefaultTraceColor)
	}

	for {
		if hubo.OnBeeper() {
			hubo.PickBeeper()
		}
		if !hubo.FrontClear() {
			break
		}
		hubo.Move()
	}
	if hubo.OnBeeper() {
		hubo.PickBeeper()
	}

	hubo.TurnLeft()
	hubo.TurnLeft()
	for hubo.FrontClear() {
		hubo.Move()
	}
	hubo.TurnLeft()
	hubo.TurnLeft()

	for hubo.CarriesBeepers() {
		hubo.DropBeeper()
	}
}

// StartMazeDemo launches a fixed-path maze traversal demonstration.
func StartMazeDemo() {
	robot.CreateWorld(10, 8)
	addMazeWalls()
	robot.CurrentWorld.AddBeeper(9, 7)
	robot.SetStop(false)
	go runMazeDemo()
}

func addMazeWalls() {
	for y := 2; y <= 6; y++ {
		robot.CurrentWorld.AddWall(robot.Point{X: 2, Y: y}, robot.Point{X: 3, Y: y})
	}
	for x := 3; x <= 7; x++ {
		robot.CurrentWorld.AddWall(robot.Point{X: x + 3, Y: 2}, robot.Point{X: x + 3, Y: 3})
	}
	for y := 1; y <= 3; y++ {
		robot.CurrentWorld.AddWall(robot.Point{X: 6, Y: y}, robot.Point{X: 7, Y: y})
	}
	for x := 5; x <= 7; x++ {
		robot.CurrentWorld.AddWall(robot.Point{X: x, Y: 5}, robot.Point{X: x, Y: 6})
	}
	for y := 4; y <= 6; y++ {
		robot.CurrentWorld.AddWall(robot.Point{X: 8, Y: y}, robot.Point{X: 9, Y: y})
	}
}

func runMazeDemo() {
	hubo := robot.New("purple")
	hubo.SetDelay(120)
	if robot.DefaultTraceColor != "" {
		hubo.SetTrace(robot.DefaultTraceColor)
	}

	for step := 0; step < 4; step++ {
		hubo.Move()
	}
	hubo.TurnLeft()
	for step := 0; step < 3; step++ {
		hubo.Move()
	}
	hubo.TurnRight()
	for step := 0; step < 3; step++ {
		hubo.Move()
	}
	hubo.TurnLeft()
	for step := 0; step < 3; step++ {
		hubo.Move()
	}
	hubo.TurnRight()
	hubo.Move()
	if hubo.OnBeeper() {
		hubo.PickBeeper()
	}
}

// StartGoroutineRaceDemo launches multiple robots concurrently to demonstrate
// goroutine-based motion in separate lanes.
func StartGoroutineRaceDemo() {
	robot.CreateWorld(14, 8)
	for x := 1; x < 14; x++ {
		robot.CurrentWorld.AddWall(robot.Point{X: x, Y: 2}, robot.Point{X: x, Y: 3})
		robot.CurrentWorld.AddWall(robot.Point{X: x, Y: 4}, robot.Point{X: x, Y: 5})
		robot.CurrentWorld.AddWall(robot.Point{X: x, Y: 6}, robot.Point{X: x, Y: 7})
	}
	robot.SetStop(false)
	go runGoroutineRaceDemo()
}

func runGoroutineRaceDemo() {
	bots := []*robot.Robot{
		robot.NewAt(1, 1, robot.East, "blue"),
		robot.NewAt(1, 3, robot.East, "green"),
		robot.NewAt(1, 5, robot.East, "purple"),
	}
	delays := []int{70, 95, 120}
	steps := []int{11, 10, 9}

	var wg sync.WaitGroup
	for i, bot := range bots {
		bot.SetDelay(delays[i])
		if robot.DefaultTraceColor != "" {
			bot.SetTrace(robot.DefaultTraceColor)
		}

		wg.Add(1)
		go func(r *robot.Robot, count int) {
			defer wg.Done()
			for step := 0; step < count; step++ {
				if !r.FrontClear() {
					return
				}
				r.Move()
			}
		}(bot, steps[i])
	}
	wg.Wait()
}
