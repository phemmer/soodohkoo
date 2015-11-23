package main

import (
	"flag"
	"fmt"
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
	var generateDifficulty string
	flag.StringVar(&generateDifficulty, "generate", "", "generate a board of the given difficulty {easy|medium|hard|insane|1-70}")
	var showStats bool
	flag.BoolVar(&showStats, "stats", false, "show solver statistics")
	flag.Parse()

	if generateDifficulty == "" {
		return mainSolve(showStats)
	}
	return mainGenerate(generateDifficulty)
}

func mainSolve(showStats bool) int {
	b := NewBoard()
	_, err := b.ReadFrom(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid board: %s\n", err)
		return 1
	}

	if !b.Solve() {
		fmt.Fprintf(os.Stderr, "invalid board: no solution\n")
		return 1
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

	return 0
}

func mainGenerate(difficulty string) int {
	lvl := difficulties[difficulty]
	if lvl == 0 {
		// try and parse as an int.
		var err error
		lvl, err = strconv.Atoi(difficulty)
		if err != nil || lvl < 0 {
			fmt.Fprintf(os.Stderr, "Invalid difficulty level\n")
			flag.PrintDefaults()
			return 1
		}
	}

	b := NewRandomBoard(lvl)
	fmt.Printf("%s", b.Art())
	return 0
}
