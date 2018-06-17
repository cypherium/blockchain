// Package types contains data types related to Ethereum consensus.
package types

import (
	"io"
	"math/big"
	"sort"
	"sync/atomic"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// A BlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
//type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
//func EncodeNonce(i uint64) BlockNonce {
//	var n BlockNonce
//	binary.BigEndian.PutUint64(n[:], i)
//	return n
//}

// Uint64 returns the integer value of a block nonce.
//func (n BlockNonce) Uint64() uint64 {
//	return binary.BigEndian.Uint64(n[:])
//}

// MarshalText encodes n as a hex string with 0x prefix.
//func (n BlockNonce) MarshalText() ([]byte, error) {
//	return hexutil.Bytes(n[:]).MarshalText()
//}

// UnmarshalText implements encoding.TextUnmarshaler.
//func (n *BlockNonce) UnmarshalText(input []byte) error {
//	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
//}

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// field type overrides for gencodec
type txheaderMarshaling struct {
	//Difficulty *hexutil.Big
	Number   *hexutil.Big
	GasLimit hexutil.Uint64
	GasUsed  hexutil.Uint64
	Time     *hexutil.Big
	Extra    hexutil.Bytes
	Hash     common.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the TxBlock hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *TxBlockHdr) Hash() common.Hash {
	return protobufHash(h)
}

// HashNoNonce returns the hash which is used as input for the proof-of-work search.
func (h *TxBlockHdr) HashNoNonce() common.Hash {
	return protobufHash([]interface{}{
		h.ParentHash,
		//h.UncleHash,
		h.Coinbase,
		h.Root,
		h.TxHash,
		h.ReceiptHash,
		h.Bloom,
		h.Difficulty,
		h.Number,
		h.GasLimit,
		h.GasUsed,
		h.Time,
		h.Extra,
	})
}

//type TxHeader struct {
//	TxblockHeadr *TxBlockHdr
//	Bloom        Bloom `json:"logsBloom"        gencodec:"required"`
//}
//type TxHeader TxBlockHdr

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *TxHeader) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen())/8)
}

//func rlpHash(x interface{}) (h common.Hash) {
//	hw := sha3.NewKeccak256()
//	rlp.Encode(hw, x)
//	hw.Sum(h[:0])
//	return h
//}

// TxBody is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type TxBody struct {
	Transactions []*Transaction
	//Uncles       []*Header
}

// TxBlock represents an entire block in the Ethereum blockchain.
type TxBlock struct {
	txblock *Txblock
	// caches
	hash atomic.Value
	size atomic.Value
}

// DeprecatedTd is an old relic for extracting the TD of a block. It is in the
// code solely to facilitate upgrading the database from the old format to the
// new, after which it should be deleted. Do not use!
//func (b *Block) DeprecatedTd() *big.Int {
//	return b.td
//}

// [deprecated by eth/63]
// StorageBlock defines the RLP encoding of a Block stored in the
// state database. The StorageBlock encoding contains fields that
// would otherwise need to be recomputed.
//type StorageBlock TxBlock

// "external" block encoding. used for eth protocol, etc.
//type extblock struct {
//	Header *Header
//	Txs    []*Transaction
//	Uncles []*Header
//}

// [deprecated by eth/63]
// "storage" block encoding. used for database.
//type storageblock struct {
//	Header *Header
//	Txs    []*Transaction
//	Uncles []*Header
//	TD     *big.Int
//}

// NewTxBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewTxBlock(header *TxBlockHdr, txs []*Transaction, uncles []*Header, receipts []*Receipt) *TxBlock {
	b := &TxBlock{header: CopyTxBlockHdr(header), td: new(big.Int)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	if len(uncles) == 0 {
		b.header.UncleHash = EmptyUncleHash
	} else {
		b.header.UncleHash = CalcUncleHash(uncles)
		b.uncles = make([]*Header, len(uncles))
		for i := range uncles {
			b.uncles[i] = CopyHeader(uncles[i])
		}
	}

	return b
}

// NewTxBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewTxBlockWithHeader(header *TxBlockHdr) *TxBlock {
	return &TxBlock{header: CopyTxBlockHdr(header)}
}

// CopyTxBlockHdr creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyTxBlockHdr(h *TxBlockHdr) *TxBlockHdr {
	cpy := *h
	cpy.Timestamp = h.Timestamp
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	return &cpy
}

// DecodeProtoBuf decodes the Cypherium
func (b *TxBlock) DecodeProtoBuf(m stream.Message) error {
	var eb Txblock
	size, _ := stream.Read(&eb, m)
	b.txblock.Tbd, b.txblock.Tbh, b.txblock.KblkCosiAggr, b.txblock.KblkCosiSign = eb.Tbd, eb.Tbh, eb.KblkCosiAggr, eb.KblkCosiSign
	b.size.Store(common.StorageSize(size))
	return nil
}

// EncodeProtoBuf serializes b into the protobuf block format.
func (b *TxBlock) EncodeProtoBuf(w io.Writer) error {
	return stream.Write(w, b.txblock)
}

// [deprecated by eth/63]
//func (b *StorageBlock) DecodeRLP(s *rlp.Stream) error {
//	var sb storageblock
//	if err := s.Decode(&sb); err != nil {
//		return err
//	}
//	b.header, b.uncles, b.transactions, b.td = sb.Header, sb.Uncles, sb.Txs, sb.TD
//	return nil
//}

// TODO: copies

//func (b *TxBlock) Uncles() []*Header          { return b.uncles }
func (b *TxBlock) Transactions() Transactions { return b.txblock.Tbd }

func (b *TxBlock) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

func (b *TxBlock) Number() *big.Int { return new(big.Int).Set(b.txblock.Tbh.Number) }
func (b *TxBlock) GasLimit() uint64 { return b.txblock.Tbh.GasLimit }
func (b *TxBlock) GasUsed() uint64  { return b.txblock.Tbh.GasUsed }

//func (b *TxBlock) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }
func (b *TxBlock) Time() *big.Int { return new(big.Int).Set(b.txblock.Tbh.Timestamp) }

func (b *TxBlock) NumberU64() uint64 { return b.txblock.Tbh.Number.Uint64() }

//func (b *TxBlock) MixDigest() common.Hash   { return b.header.MixDigest }
//func (b *TxBlock) Nonce() uint64            { return binary.BigEndian.Uint64(b.header.Nonce[:]) }
func (b *TxBlock) Bloom() Bloom { return b.txblock.Tbh.Bloom }

func (b *TxBlock) Coinbase() common.Address { return b.txblock.Tbh.Coinbase }
func (b *TxBlock) Root() common.Hash        { return b.txblock.Tbh.TxRoot }
func (b *TxBlock) ParentHash() common.Hash  { return b.txblock.Tbh.ParentHash }
func (b *TxBlock) TxHash() common.Hash      { return b.header.TxHash }

//func (b *TxBlock) ReceiptHash() common.Hash { return b.header.ReceiptHash }

//func (b *TxBlock) UncleHash() common.Hash   { return b.header.UncleHash }
func (b *TxBlock) Extra() []byte { return common.CopyBytes(b.txblock.Tbh.Extra) }

func (b *TxBlock) Header() *Header { return CopyTxBlockHdr(b.txblock.Tbh) }

// Body returns the non-header content of the block.
func (b *TxBlock) Body() *Body { return &Body{b.txblock.Tbd.transactions} }

func (b *TxBlock) HashNoNonce() common.Hash {
	return b.txblock.Tbh.HashNoNonce()
}

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *TxBlock) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	stream.Write(&c, b)
	//rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

type writeTxBlockCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

//func CalcUncleHash(uncles []*Header) common.Hash {
//	return rlpHash(uncles)
//}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *TxBlock) WithSeal(header *TxBlockHdr) *TxBlock {
	cpy := *header

	return &TxBlock{
		header:       &cpy,
		transactions: b.txblock.Tbd.transactions,
		//uncles:       b.uncles,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *TxBlock) WithBody(transactions []*Transaction) *TxBlock {
	block := &TxBlock{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(transactions)),
		//uncles:       make([]*Header, len(uncles)),
	}
	copy(block.transactions, transactions)
	for i := range uncles {
		block.uncles[i] = CopyHeader(uncles[i])
	}
	return block
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *TxBlock) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v, _ := b.txblock.Tbh.Marshal()
	b.hash.Store(v)
	return v
}

type TxBlocks []*TxBlock

type TxBlockBy func(b1, b2 *TxBlock) bool

func (self TxBlockBy) Sort(blocks TxBlocks) {
	bs := txblockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}

type txblockSorter struct {
	blocks TxBlocks
	by     func(b1, b2 *TxBlock) bool
}

func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

func Number(b1, b2 *TxBlock) bool { return b1.txblock.Tbh.Number.Cmp(b2.txblock.Tbh.Number) < 0 }
