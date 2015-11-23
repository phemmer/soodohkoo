package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func runMain(t *testing.T, input io.Reader, args ...string) (int, *bytes.Buffer) {
	defer func(fs *flag.FlagSet) { flag.CommandLine = fs }(flag.CommandLine)
	defer func(args []string) { os.Args = args }(os.Args)
	flag.CommandLine = flag.NewFlagSet("soodohkoo", 0)
	os.Args = append([]string{"soodohkoo"}, args...)

	wg := sync.WaitGroup{}

	if input == nil {
		input = bytes.NewBuffer(nil)
	}
	defer func(f *os.File) { os.Stdin = f }(os.Stdin)
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("error creating pipe: %s", err)
	}
	defer stdinR.Close()
	defer stdinW.Close()
	os.Stdin = stdinR
	wg.Add(1)
	go func() { io.Copy(stdinW, input); wg.Done() }()

	defer func(f *os.File) { os.Stdout = f }(os.Stdout)
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("error creating pipe: %s", err)
	}
	defer stdoutW.Close()
	defer stdoutR.Close()
	os.Stdout = stdoutW
	stdoutBuf := bytes.NewBuffer(nil)
	wg.Add(1)
	go func() { io.Copy(stdoutBuf, stdoutR); wg.Done() }()

	status := mainMain()
	stdinR.Close()  // unblock STDIN io.Copy()
	stdoutW.Close() // unblock STDOUT io.Copy()
	wg.Wait()

	return status, stdoutBuf
}

func TestMainSolve(t *testing.T) {
	input := strings.NewReader(`_ 8 _ _ 6 _ _ _ _
5 4 _ _ _ 7 _ 3 _
_ _ _ 1 _ _ 8 6 7
_ _ 9 _ 3 _ _ _ 6
_ _ 5 _ _ _ 3 _ _
3 _ _ _ 4 _ 2 _ _
7 5 4 _ _ 6 _ _ _
_ 2 _ 4 _ _ _ 7 9
_ _ _ _ 2 _ _ 8 _
`)
	status, output := runMain(t, input, "-stats")
	if status != 0 {
		t.Errorf("mainSolve() returned %d, expected %d", status, 0)
	}

	//TODO check stats?

	b := NewBoard()
	_, err := b.ReadFrom(output)
	if err != nil {
		t.Errorf("error reading output board: %s", err)
	}
	if !b.Solved() {
		t.Errorf("output board is not solved")
	}
}

func TestMainGenerate(t *testing.T) {
	status, output := runMain(t, nil, "-generate=easy")
	if status != 0 {
		t.Errorf("mainGenerate() returned %d, expected %d", status, 0)
	}

	b := NewBoard()
	_, err := b.ReadFrom(output)
	if err != nil {
		t.Errorf("error reading output board: %s", err)
	}

	unknownCount := 0
	for _, t := range b.Tiles {
		if !t.isKnown() {
			unknownCount++
		}
	}
	if unknownCount != difficulties["easy"] {
		t.Errorf("have %d unknown tiles, expected %d", unknownCount, difficulties["easy"])
	}
}

func TestMainGenerate_difficultyInt(t *testing.T) {
	status, output := runMain(t, nil, "-generate=3")
	if status != 0 {
		t.Errorf("mainGenerate() returned %d, expected %d", status, 0)
	}

	b := NewBoard()
	_, err := b.ReadFrom(output)
	if err != nil {
		t.Errorf("error reading output board: %s", err)
	}

	unknownCount := 0
	for _, t := range b.Tiles {
		if !t.isKnown() {
			unknownCount++
		}
	}
	if unknownCount != 3 {
		t.Errorf("have %d unknown tiles, expected %d", unknownCount, 3)
	}
}
