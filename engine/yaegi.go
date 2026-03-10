package engine

import (
	"fmt"
	"go/build"
	"reflect"
	"regexp"
	"strings"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/yesetoda/cs1gobot/robot"
)

var packageDeclRE = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)

func normalizeSourceForRun(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return "package main\n\nfunc main() {}\n"
	}

	idx := packageDeclRE.FindStringSubmatchIndex(code)
	if idx == nil {
		return "package main\n\n" + code
	}

	pkgName := code[idx[2]:idx[3]]
	if pkgName == "main" {
		return code
	}

	return code[:idx[0]] + "package main" + code[idx[1]:]
}

var robotSymbolTable = map[string]reflect.Value{
	"New":                  reflect.ValueOf(robot.New),
	"NewAt":                reflect.ValueOf(robot.NewAt),
	"LoadWorld":            reflect.ValueOf(robot.LoadWorld),
	"SaveWorld":            reflect.ValueOf(robot.SaveWorld),
	"CreateWorld":          reflect.ValueOf(robot.CreateWorld),
	"Reset":                reflect.ValueOf(robot.Reset),
	"SetDefaultDelay":      reflect.ValueOf(robot.SetDefaultDelay),
	"SetDefaultTraceColor": reflect.ValueOf(robot.SetDefaultTraceColor),
	"SetStop":              reflect.ValueOf(robot.SetStop),
	"WorldDimensions":      reflect.ValueOf(robot.WorldDimensions),
	"WorldBeeperLocations": reflect.ValueOf(robot.WorldBeeperLocations),
	"WorldBeeperTotal":     reflect.ValueOf(robot.WorldBeeperTotal),
	"WorldWallCount":       reflect.ValueOf(robot.WorldWallCount),
	"WorldWalls":           reflect.ValueOf(robot.WorldWalls),
	"WorldRobotStates":     reflect.ValueOf(robot.WorldRobotStates),
	"WorldDetails":         reflect.ValueOf(robot.WorldDetails),
	"Robot":                reflect.ValueOf((*robot.Robot)(nil)),
	"RobotState":           reflect.ValueOf((*robot.RobotState)(nil)),
	"World":                reflect.ValueOf((*robot.World)(nil)),
	"Point":                reflect.ValueOf((*robot.Point)(nil)),
	"Direction":            reflect.ValueOf((*robot.Direction)(nil)),
	"North":                reflect.ValueOf(robot.North),
	"West":                 reflect.ValueOf(robot.West),
	"South":                reflect.ValueOf(robot.South),
	"East":                 reflect.ValueOf(robot.East),
}

// RobotSymbols exposes the robot package API to the Yaegi interpreter.
//
// Callers typically pass this value to interp.Use so interpreted source can
// import and use the robot package like compiled Go code.
var RobotSymbols = func() interp.Exports {
	// Yaegi exports are keyed as "import/path/packageName" (for example
	// "fmt/fmt" in stdlib). Register both short import style and the actual
	// module import path derived from the compiled robot package.
	exports := interp.Exports{"robot/robot": robotSymbolTable}
	fullPath := reflect.TypeOf(robot.World{}).PkgPath()
	if fullPath != "" {
		exports[fullPath+"/robot"] = robotSymbolTable
	}
	return exports
}()

// RunCode executes the provided Go source using the yaegi interpreter and
// returns any error that occurs. Robot panics are recovered and turned into
// regular errors so the caller can display a friendly message.
func RunCode(code string) (err error) {
	i := interp.New(interp.Options{GoPath: build.Default.GOPATH})
	if useErr := i.Use(stdlib.Symbols); useErr != nil {
		return useErr
	}
	if useErr := i.Use(RobotSymbols); useErr != nil {
		return useErr
	}

	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			default:
				err = fmt.Errorf("%v", v)
			}
		}
	}()

	_, err = i.Eval(normalizeSourceForRun(code))
	return err
}
