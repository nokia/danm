package bitarray

import (
  b64 "encoding/base64"
  "errors"
)

// BitArray is type to represent an arbitrary long array of bits
type BitArray struct {
  len int
  data []byte
}

// NewBitArray creates a new, empty BitArray object
// Returns error if length is zero, otherwise a pointer to the array
func NewBitArray(len int) (*BitArray, error) {
  if 0 >= len {
    return nil, errors.New("Can't make a BitArray with a negative length!")
  }
  bitArray := &BitArray{len, make([]byte, (len+7)/8)}
  bitArray.Set(0)
  return bitArray, nil
}

// NewBitArrayFromBase64 creates a new BitArray from a Base64 encoded string
func NewBitArrayFromBase64(text string) *BitArray {
  var tmp []byte
  tmp, _ = b64.StdEncoding.DecodeString(text)
  arr := new(BitArray)
  arr.len = len(tmp)*8
  arr.data = tmp
  return arr
}

// Set sets the bit at the input position of the BitArray
func (arr *BitArray) Set(pos uint32) {
  arr.data[pos/8] |= byte( 0x1 << (7-pos%8))
}

// Reset unsets the bit at the input position of the BitArray
func (arr *BitArray) Reset(pos uint32) {
  arr.data[pos/8] &= ^byte( 0x1 << (7-pos%8))
}

// Get returns whether the input position of the BitArray is set, or not
func (arr *BitArray) Get(pos uint32) bool {
  return (arr.data[pos/8] & (0x1 << (7-pos%8))) != 0
}

// Encode returns the Base64 encoded string of the BitArray
func (arr *BitArray) Encode() string {
  return b64.StdEncoding.EncodeToString(arr.data)
}

// Len returns the length of the BitArray
func (arr *BitArray) Len() int {
  return arr.len
}
