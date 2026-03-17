package main

import (
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"github.com/yesetoda/cs1gobot/demos"
	"github.com/yesetoda/cs1gobot/engine"
	"github.com/yesetoda/cs1gobot/robot"
	"github.com/yesetoda/cs1gobot/ui"
)

func main() {
	a := app.NewWithID("github.com.yesetoda.cs1gobot")
	w := a.NewWindow("Go Robots - Learning Environment")

	defaultCode := `package main

import "github.com/yesetoda/cs1gobot/robot"

func main() {
	hubo := robot.New("blue")
	hubo.SetDelay(100)
	hubo.SetTrace("cyan")
	for i := 0; i < 4; i++ {
		for s := 0; s < 4; s++ {
			hubo.Move()
		}
		hubo.TurnLeft()
	}
}
`
	robot.CreateWorld(10, 10)
	actions := make([]ui.DemoAction, 0, len(demos.Actions()))
	for _, action := range demos.Actions() {
		actions = append(actions, ui.DemoAction{Label: action.Label, Run: action.Run})
	}

	content := ui.MakeWindowContent(w, defaultCode, func(code string) {
		// Stop any previous execution
		robot.SetStop(true)
		// Small wait to ensure goroutine catches it
		time.Sleep(50 * time.Millisecond)
		robot.SetStop(false)
		robot.Reset()
		robot.BeginRun()
		ui.SetGoalFeedback("Running...")
		ui.RequestGridRefresh()
		go func() {
			err := engine.RunCode(code)
			if err != nil {
				if robot.IsConstraintReachedError(err) {
					eval := robot.EvaluateGoal()
					msg := eval.Message
					if eval.Details != "" {
						msg += "\n" + eval.Details
					}
					constraintText := strings.TrimSpace(strings.TrimPrefix(err.Error(), "ConstraintReached:"))
					if constraintText != "" {
						msg += "\nConstraint: " + constraintText
					}
					ui.SetGoalFeedback(msg)
					dialog.ShowInformation("Constraint reached", msg, w)
					return
				}

				if err.Error() != "Execution stopped by user" {
					log.Println("Error executing code:", err)
					ui.SetGoalFeedback("Run error: " + err.Error())
					dialog.ShowError(err, w)
					return
				}

				ui.SetGoalFeedback("Execution stopped by user.")
				return
			}

			eval := robot.EvaluateGoal()
			msg := eval.Message
			if eval.Details != "" {
				msg += "\n" + eval.Details
			}
			ui.SetGoalFeedback(msg)
		}()
	}, actions...)
	w.SetContent(content)
	w.ShowAndRun()
}
