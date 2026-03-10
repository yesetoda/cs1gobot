package robot

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// LoadWorld parses a Python `.wld` file to configure the CurrentWorld.
func LoadWorld(filename string) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	content := string(b)

	av := extractInt(content, `avenues\s*=\s*(\d+)`, 10)
	st := extractInt(content, `streets\s*=\s*(\d+)`, 10)

	CreateWorld(av, st)

	// Walls format: (col, row)
	// Example: walls = [ (col, row), (col2, row2) ]
	wallsText := extractBlock(content, `(?s)walls\s*=\s*\[(.*?)\]`)
	if wallsText != "" {
		reTuple := regexp.MustCompile(`\(\s*(\d+)\s*,\s*(\d+)\s*\)`)
		matches := reTuple.FindAllStringSubmatch(wallsText, -1)
		for _, m := range matches {
			col, _ := strconv.Atoi(m[1])
			row, _ := strconv.Atoi(m[2])

			// In Python:
			// col % 2 == 0 -> vertical wall. x1, y1 = (col, row-1) and x2, y2 = (col, row+1)
			// Wait, grid coordinates map:
			// col = 2 * avenue - 1, row = 2 * street - 1 for centers.
			if col%2 == 0 { // vertical wall. Blocks avenue col/2 from col/2+1
				CurrentWorld.AddWall(Point{X: col / 2, Y: (row + 1) / 2}, Point{X: col/2 + 1, Y: (row + 1) / 2})
			} else { // horizontal wall. Blocks street row/2 from row/2+1
				CurrentWorld.AddWall(Point{X: (col + 1) / 2, Y: row / 2}, Point{X: (col + 1) / 2, Y: row/2 + 1})
			}
		}
	}

	// Beepers format: (av, st): count
	beepersText := extractBlock(content, `(?s)beepers\s*=\s*\{(.*?)\}`)
	if beepersText != "" {
		reDict := regexp.MustCompile(`\(\s*(\d+)\s*,\s*(\d+)\s*\)\s*:\s*(\d+)`)
		matches := reDict.FindAllStringSubmatch(beepersText, -1)
		for _, m := range matches {
			av, _ := strconv.Atoi(m[1])
			st, _ := strconv.Atoi(m[2])
			count, _ := strconv.Atoi(m[3])
			for i := 0; i < count; i++ {
				CurrentWorld.AddBeeper(av, st)
			}
		}
	}

	return nil
}

// SaveWorld dumps CurrentWorld to a Python `.wld` file.
func SaveWorld(filename string) error {
	if CurrentWorld == nil {
		return fmt.Errorf("no current world initialized")
	}

	av, st := CurrentWorld.GetAvenuesAndStreets()
	walls, beepers := CurrentWorld.GetSnapshot()

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "avenues = %d\n", av)
	fmt.Fprintf(f, "streets = %d\n", st)

	if len(walls) > 0 {
		fmt.Fprintf(f, "walls = [\n")
		// Convert Point back to (col, row)
		for w := range walls {
			var col, row int
			if w.P1.X == w.P2.X {
				// horizontal wall
				col = w.P1.X*2 - 1
				row = w.P1.Y * 2
			} else if w.P1.Y == w.P2.Y {
				// vertical wall
				col = w.P1.X * 2
				row = w.P1.Y*2 - 1
			}
			fmt.Fprintf(f, "    (%d, %d), \n", col, row)
		}
		fmt.Fprintf(f, "]\n")
	} else {
		fmt.Fprintf(f, "walls = []\n")
	}

	if len(beepers) > 0 {
		fmt.Fprintf(f, "beepers = {\n")
		for pt, count := range beepers {
			fmt.Fprintf(f, "    (%d, %d): %d, \n", pt.X, pt.Y, count)
		}
		fmt.Fprintf(f, "}\n")
	} else {
		fmt.Fprintf(f, "beepers = {}\n")
	}

	return nil
}

func extractInt(content, pattern string, def int) int {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		val, err := strconv.Atoi(matches[1])
		if err == nil {
			return val
		}
	}
	return def
}

func extractBlock(content, blockPattern string) string {
	reBlock := regexp.MustCompile(blockPattern)
	blockMatch := reBlock.FindStringSubmatch(content)
	if len(blockMatch) == 0 {
		return ""
	}
	return blockMatch[1]
}
