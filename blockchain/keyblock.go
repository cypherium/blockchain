package blockchain

import (
	"golang.org/x/crypto/sha3"
	//"log"
	"net"
)

/*
type KeyBlock struct {
	Block
}
*/

func (*KeyBlock) NewKeyBlock(transactions TxBlockData, header *KeyBlockHdr) (tr KeyBlock) {
	kb := new(KeyBlock)
	//kb.HeaderHash = kb.Hash(header)
	//kb.TransactionList = transactions
	//kb.BlockSize = 0
	kb.Kbh = header
	return *kb
}

func (trb *KeyBlock) Hash(h *KeyBlockHdr) (res []byte) {
	HStr := h.String()
	HSha := sha3.Sum256([]byte(HStr))
	return HSha[:]
}

func (t *KeyBlock) NewHeader(transactions TxBlockData, parent string, IP net.IP, key string) (hd KeyBlockHdr) {
	hdr := new(KeyBlockHdr)
	//hdr.LeaderId = IP
	hdr.Version = 0
	//hdr.PublicKey = key
	hdr.PKblkHash = []byte(parent)
	//hdr.MerkleRoot = t.Calculate_root(transactions)
	return *hdr
}

/*
func (trb *KeyBlock) Print() {
	log.Println("Header:")
	log.Printf("Leader %v", trb.LeaderId)
	//log.Printf("Pkey %v", trb.PublicKey)
	log.Printf("ParentKey %v", trb.ParentKey)
	log.Printf("Merkle %v", trb.MerkleRoot)
	//log.Println("Rest:")
	log.Printf("Hash %v", trb.HeaderHash)
	return
}
*/
