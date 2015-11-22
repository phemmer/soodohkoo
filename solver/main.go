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
	var generateDifficulty string
	flag.StringVar(&generateDifficulty, "generate", "", "generate a board of the given difficulty {easy|medium|hard|insane|1-70}")
	flag.Parse()

	if generateDifficulty == "" {
		mainSolve()
		os.Exit(0)
	}
	mainGenerate(generateDifficulty)
}

func mainSolve() {
	b := NewBoard()
	_, err := b.ReadFrom(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid board: %s\n", err)
		os.Exit(1)
	}

	if !b.Solve() {
		fmt.Fprintf(os.Stderr, "invalid board: no solution\n")
		os.Exit(1)
	}

	fmt.Printf("%s", b.Art())

	fmt.Printf("Stats:\n")
	fmt.Printf("  %-30s %8s %8s %14s\n", "Algorithm", "Calls", "Changes", "Duration (ns)")
	for _, a := range b.Algorithms {
		stats := a.Stats()
		fmt.Printf("  %-30s %8d %8d %14d\n", a.Name(), stats.Calls, stats.Changes, stats.Duration)
	}
	stats := b.guessStats
	fmt.Printf("  %-30s %8d %8d %14d\n", "guesser", stats.Calls, stats.Changes, stats.Duration)

	os.Exit(0)
}

func mainGenerate(difficulty string) {
	lvl := difficulties[difficulty]
	if lvl == 0 {
		// try and parse as an int.
		var err error
		lvl, err = strconv.Atoi(difficulty)
		if err != nil || lvl < 0 {
			fmt.Fprintf(os.Stderr, "Invalid difficulty level\n")
			flag.PrintDefaults()
			os.Exit(1)
		}
	}

	b := NewRandomBoard(lvl)
	fmt.Printf("%s", b.Art())
}
