package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

type Tile uint16 // 9-bit mask of the possible digits
type Region [9]Tile
type Board [9]Region
type TileRef struct {
	RegionIndex uint8
	TileIndex   uint8
}

const tAny = Tile((1 << 9) - 1)

var byteToTileMap = map[byte]Tile{
	'1': 1 << 0, // 0b000000001
	'2': 1 << 1, // 0b000000010
	'3': 1 << 2, // 0b000000100
	'4': 1 << 3, // 0b000001000
	'5': 1 << 4, // 0b000010000
	'6': 1 << 5, // 0b000100000
	'7': 1 << 6, // 0b001000000
	'8': 1 << 7, // 0b010000000
	'9': 1 << 8, // 0b100000000
	'_': tAny,   // 0b111111111
}

// returns true if the tile only has a single possible value
func (t Tile) isKnown() bool {
	// http://graphics.stanford.edu/~seander/bithacks.html#DetermineIfPowerOf2
	return (t & (t - 1)) == 0
}

func (t Tile) Num() uint8 {
	if !t.isKnown() {
		return 0
	}
	// http://graphics.stanford.edu/~seander/bithacks.html#IntegerLogDeBruijn
	lookupTable := [32]uint8{
		0, 1, 28, 2, 29, 14, 24, 3, 30, 22, 20, 15, 25, 17, 4, 8,
		31, 27, 13, 23, 21, 19, 16, 7, 26, 12, 18, 6, 11, 5, 10, 9,
	}
	//TODO adjust the table to remove the <<1
	// the table is also larger than we need
	// it should also be global so it's not redeclared
	return lookupTable[(uint32(t<<1)*0x077CB531)>>27]
}

// converts board x,y coordinates into a region & tile index
func xyToIndices(x, y uint8) (ri, ti uint8) {
	ri = (y/3)*3 + x/3
	ti = (y%3)*3 + x%3
	return
}

func indicesToXY(ri, ti uint8) (x, y uint8) {
	x = (ri%3)*3 + ti%3
	y = (ri/3)*3 + ti/3
	return
}

func NewBoard() Board {
	return Board{
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
		Region{tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny},
	}
}

func (b *Board) RefRegion(ri uint8) [9]TileRef {
	return [9]TileRef{
		{ri, 0},
		{ri, 1},
		{ri, 2},
		{ri, 3},
		{ri, 4},
		{ri, 5},
		{ri, 6},
		{ri, 7},
		{ri, 8},
	}
}
func (b *Board) RowRefs(y uint8) [9]TileRef {
	rRow := y / 3
	tRow := y % 3
	tStart := tRow * 3

	return [9]TileRef{
		{rRow*3 + 0, tStart + 0},
		{rRow*3 + 0, tStart + 1},
		{rRow*3 + 0, tStart + 2},
		{rRow*3 + 1, tStart + 0},
		{rRow*3 + 1, tStart + 1},
		{rRow*3 + 1, tStart + 2},
		{rRow*3 + 2, tStart + 0},
		{rRow*3 + 2, tStart + 1},
		{rRow*3 + 2, tStart + 2},
	}
}
func (b *Board) ColRefs(x uint8) [9]TileRef {
	rCol := x / 3
	tCol := x % 3
	tStart := tCol

	return [9]TileRef{
		{rCol + 0, tStart + 0},
		{rCol + 0, tStart + 3},
		{rCol + 0, tStart + 6},
		{rCol + 3, tStart + 0},
		{rCol + 3, tStart + 3},
		{rCol + 3, tStart + 6},
		{rCol + 6, tStart + 0},
		{rCol + 6, tStart + 3},
		{rCol + 6, tStart + 6},
	}
}

// Set tries to set the given indices to the given Tile.
// The tile set on the board might be different than the one provided if
// possiblities can be eliminated.
// Returns the tile actually set, and 0 if not possible.
func (b *Board) Set(ri, ti uint8, t Tile) Tile {
	t0 := b[ri][ti]
	if t == t0 {
		// already set
		return t
	}

	// discard possible values based on the current tile mask
	t &= t0
	if t == 0 {
		// not possible captain
		return t
	}

	b[ri][ti] = t

	if !t.isKnown() {
		// it has multiple possible values, so we can't eliminated possiblities
		// from our neighbors
		return t
	}
	// ok, it has only a single possible value, so remove the value from
	// neighbors possiblities

	// back up the entire board. The other option is to maintain a list of
	// neighbor changes.
	// We also do this after the !t.isKnown() check above as the board is somewhat
	// large, and the !t.isKnown() check is likely to prevent us from getting
	// here.
	b0 := *b
	b0[ri][ti] = t0

	// iterate over the region
	for nti := range b[ri] {
		if uint8(nti) == ti {
			// skip ourself
			continue
		}
		nt := b[ri][nti]
		if b.Set(ri, uint8(nti), nt&^t) == 0 {
			// invalid board configuration, revert the change
			*b = b0
			return 0
		}
	}

	x, y := indicesToXY(ri, ti)
	rowRefs := b.RowRefs(y)
	colRefs := b.ColRefs(x)

	// iterate over the row
	for _, ntr := range rowRefs {
		if ntr.RegionIndex == ri && ntr.TileIndex == ti {
			// skip ourself
			continue
		}
		if b.Set(ntr.RegionIndex, ntr.TileIndex, b[ntr.RegionIndex][ntr.TileIndex]&^t) == 0 {
			// invalid board configuration, revert the change
			*b = b0
			return 0
		}
	}

	// iterate over the column
	for _, ntr := range colRefs {
		if ntr.RegionIndex == ri && ntr.TileIndex == ti {
			// skip ourself
			continue
		}
		if b.Set(ntr.RegionIndex, ntr.TileIndex, b[ntr.RegionIndex][ntr.TileIndex]&^t) == 0 {
			// invalid board configuration, revert the change
			*b = b0
			return 0
		}
	}

	// ok, now scan each set of neighbors for any values which only have one
	// possible tile

	// iterate over the region
OnePossibleTileRegionLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		//TODO this feels like there should have an optimized way to find which bits are set in only one of a set of numbers
		tcTI := uint8(10)
		for nti, nt := range b[ri] {
			if nt == v {
				// this value already has been found
				continue OnePossibleTileRegionLoop
			}
			if nt&v == 0 {
				// not a possible tile
				continue
			}
			// is a candidate
			if tcTI != 10 {
				// this is the second candidate
				continue OnePossibleTileRegionLoop
			}
			tcTI = uint8(nti)
		}
		if tcTI == 10 {
			// no possible tiles for this value
			//TODO does this ever happen?
			*b = b0
			return 0
		}
		if b.Set(ri, tcTI, v) == 0 {
			// invalid board configuration
			//TODO does this ever happen?
			*b = b0
			return 0
		}
	}

OnePossibleTileRowLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		var tcRef TileRef
		for _, ntr := range rowRefs {
			nt := b[ntr.RegionIndex][ntr.TileIndex]
			if nt == v {
				continue OnePossibleTileRowLoop
			}
			if nt&v == 0 {
				continue
			}
			if tcRef != (TileRef{}) {
				continue OnePossibleTileRowLoop
			}
			tcRef = ntr
		}
		if tcRef == (TileRef{}) {
			*b = b0
			return 0
		}
		if b.Set(tcRef.RegionIndex, tcRef.TileIndex, v) == 0 {
			*b = b0
			return 0
		}
	}

OnePossibleTileColumnLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		var tcRef TileRef
		for _, ntr := range colRefs {
			nt := b[ntr.RegionIndex][ntr.TileIndex]
			if nt == v {
				continue OnePossibleTileColumnLoop
			}
			if nt&v == 0 {
				continue
			}
			if tcRef != (TileRef{}) {
				continue OnePossibleTileColumnLoop
			}
			tcRef = ntr
		}
		if tcRef == (TileRef{}) {
			*b = b0
			return 0
		}
		if b.Set(tcRef.RegionIndex, tcRef.TileIndex, v) == 0 {
			*b = b0
			return 0
		}
	}

	// only-row elimination
	// see if there is only a single row or column within a region which can hold a value. If so, eliminate neighboring regions from holding that value in the same row.

	// row first
OnePossibleRowLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		tcRow := uint8(10)
		for nti, nt := range b[ri] {
			if nt == v {
				// this value has already been found
				continue OnePossibleRowLoop
			}
			if nt&v == 0 {
				// not a possible tile
				continue
			}
			_, y := indicesToXY(ri, uint8(nti))
			if tcRow == y {
				// row already a candidate
				continue
			}
			if tcRow != 10 {
				// multiple candidate rows
				continue OnePossibleRowLoop
			}
			tcRow = y
		}
		if tcRow == 10 {
			// no candidate rows. Wat?
			*b = b0
			return 0
		}

		// iterate over the candidate row, excluding the value from tiles in other regions
		tcRowRef := b.RowRefs(tcRow)
		for _, ntr := range tcRowRef {
			if ntr.RegionIndex == ri {
				// skip our region
				continue
			}
			if b.Set(ntr.RegionIndex, ntr.TileIndex, b[ntr.RegionIndex][ntr.TileIndex]&^v) == 0 {
				// invalid board configuration, revert the change
				*b = b0
				return 0
			}
		}
	}

OnePossibleColumnLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		tcCol := uint8(10)
		for nti, nt := range b[ri] {
			if nt == v {
				// this value has already been found
				continue OnePossibleColumnLoop
			}
			if nt&v == 0 {
				// not a possible tile
				continue
			}
			x, _ := indicesToXY(ri, uint8(nti))
			if tcCol == x {
				// column already a candidate
				continue
			}
			if tcCol != 10 {
				// multiple candidate columns
				continue OnePossibleColumnLoop
			}
			tcCol = x
		}
		if tcCol == 10 {
			// no candidate columns. Wat?
			*b = b0
			return 0
		}

		tcColRef := b.ColRefs(tcCol)
		for _, ntr := range tcColRef {
			if ntr.RegionIndex == ri {
				// skip our region
				continue
			}
			if b.Set(ntr.RegionIndex, ntr.TileIndex, b[ntr.RegionIndex][ntr.TileIndex]&^v) == 0 {
				// invalid board configuration, revert the change
				*b = b0
				return 0
			}
		}
	}

	return t
}

func (b *Board) ReadFrom(r io.Reader) (int64, error) {
	var ba [9 * 9 * 2]byte
	nr, err := io.ReadAtLeast(r, ba[:], len(ba)-1)
	// io.ReadAtLeast() because we don't care about a trailing newline if it's there
	if err != nil {
		return int64(nr), err
	}
	for i := 0; i < len(ba); i += 2 {
		x := uint8(i / 2 % 9)
		y := uint8(i / 2 / 9)
		ri, ti := xyToIndices(x, y)
		t := byteToTileMap[ba[i]]
		if t == 0 {
			return int64(nr), errors.New("invalid byte")
		}
		b.Set(ri, ti, t)
		//b[ri][ti] = byteToTileMap[ba[i]]
	}

	return int64(nr), nil
}

func (b Board) Art() [9 * 9 * 2]byte {
	var ba [9 * 9 * 2]byte
	for y := uint8(0); y < 9; y++ {
		rowStart := y * 9 * 2
		for x, tr := range b.RowRefs(y) {
			t := b[tr.RegionIndex][tr.TileIndex]
			i := rowStart + uint8(x)*2
			ba[i] = '0' + t.Num()
			if ba[i] == '0' {
				ba[i] = '_'
			}
			ba[i+1] = ' '
		}
		ba[rowStart+8*2+1] = '\n'
	}
	return ba
}

func main() {
	b := NewBoard()
	b.ReadFrom(os.Stdin)
	fmt.Printf("%s\n", b.Art())
}
