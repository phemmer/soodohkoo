package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
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
		err = mainSolveOne(*showStats)
	case "solveStream":
		err = mainSolveStream(*showStats)
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

func mainSolveReader(input io.Reader, showStats bool) ([]byte, error) {
	b := NewBoard()
	_, err := b.ReadFrom(input)
	if err != nil {
		return nil, err
	}

	if !b.Solve() {
		return nil, fmt.Errorf("invalid board: no solution")
	}

	outa := b.Art()
	out := outa[:]

	if showStats {
		out = append(out, []byte(fmt.Sprintf("Stats:\n"))...)
		out = append(out, []byte(fmt.Sprintf("  %-30s %8s %8s %14s\n", "Algorithm", "Calls", "Changes", "Duration (ns)"))...)
		for _, a := range b.Algorithms {
			stats := a.Stats()
			out = append(out, []byte(fmt.Sprintf("  %-30s %8d %8d %14d\n", a.Name(), stats.Calls, stats.Changes, stats.Duration))...)
		}
		stats := b.guessStats
		out = append(out, []byte(fmt.Sprintf("  %-30s %8d %8d %14d\n", "guesser", stats.Calls, stats.Changes, stats.Duration))...)
	}

	return out, nil
}

func mainSolveOne(showStats bool) error {
	out, err := mainSolveReader(os.Stdin, showStats)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

type job struct {
	bs  []byte
	err error
	wg  sync.WaitGroup
}

func mainSolveStream(showStats bool) error {
	wg := sync.WaitGroup{}
	defer wg.Wait()

	workerCount := runtime.GOMAXPROCS(-1)

	workerJobs := make(chan *job, workerCount*4)
	defer close(workerJobs)
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			for job := range workerJobs {
				buf := bytes.NewBuffer(job.bs)
				job.bs, job.err = mainSolveReader(buf, showStats)
				job.wg.Done()
			}
			wg.Done()
		}()
	}

	publisherJobs := make(chan *job, workerCount*8)
	defer close(publisherJobs)
	wg.Add(1)
	go func() {
		for job := range publisherJobs {
			job.wg.Wait()
			if job.err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", job.err)
				os.Exit(1)
			}
			fmt.Printf("%s", job.bs)
		}
		wg.Done()
	}()

	for {
		job := &job{
			bs: make([]byte, 9*9*2),
		}
		_, err := io.ReadFull(os.Stdin, job.bs)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		job.wg.Add(1)
		workerJobs <- job
		publisherJobs <- job
	}
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
