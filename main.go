package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
)

var difficulties = map[string]int{
	"easy":   45,
	"medium": 50,
	"hard":   55,
	"insane": 60,
}

func main() {
	os.Exit(mainMain())
}
func mainMain() int {
	mode := flag.String("mode", "solve", "Operation mode {solve|solveStream|generate}")
	difficulty := flag.String("difficulty", "medium", "Difficulty of generated board {easy|medium|hard|insane|1-70}")
	showStats := flag.Bool("stats", false, "show solver statistics")
	flag.Parse()

	var err error
	switch *mode {
	case "solve":
		err = mainSolve(*showStats)
	case "solveStream":
		for err == nil {
			err = mainSolve(*showStats)
		}
		if err == io.EOF {
			err = nil
		}
	case "generate":
		err = mainGenerate(*difficulty)
	default:
		flag.Usage()
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}
	return 0
}

func mainSolve(showStats bool) error {
	b := NewBoard()
	_, err := b.ReadFrom(os.Stdin)
	if err != nil {
		return err
	}

	if !b.Solve() {
		return fmt.Errorf("invalid board: no solution")
	}

	fmt.Printf("%s", b.Art())

	if showStats {
		fmt.Printf("Stats:\n")
		fmt.Printf("  %-30s %8s %8s %14s\n", "Algorithm", "Calls", "Changes", "Duration (ns)")
		for _, a := range b.Algorithms {
			stats := a.Stats()
			fmt.Printf("  %-30s %8d %8d %14d\n", a.Name(), stats.Calls, stats.Changes, stats.Duration)
		}
		stats := b.guessStats
		fmt.Printf("  %-30s %8d %8d %14d\n", "guesser", stats.Calls, stats.Changes, stats.Duration)
	}

	return nil
}

func mainGenerate(difficulty string) error {
	lvl := difficulties[difficulty]
	if lvl == 0 {
		// try and parse as an int.
		var err error
		lvl, err = strconv.Atoi(difficulty)
		if err != nil || lvl < 0 {
			return fmt.Errorf("invalid difficulty level")
		}
	}

	b := NewRandomBoard(lvl)
	fmt.Printf("%s", b.Art())
	return nil
}
