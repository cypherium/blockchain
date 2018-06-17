// Package types contains data types related to Ethereum consensus.
package types

import (
	"encoding/binary"
	"io"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cypherium_private/go-cypherium/common"
	"github.com/cypherium_private/go-cypherium/common/hexutil"
	"github.com/cypherium_private/go-cypherium/crypto/sha3"
	"github.com/cypherium_private/go-cypherium/protobuf"
)

// A KeyBlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
type KeyBlockNonce [8]byte

// KeyBlockEncodeNonce converts the given integer to a block nonce.
func KeyBlockEncodeNonce(i uint64) KeyBlockNonce {
	var n KeyBlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n KeyBlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// MarshalText encodes n as a hex string with 0x prefix.
func (n KeyBlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *KeyBlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("KeyBlockNonce", input, n[:])
}

//go:generate gencodec -type Header -field-override keyheaderMarshaling -out gen_header_json.go

// field type overrides for gencodec
type keyheaderMarshaling struct {
	Version *hexutil.Big
	Number  *hexutil.Big
	Hash    common.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *KeyBlockHdr) Hash() common.Hash {
	return protobufHash(h)
}

// HashNoNonce returns the hash which is used as input for the proof-of-work search.
func (h *KeyBlockHdr) HashNoNonce() common.Hash {

	return protobufHash([]interface{}{
		h.Version,
		h.PKblkHash,
		h.Timestamp,
		h.Number,
		h.ViewchangeCounter,
		h.Leader,
	})
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *KeyBlockHdr) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen())/8)
}

func protobufHash(x interface{}) (h common.Hash) {
	hw := sha3.New256()
	stream.Write(hw, x) //rlp to protobuf
	hw.Sum(h[:0])
	return h
}

// KeyBody is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (Pows and ) together.
type KeyBody struct {
	Pows []*PoW
}

// Block represents an entire block in the Ethereum blockchain.
type KeyBlock struct {
	keyblock *Keyblock
	// caches
	hash atomic.Value
	size atomic.Value
	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// // "external" block encoding. used for cypherium protocol, etc.
// type extkeyblock struct {
// 	Header *KeyBlockHdr
// 	Pows   []*PoW
// }

// NewKeyBlock creates a new keyblock. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewKeyBlock(header *KeyBlockHdr, Pows []*PoW) *KeyBlock {
	//上层填补
	// PKblkHash         []byte  `protobuf:"bytes,2,opt,name=PKblkHash,proto3" json:"PKblkHash,omitempty"`
	// ViewchangeCounter uint64  `protobuf:"varint,5,opt,name=ViewchangeCounter,proto3" json:"ViewchangeCounter,omitempty"`
	// Leader            []byte  `protobuf:"bytes,6,opt,name=Leader,proto3" json:"Leader,omitempty"`
	b := &KeyBlock{}
	b.keyblock = &Keyblock{KeyBlockHdr: CopyKeyBlockHeader(header), Version: []byte("0.0.1")}

	b.Pows = Pows[:]
	b.NumPow = len(b.Pows)
	//上层填补
	// KblkCosiSign []byte       `protobuf:"bytes,2,opt,name=KblkCosiSign,proto3" json:"KblkCosiSign,omitempty"`
	// KblkCosiAggr []byte       `protobuf:"bytes,3,opt,name=KblkCosiAggr,proto3" json:"KblkCosiAggr,omitempty"`

	return b
}

// NewKeyBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewKeyBlockWithHeader(header *KeyBlockHdr) *KeyBlock {
	b := &Keyblock{header: CopyKeyBlockHeader(header), Version: []byte("0.0.1")}
	return &KeyBlock{keyblock: b}
}

// CopyKeyBlockHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyKeyBlockHeader(h *KeyBlockHdr) *KeyBlockHdr {
	cpy := *h
	if cpy.Timestamp = new(BigInt); h.Timestamp != nil {
		cpy.Timestamp.Neg = h.Timestamp.Neg
		cpy.Timestamp.Abs = h.Timestamp.Abs[:]
	}

	if cpy.Number = new(BigInt); h.Number != nil {
		cpy.Number = h.Number
	}

	return &cpy
}

// DecodeProtoBuf decodes the Cypherium
func (b *KeyBlock) DecodeProtoBuf(m stream.Message) error {
	var eb Keyblock
	size, _ := stream.Read(&eb, m)
	b.keyblock.Kbh, b.keyblock.KblkCosiAggr, b.keyblock.KblkCosiSign, b.keyblock.NumPow, b.keyblock.Pows = eb.Kbh, eb.KblkCosiAggr, eb.KblkCosiSign, eb.KblkCosiSign, eb.NumPow, eb.Pows
	b.size.Store(common.StorageSize(size))
	return nil
}

// EncodeProtoBuf serializes b into the protobuf block format.
func (b *KeyBlock) EncodeProtoBuf(w io.Writer) error {
	return stream.Write(w, b.keyblock)
}

// Body returns the non-header content of the block.
func (b *KeyBlock) Body() *KeyBody { return &KeyBody{b.keyblock.Pows} }

func (b *KeyBlock) HashNoNonce() common.Hash {
	return b.keyblock.Kbh.HashNoNonce()
}

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *KeyBlock) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeKeyBlockCounter(0)
	stream.Write(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

type writeKeyBlockCounter common.StorageSize

func (c *writeKeyBlockCounter) Write(b []byte) (int, error) {
	*c += writeKeyBlockCounter(len(b))
	return len(b), nil
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *KeyBlock) WithSeal(header *KeyBlockHdr) *KeyBlock {
	cpy := *header

	keyblock := &Keyblock{
		Kbh:          &cpy,
		KblkCosiAggr: b.keyblock.KblkCosiAggr,
		KblkCosiSign: b.keyblock.KblkCosiSign,
		NumPow:       b.keyblock.NumPow,
		Pows:         b.keyblock.Pows,
	}
	return &KeyBlock{keyblock: keyblock}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *KeyBlock) WithBody(Pows []*PoW) *KeyBlock {
	block := &Keyblock{
		Kbh:          CopyHeader(b.keyblock.Kbh),
		KblkCosiAggr: b.keyblock.KblkCosiAggr,
		KblkCosiSign: b.keyblock.KblkCosiSign,
		NumPow:       b.keyblock.NumPow,
		Pows:         Pows[:],
	}

	keyblock := &KeyBlock{keyblock: block}
	return keyblock
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *KeyBlock) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.keyblock.Kbh.Hash()
	b.hash.Store(v)
	return v
}

type KeyBlocks []*KeyBlock

type KeyBlockBy func(b1, b2 *KeyBlock) bool

func (self KeyBlockBy) Sort(blocks KeyBlocks) {
	bs := keyblockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}

type keyblockSorter struct {
	blocks KeyBlocks
	by     func(b1, b2 *KeyBlock) bool
}

func (self keyblockSorter) Len() int { return len(self.blocks) }
func (self keyblockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self keyblockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

func KeyBlockNumber(b1, b2 *KeyBlock) bool {
	return b1.keyblock.Kbh.Number.Cmp(b2.keyblock.Kbh.Number) < 0
}
