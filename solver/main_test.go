package main

import (
	"strings"
	"testing"
)

func BenchmarkSet(b *testing.B) {
	boardReader1 := strings.NewReader(`_ 8 _ _ 6 _ _ _ _
5 4 _ _ _ 7 _ 3 _
_ _ _ 1 _ _ 8 6 7
_ _ 9 _ 3 _ _ _ 6
_ _ 5 _ _ _ 3 _ _
3 _ _ _ 4 _ 2 _ _
7 5 4 _ _ 6 _ _ _
_ 2 _ 4 _ _ _ 7 9
_ _ _ _ 2 _ _ 8 _
`)
	boardReader2 := strings.NewReader(`_ _ 7 6 _ _ _ 9 _
_ 3 6 _ _ _ _ 7 8
_ _ 8 _ 3 2 _ _ _
_ 7 _ _ _ _ 6 _ _
6 _ _ 3 _ 4 _ _ 2
_ _ 1 _ _ _ _ 4 _
_ _ _ 9 5 _ 4 _ _
8 4 _ _ _ _ 5 2 _
_ 5 _ _ _ 3 7 _ _
`)
	boardReader3 := strings.NewReader(`_ 8 _ _ 2 _ _ _ _
9 7 _ _ _ 4 _ 2 _
_ _ _ 6 _ _ 4 5 7
_ _ 2 _ 4 _ _ _ 3
_ _ 3 _ _ _ 5 _ _
6 _ _ _ 3 _ 9 _ _
7 6 8 _ _ 1 _ _ _
_ 3 _ 7 _ _ _ 4 5
_ _ _ _ 6 _ _ 8 _
`)
	for i := 0; i < b.N; i++ {
		board := NewBoard()
		boardReader1.Seek(0, 0)
		board.ReadFrom(boardReader1)

		board = NewBoard()
		boardReader2.Seek(0, 0)
		board.ReadFrom(boardReader2)

		board = NewBoard()
		boardReader3.Seek(0, 0)
		board.ReadFrom(boardReader3)
	}
}
