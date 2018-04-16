/*
 * Copyright (C) 2018 The Cypherium Blockchain authors
 *
 * This file is part of the Cypherium Blockchain library.
 *
 * The Cypherium Blockchain library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The Cypherium Blockchain library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with the Cypherium Blockchain library. If not, see <http://www.gnu.org/licenses/>.
 *
 */

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

func (tr *TxBlock) MarshalBinary() ([]byte, error) {
	return proto.Marshal(tr)
}

// Hash returns a hash representation of the block
func (tr *TxBlock) HashSum() []byte {
	TxStr := tr.String()
	TxSha := sha3.Sum256([]byte(TxStr))
	return TxSha[:]
}

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

func (txb *TxBlock) Hash(h *TxBlockHdr) (res []byte) {
	HStr := h.String()
	HSha := sha3.Sum256([]byte(HStr))
	return HSha[:]
	//change it to be more portable
	//return HashHeader(h)

}
