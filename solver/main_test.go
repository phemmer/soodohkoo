package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"sync"
	"testing"
)

func runMain(args ...string) int {
	defer func(fs *flag.FlagSet) { flag.CommandLine = fs }(flag.CommandLine)
	defer func(args []string) { os.Args = args }(os.Args)
	flag.CommandLine = flag.NewFlagSet("soodohkoo", 0)
	os.Args = append([]string{"soodohkoo"}, args...)
	return mainMain()
}

func TestMainSolve(t *testing.T) {
	defer func(f *os.File) { os.Stdin = f }(os.Stdin)
	defer func(f *os.File) { os.Stdout = f }(os.Stdout)
	defer func(f *os.File) { os.Stderr = f }(os.Stderr)

	var stdinW *os.File
	var err error
	os.Stdin, stdinW, err = os.Pipe()
	if err != nil {
		t.Fatalf("error creating pipe: %s", err)
	}
	defer os.Stdin.Close()

	go func() {
		defer stdinW.Close()
		_, err := stdinW.WriteString(`_ 8 _ _ 6 _ _ _ _
5 4 _ _ _ 7 _ 3 _
_ _ _ 1 _ _ 8 6 7
_ _ 9 _ 3 _ _ _ 6
_ _ 5 _ _ _ 3 _ _
3 _ _ _ 4 _ 2 _ _
7 5 4 _ _ 6 _ _ _
_ 2 _ 4 _ _ _ 7 9
_ _ _ _ 2 _ _ 8 _
`)
		if err != nil {
			t.Errorf("error writing to pipe: %s", err)
		}
	}()

	var stdoutR *os.File
	stdoutR, os.Stdout, err = os.Pipe()
	if err != nil {
		t.Fatalf("error creating pipe: %s", err)
	}
	stdoutBuf := bytes.NewBuffer(nil)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { io.Copy(stdoutBuf, stdoutR); wg.Done() }()

	// Suppress the stats noise.
	// We might test it one day, but for now we don't care.
	//os.Stderr, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)

	status := runMain("-stats")
	if status != 0 {
		t.Errorf("mainSolve() returned %d, expected %d", status, 0)
	}
	os.Stdout.Close() // unblock the STDOUT io.Copy()

	wg.Wait()
	b := NewBoard()
	_, err = b.ReadFrom(stdoutBuf)
	if err != nil {
		t.Errorf("error reading output board: %s", err)
	}

	//TODO check stats?

	if !b.Solved() {
		t.Errorf("output board is not solved")
	}
}

func TestMainGenerate(t *testing.T) {
	defer func(f *os.File) { os.Stdout = f }(os.Stdout)

	var err error
	var stdoutR *os.File
	stdoutR, os.Stdout, err = os.Pipe()
	if err != nil {
		t.Fatalf("error creating pipe: %s", err)
	}
	stdoutBuf := bytes.NewBuffer(nil)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { io.Copy(stdoutBuf, stdoutR); wg.Done() }()

	defer func(d int) { difficulties["easy"] = d }(difficulties["easy"])
	difficulties["easy"] = 4 // to speed up the test
	status := runMain("-generate=easy")
	if status != 0 {
		t.Errorf("mainGenerate() returned %d, expected %d", status, 0)
	}
	os.Stdout.Close() // unblock the STDOUT io.Copy()

	wg.Wait()
	b := NewBoard()
	_, err = b.ReadFrom(stdoutBuf)
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
	defer func(f *os.File) { os.Stdout = f }(os.Stdout)

	var err error
	var stdoutR *os.File
	stdoutR, os.Stdout, err = os.Pipe()
	if err != nil {
		t.Fatalf("error creating pipe: %s", err)
	}
	stdoutBuf := bytes.NewBuffer(nil)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { io.Copy(stdoutBuf, stdoutR); wg.Done() }()

	status := runMain("-generate=3")
	if status != 0 {
		t.Errorf("mainGenerate() returned %d, expected %d", status, 0)
	}
	os.Stdout.Close() // unblock the STDOUT io.Copy()

	wg.Wait()
	b := NewBoard()
	_, err = b.ReadFrom(stdoutBuf)
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
