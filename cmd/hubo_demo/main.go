package main

import (
	"time"

	"fyne.io/fyne/v2/app"
	"github.com/yesetoda/cs1gobot/demos"
	"github.com/yesetoda/cs1gobot/robot"
	"github.com/yesetoda/cs1gobot/ui"
)

func main() {
	a := app.NewWithID("github.com.yesetoda.cs1gobot.compiled")
	w := a.NewWindow("Go Robots - Compiled Demo")

	// Initial world for the demo.
	robot.CreateWorld(10, 10)
	actions := make([]ui.DemoAction, 0, len(demos.Actions()))
	for _, action := range demos.Actions() {
		actions = append(actions, ui.DemoAction{Label: action.Label, Run: action.Run})
	}

	// Build UI in compiled mode (editor run button disabled).
	content := ui.MakeWindowContent(w, compiledExampleCode, nil, actions...)
	w.SetContent(content)
	w.Show()

	// Run one demo automatically after UI wiring is active.
	go func() {
		time.Sleep(200 * time.Millisecond)
		demos.StartSquareDemo()
		if robot.CurrentWorld != nil {
			robot.CurrentWorld.SetUpdateFunc(robot.UpdateUI)
			ui.RequestGridRefresh()
		}
	}()

	a.Run()
}

const compiledExampleCode = `package main

import "github.com/yesetoda/cs1gobot/robot"

func main() {
	hubo := robot.New("blue")
	hubo.SetDelay(120)
	hubo.SetTrace("cyan")
	for i := 0; i < 4; i++ {
		for s := 0; s < 4; s++ {
			hubo.Move()
		}
		hubo.TurnLeft()
	}
}
`
