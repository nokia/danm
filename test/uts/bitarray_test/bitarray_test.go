package bitarray_test

import (
  "strconv"
  "testing"
  "github.com/nokia/danm/pkg/bitarray"
)

var arraySizeTestConsts = []struct {
  inputSize int
  isErrorExpected bool
  expectedSize int
}{
  {-1, true, 0},
  {0, true, 0},
  {1, false, 1},
  {33000, false, 33000},
}

func TestNewBitArray(t *testing.T) {
  for _, tt := range arraySizeTestConsts {
    t.Run("TestNewBitArray Size:"+strconv.Itoa(tt.inputSize), func(t *testing.T) {
      testArray,error := bitarray.NewBitArray(tt.inputSize)
      if (tt.isErrorExpected && nil==error) && (!tt.isErrorExpected && nil!=error) {
        t.Errorf("BitArray initialization returned unexpected error result at test value %d: error expected %t, returned error %s", tt.inputSize, tt.isErrorExpected, error )
      }
      if tt.isErrorExpected  {
        return
      }
      actualSize := testArray.Len()
      if actualSize != tt.expectedSize {
        t.Errorf("BitArray returned unexpected size at test value %d: expected size %d, actual size %d", tt.inputSize, tt.expectedSize, actualSize)
      }
    })
  }
}

func TestBitArrayFunctionality(t *testing.T) {
  positionsToSet := [...]int{0,4,7}
  origBa,_ := bitarray.NewBitArray(8)
  for _, position := range positionsToSet {
    origBa.Set(uint32(position))
  }
  encodedBa := origBa.Encode()
  newBa := bitarray.NewBitArrayFromBase64(encodedBa)
  var isPositionSet bool
  for i := 0; i < 8; i++ {
    isPositionSet = newBa.Get(uint32(i))
    var shouldPositionBeSet bool
    for _, position := range positionsToSet {
      if i == position {
        shouldPositionBeSet = true
      }
    }
    if isPositionSet != shouldPositionBeSet {
      t.Errorf("After one round of Encoding + Decoding Bitarray field:%d changed value", i)
    }
  }
  for _, position := range positionsToSet {
    origBa.Reset(uint32(position))
    if origBa.Get(uint32(position)) {
      t.Errorf("Resetting did not work for number:%d", position)
    }
  }
}
