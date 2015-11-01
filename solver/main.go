package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

type Tile uint16 // 9-bit mask of the possible digits
type Board [9 * 9]Tile

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

// converts board x,y coordinates into an index
func xyToIndex(x, y uint8) (idx uint8) {
	return y*9 + x
}

func indexToXY(idx uint8) (x, y uint8) {
	return idx % 9, idx / 9
}

//TODO bench indexToX/indexToY

func indexToRegionIndex(idx uint8) uint8 {
	return idx/3%3 + idx/(9*3)*3
}

func RegionIndices(ri uint8) [9]uint8 {
	idx0 := (ri / 3 * 27) + (ri % 3 * 3)
	return [9]uint8{
		idx0 + 9*0 + 0,
		idx0 + 9*0 + 1,
		idx0 + 9*0 + 2,
		idx0 + 9*1 + 0,
		idx0 + 9*1 + 1,
		idx0 + 9*1 + 2,
		idx0 + 9*2 + 0,
		idx0 + 9*2 + 1,
		idx0 + 9*2 + 2,
	}
}

func RowIndices(y uint8) [9]uint8 {
	idx0 := y * 9
	return [9]uint8{
		idx0 + 0,
		idx0 + 1,
		idx0 + 2,
		idx0 + 3,
		idx0 + 4,
		idx0 + 5,
		idx0 + 6,
		idx0 + 7,
		idx0 + 8,
	}
}

func ColumnIndices(x uint8) [9]uint8 {
	return [9]uint8{
		x + 9*0,
		x + 9*1,
		x + 9*2,
		x + 9*3,
		x + 9*4,
		x + 9*5,
		x + 9*6,
		x + 9*7,
		x + 9*8,
	}
}

func NewBoard() Board {
	return Board{
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
	}
}

// Set tries to set the given indices to the given Tile.
// The tile set on the board might be different than the one provided if
// possiblities can be eliminated.
// Returns the tile actually set, and 0 if not possible.
func (b *Board) Set(ti uint8, t Tile) Tile {
	t0 := b[ti]
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

	b[ti] = t

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
	b0[ti] = t0

	x, y := indexToXY(ti)
	rgnIdx := indexToRegionIndex(ti)

	rgnIndices := RegionIndices(rgnIdx)

	// iterate over the region
	for _, nti := range rgnIndices[:] {
		if nti == ti {
			// skip ourself
			continue
		}
		if b.Set(nti, b[nti]&^t) == 0 {
			// invalid board configuration, revert the change
			*b = b0
			return 0
		}
	}

	rowIndices := RowIndices(y)

	// iterate over the row
	for _, nti := range rowIndices[:] {
		if nti == ti {
			// skip ourself
			continue
		}
		if b.Set(nti, b[nti]&^t) == 0 {
			// invalid board configuration, revert the change
			*b = b0
			return 0
		}
	}

	colIndices := ColumnIndices(x)

	// iterate over the column
	for _, nti := range colIndices[:] {
		if nti == ti {
			// skip ourself
			continue
		}
		if b.Set(nti, b[nti]&^t) == 0 {
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
		tcIdx := uint8(255)
		for _, nti := range rgnIndices[:] {
			nt := b[nti]
			if nt == v {
				// this value already has been found
				continue OnePossibleTileRegionLoop
			}
			if nt&v == 0 {
				// not a possible tile
				continue
			}
			// is a candidate
			if tcIdx != 255 {
				// this is the second candidate
				continue OnePossibleTileRegionLoop
			}
			tcIdx = nti
		}
		if tcIdx == 255 {
			// no possible tiles for this value
			//TODO does this ever happen?
			*b = b0
			return 0
		}
		if b.Set(tcIdx, v) == 0 {
			// invalid board configuration
			//TODO does this ever happen?
			*b = b0
			return 0
		}
	}

OnePossibleTileRowLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		tcIdx := uint8(255)
		for _, nti := range rowIndices[:] {
			nt := b[nti]
			if nt == v {
				continue OnePossibleTileRowLoop
			}
			if nt&v == 0 {
				continue
			}
			if tcIdx != 255 {
				continue OnePossibleTileRowLoop
			}
			tcIdx = nti
		}
		if tcIdx == 255 {
			*b = b0
			return 0
		}
		if b.Set(tcIdx, v) == 0 {
			*b = b0
			return 0
		}
	}

OnePossibleTileColumnLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		tcIdx := uint8(255)
		for _, nti := range colIndices[:] {
			nt := b[nti]
			if nt == v {
				continue OnePossibleTileColumnLoop
			}
			if nt&v == 0 {
				continue
			}
			if tcIdx != 255 {
				continue OnePossibleTileColumnLoop
			}
			tcIdx = nti
		}
		if tcIdx == 255 {
			*b = b0
			return 0
		}
		if b.Set(tcIdx, v) == 0 {
			*b = b0
			return 0
		}
	}

	// only-row elimination
	// see if there is only a single row or column within a region which can hold a value. If so, eliminate neighboring regions from holding that value in the same row.

	// row first
OnePossibleRowLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		tcRow := uint8(255)
		for _, nti := range rgnIndices[:] {
			nt := b[nti]
			if nt == v {
				// this value has already been found
				continue OnePossibleRowLoop
			}
			if nt&v == 0 {
				// not a possible tile
				continue
			}
			_, y := indexToXY(nti)
			if tcRow == y {
				// row already a candidate
				continue
			}
			if tcRow != 255 {
				// multiple candidate rows
				continue OnePossibleRowLoop
			}
			tcRow = y
		}
		if tcRow == 255 {
			// no candidate rows. Wat?
			*b = b0
			return 0
		}

		// iterate over the candidate row, excluding the value from tiles in other regions
		for _, nti := range RowIndices(tcRow) {
			if indexToRegionIndex(nti) == rgnIdx {
				// skip our region
				continue
			}
			if b.Set(nti, b[nti]&^v) == 0 {
				// invalid board configuration, revert the change
				*b = b0
				return 0
			}
		}
	}

OnePossibleColumnLoop:
	for v := Tile(1); v < tAny; v = v << 1 {
		tcCol := uint8(255)
		for _, nti := range rgnIndices[:] {
			nt := b[nti]
			if nt == v {
				// this value has already been found
				continue OnePossibleColumnLoop
			}
			if nt&v == 0 {
				// not a possible tile
				continue
			}
			x, _ := indexToXY(nti)
			if tcCol == x {
				// column already a candidate
				continue
			}
			if tcCol != 255 {
				// multiple candidate columns
				continue OnePossibleColumnLoop
			}
			tcCol = x
		}
		if tcCol == 255 {
			// no candidate columns. Wat?
			*b = b0
			return 0
		}

		for _, nti := range ColumnIndices(tcCol) {
			if indexToRegionIndex(nti) == rgnIdx {
				// skip our region
				continue
			}
			if b.Set(nti, b[nti]&^v) == 0 {
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
		ti := xyToIndex(x, y)
		t := byteToTileMap[ba[i]]
		if t == 0 {
			return int64(nr), errors.New("invalid byte")
		}
		if b.Set(ti, t) == 0 {
			return int64(nr), fmt.Errorf("invalid board (offset=%d byte=%q)", i, []byte{ba[i]})
		}
		//b[ri][ti] = byteToTileMap[ba[i]]
	}

	return int64(nr), nil
}

func (b Board) Art() [9 * 9 * 2]byte {
	var ba [9 * 9 * 2]byte
	for y := uint8(0); y < 9; y++ {
		rowStart := y * 9 * 2
		for x, ti := range RowIndices(y) {
			t := b[ti]
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
