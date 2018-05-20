// Bitcoin-blockchain specific functions.
package blockchain

import (
	proto "github.com/golang/protobuf/proto"
	sha3 "golang.org/x/crypto/sha3"
	"time"
	//"crypto/sha256"
	//"encoding/binary"
	//"encoding/hex"
	//"encoding/json"
	//"fmt"
	//"net"
	//"github.com/dedis/onet/log"
)

/*
type Block struct {
	Magic      [4]byte
	BlockSize  uint32
	HeaderHash string
	*Header
	TransactionList
}

type TrBlock struct {
	Block
}
*/

func (tr *TxBlock) MarshalBinary() ([]byte, error) {
	return proto.Marshal(tr)
}

// Hash returns a hash representation of the block
func (tr *TxBlock) HashSum() []byte {
	TxStr := tr.String()
	TxSha := sha3.Sum256([]byte(TxStr))
	return TxSha[:]
}

//type Header struct {
//	MerkleRoot string
//	Parent     string
//	ParentKey  string
//	PublicKey  string
//	LeaderId   net.IP
//}

// HashSum returns a hash representation of the header
func (h *TxBlockHdr) HashSum() []byte {
	HStr := h.String()
	HSha := sha3.Sum256([]byte(HStr))
	return HSha[:]
}

func (txb *TxBlock) NewTxBlock(transactions *TxBlockData, header *TxBlockHdr) *TxBlock {
	return NewTxBlock(transactions, header)
}

func (t *TxBlock) NewHeader(transactions TxBlockData, parent string, parentKey string) *TxBlockHdr {
	return NewHeader(parent, parentKey)
}

func (txb *TxBlock) Calculate_root(transactions TxBlockData) (res string) {
	return ""
	//return HashRootTransactions(transactions)
}

// Porting to public method non related to Header / TrBlock whatsoever
func NewTxBlock(transactions *TxBlockData, header *TxBlockHdr) *TxBlock {
	txb := new(TxBlock)
	txb.Tbd = transactions
	txb.Tbh = header
	txb.Coinbase = nil
	return txb
}

func NewHeader(parent, parentKey string) *TxBlockHdr {
	hdr := new(TxBlockHdr)
	hdr.Version = 0
	hdr.Type = 0
	hdr.KblkHash = []byte(parentKey)
	hdr.PblkHash = []byte(parent)
	hdr.GasLimit = nil
	hdr.GasUsed = nil
	hdr.StateRoot = nil
	hdr.TxRoot = nil
	hdr.Timestamp = uint64(time.Now().Unix())
	//hdr.MerkleRoot = HashRootTransactions(transactions)
	return hdr
}

func NewTxBlockData(transactions []STransaction, cnt int) *TxBlockData {
	tbd := new(TxBlockData)
	tbd.StxCnt = uint32(cnt)
	stx := make([]*STransaction, cnt)
	for i, _ := range transactions {
		stx[i] = &transactions[i]
	}
	tbd.Stx = stx
	return tbd
}

/*
func HashRootTransactions(transactions TxBlockData) string {
	var hashes []HashID

	for _, t := range transactions.Stx {
		temp, _ := hex.DecodeString(t.Hash)
		hashes = append(hashes, temp)
	}
	out, _ := ProofTree(sha256.New, hashes)
	return hex.EncodeToString(out)
}
*/

func (txb *TxBlock) Hash(h *TxBlockHdr) (res []byte) {
	HStr := h.String()
	HSha := sha3.Sum256([]byte(HStr))
	return HSha[:]
	//change it to be more portable
	//return HashHeader(h)

}

//func HashHeader(h *Header) string {
//	data := fmt.Sprintf("%v", h)
//	sha := sha256.New()
//	if _, err := sha.Write([]byte(data)); err != nil {
//		log.Error("Couldn't hash header:", err)
//	}
//	hash := sha.Sum(nil)
//	return hex.EncodeToString(hash)
//}
