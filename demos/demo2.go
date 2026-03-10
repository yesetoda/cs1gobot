package demos

import "github.com/yesetoda/cs1gobot/robot"

var directionStep = map[int]robot.Point{
	int(robot.North): {X: 0, Y: 1},
	int(robot.West):  {X: -1, Y: 0},
	int(robot.South): {X: 0, Y: -1},
	int(robot.East):  {X: 1, Y: 0},
}

func pickAllBeepers(r *robot.Robot) {
	for r.OnBeeper() {
		r.PickBeeper()
	}
}

func turnTo(r *robot.Robot, facing, target int) int {
	diff := (target - facing + 4) % 4
	switch diff {
	case 1:
		r.TurnLeft()
	case 2:
		r.TurnLeft()
		r.TurnLeft()
	case 3:
		r.TurnRight()
	}
	return target
}

// explore does DFS backtracking and returns with the robot at (x,y)
// facing the same direction it had on entry.
func explore(r *robot.Robot, x, y, facing int, seen map[robot.Point]bool) int {
	entryFacing := facing
	cur := robot.Point{X: x, Y: y}
	if seen[cur] {
		return facing
	}

	seen[cur] = true
	pickAllBeepers(r)

	directions := []int{int(robot.North), int(robot.West), int(robot.South), int(robot.East)}
	for _, dir := range directions {
		step := directionStep[dir]
		next := robot.Point{X: x + step.X, Y: y + step.Y}
		if seen[next] {
			continue
		}

		facing = turnTo(r, facing, dir)
		if !r.FrontClear() {
			continue
		}

		r.Move()
		facing = explore(r, next.X, next.Y, facing, seen)

		// Backtrack to current cell and restore facing for more exploration.
		facing = turnTo(r, facing, (dir+2)%4)
		r.Move()
		facing = turnTo(r, facing, dir)
	}

	return turnTo(r, facing, entryFacing)
}

func main() {
	hubo := robot.New("blue")
	hubo.SetDelay(100)
	hubo.SetTrace("cyan")
	x, y, dir, _ := hubo.GetState()
	seen := make(map[robot.Point]bool)
	explore(hubo, x, y, dir, seen)
}
