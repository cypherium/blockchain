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

// Code generated by protoc-gen-go. DO NOT EDIT.
// source: common.proto

/*
Package blockchain is a generated protocol buffer package.

It is generated from these files:
	common.proto

It has these top-level messages:
	Account
	Transaction
	STransaction
	Coinbase
	TxBlockHdr
	TxBlockData
	TxBlock
	PoWHdr
	PoW
	KeyBlockHdr
	KeyBlock
*/
package blockchain

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// Accounts
type Account struct {
	Version  uint32 `protobuf:"varint,1,opt,name=Version" json:"Version,omitempty"`
	Type     uint32 `protobuf:"varint,2,opt,name=Type" json:"Type,omitempty"`
	Address  []byte `protobuf:"bytes,3,opt,name=Address,proto3" json:"Address,omitempty"`
	Latency  uint32 `protobuf:"varint,4,opt,name=Latency" json:"Latency,omitempty"`
	Balance  []byte `protobuf:"bytes,5,opt,name=Balance,proto3" json:"Balance,omitempty"`
	CodeHash []byte `protobuf:"bytes,6,opt,name=CodeHash,proto3" json:"CodeHash,omitempty"`
	StgRoot  []byte `protobuf:"bytes,7,opt,name=StgRoot,proto3" json:"StgRoot,omitempty"`
}

func (m *Account) Reset()                    { *m = Account{} }
func (m *Account) String() string            { return proto.CompactTextString(m) }
func (*Account) ProtoMessage()               {}
func (*Account) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Account) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *Account) GetType() uint32 {
	if m != nil {
		return m.Type
	}
	return 0
}

func (m *Account) GetAddress() []byte {
	if m != nil {
		return m.Address
	}
	return nil
}

func (m *Account) GetLatency() uint32 {
	if m != nil {
		return m.Latency
	}
	return 0
}

func (m *Account) GetBalance() []byte {
	if m != nil {
		return m.Balance
	}
	return nil
}

func (m *Account) GetCodeHash() []byte {
	if m != nil {
		return m.CodeHash
	}
	return nil
}

func (m *Account) GetStgRoot() []byte {
	if m != nil {
		return m.StgRoot
	}
	return nil
}

type Transaction struct {
	Version   uint32 `protobuf:"varint,1,opt,name=Version" json:"Version,omitempty"`
	SenderKey []byte `protobuf:"bytes,3,opt,name=SenderKey,proto3" json:"SenderKey,omitempty"`
	Recipient []byte `protobuf:"bytes,4,opt,name=Recipient,proto3" json:"Recipient,omitempty"`
	Quantity  []byte `protobuf:"bytes,5,opt,name=Quantity,proto3" json:"Quantity,omitempty"`
	Data      []byte `protobuf:"bytes,6,opt,name=Data,proto3" json:"Data,omitempty"`
	AvailGas  uint64 `protobuf:"varint,7,opt,name=AvailGas" json:"AvailGas,omitempty"`
	GasPrice  []byte `protobuf:"bytes,8,opt,name=GasPrice,proto3" json:"GasPrice,omitempty"`
}

func (m *Transaction) Reset()                    { *m = Transaction{} }
func (m *Transaction) String() string            { return proto.CompactTextString(m) }
func (*Transaction) ProtoMessage()               {}
func (*Transaction) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *Transaction) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *Transaction) GetSenderKey() []byte {
	if m != nil {
		return m.SenderKey
	}
	return nil
}

func (m *Transaction) GetRecipient() []byte {
	if m != nil {
		return m.Recipient
	}
	return nil
}

func (m *Transaction) GetQuantity() []byte {
	if m != nil {
		return m.Quantity
	}
	return nil
}

func (m *Transaction) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *Transaction) GetAvailGas() uint64 {
	if m != nil {
		return m.AvailGas
	}
	return 0
}

func (m *Transaction) GetGasPrice() []byte {
	if m != nil {
		return m.GasPrice
	}
	return nil
}

type STransaction struct {
	Tx        *Transaction `protobuf:"bytes,1,opt,name=tx" json:"tx,omitempty"`
	SenderSig []byte       `protobuf:"bytes,2,opt,name=SenderSig,proto3" json:"SenderSig,omitempty"`
}

func (m *STransaction) Reset()                    { *m = STransaction{} }
func (m *STransaction) String() string            { return proto.CompactTextString(m) }
func (*STransaction) ProtoMessage()               {}
func (*STransaction) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *STransaction) GetTx() *Transaction {
	if m != nil {
		return m.Tx
	}
	return nil
}

func (m *STransaction) GetSenderSig() []byte {
	if m != nil {
		return m.SenderSig
	}
	return nil
}

type Coinbase struct {
	NumCommittee uint64 `protobuf:"varint,1,opt,name=NumCommittee" json:"NumCommittee,omitempty"`
	Committee    []byte `protobuf:"bytes,2,opt,name=Committee,proto3" json:"Committee,omitempty"`
	Quantity     []byte `protobuf:"bytes,3,opt,name=Quantity,proto3" json:"Quantity,omitempty"`
}

func (m *Coinbase) Reset()                    { *m = Coinbase{} }
func (m *Coinbase) String() string            { return proto.CompactTextString(m) }
func (*Coinbase) ProtoMessage()               {}
func (*Coinbase) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *Coinbase) GetNumCommittee() uint64 {
	if m != nil {
		return m.NumCommittee
	}
	return 0
}

func (m *Coinbase) GetCommittee() []byte {
	if m != nil {
		return m.Committee
	}
	return nil
}

func (m *Coinbase) GetQuantity() []byte {
	if m != nil {
		return m.Quantity
	}
	return nil
}

type TxBlockHdr struct {
	Version   uint32 `protobuf:"varint,1,opt,name=Version" json:"Version,omitempty"`
	Type      uint32 `protobuf:"varint,2,opt,name=Type" json:"Type,omitempty"`
	GasLimit  []byte `protobuf:"bytes,3,opt,name=GasLimit,proto3" json:"GasLimit,omitempty"`
	GasUsed   []byte `protobuf:"bytes,4,opt,name=GasUsed,proto3" json:"GasUsed,omitempty"`
	KblkHash  []byte `protobuf:"bytes,5,opt,name=KblkHash,proto3" json:"KblkHash,omitempty"`
	PblkHash  []byte `protobuf:"bytes,6,opt,name=PblkHash,proto3" json:"PblkHash,omitempty"`
	PTblkHash []byte `protobuf:"bytes,7,opt,name=PTblkHash,proto3" json:"PTblkHash,omitempty"`
	Timestamp uint64 `protobuf:"varint,8,opt,name=Timestamp" json:"Timestamp,omitempty"`
	StateRoot []byte `protobuf:"bytes,9,opt,name=StateRoot,proto3" json:"StateRoot,omitempty"`
	TxRoot    []byte `protobuf:"bytes,10,opt,name=TxRoot,proto3" json:"TxRoot,omitempty"`
}

func (m *TxBlockHdr) Reset()                    { *m = TxBlockHdr{} }
func (m *TxBlockHdr) String() string            { return proto.CompactTextString(m) }
func (*TxBlockHdr) ProtoMessage()               {}
func (*TxBlockHdr) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *TxBlockHdr) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *TxBlockHdr) GetType() uint32 {
	if m != nil {
		return m.Type
	}
	return 0
}

func (m *TxBlockHdr) GetGasLimit() []byte {
	if m != nil {
		return m.GasLimit
	}
	return nil
}

func (m *TxBlockHdr) GetGasUsed() []byte {
	if m != nil {
		return m.GasUsed
	}
	return nil
}

func (m *TxBlockHdr) GetKblkHash() []byte {
	if m != nil {
		return m.KblkHash
	}
	return nil
}

func (m *TxBlockHdr) GetPblkHash() []byte {
	if m != nil {
		return m.PblkHash
	}
	return nil
}

func (m *TxBlockHdr) GetPTblkHash() []byte {
	if m != nil {
		return m.PTblkHash
	}
	return nil
}

func (m *TxBlockHdr) GetTimestamp() uint64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

func (m *TxBlockHdr) GetStateRoot() []byte {
	if m != nil {
		return m.StateRoot
	}
	return nil
}

func (m *TxBlockHdr) GetTxRoot() []byte {
	if m != nil {
		return m.TxRoot
	}
	return nil
}

type TxBlockData struct {
	StxCnt uint32          `protobuf:"varint,9,opt,name=StxCnt" json:"StxCnt,omitempty"`
	Stx    []*STransaction `protobuf:"bytes,10,rep,name=Stx" json:"Stx,omitempty"`
}

func (m *TxBlockData) Reset()                    { *m = TxBlockData{} }
func (m *TxBlockData) String() string            { return proto.CompactTextString(m) }
func (*TxBlockData) ProtoMessage()               {}
func (*TxBlockData) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *TxBlockData) GetStxCnt() uint32 {
	if m != nil {
		return m.StxCnt
	}
	return 0
}

func (m *TxBlockData) GetStx() []*STransaction {
	if m != nil {
		return m.Stx
	}
	return nil
}

type TxBlock struct {
	Tbh          *TxBlockHdr  `protobuf:"bytes,1,opt,name=tbh" json:"tbh,omitempty"`
	Tbd          *TxBlockData `protobuf:"bytes,2,opt,name=tbd" json:"tbd,omitempty"`
	Coinbase     *Coinbase    `protobuf:"bytes,3,opt,name=Coinbase" json:"Coinbase,omitempty"`
	KblkCosiSign []byte       `protobuf:"bytes,4,opt,name=KblkCosiSign,proto3" json:"KblkCosiSign,omitempty"`
	KblkCosiAggr []byte       `protobuf:"bytes,5,opt,name=KblkCosiAggr,proto3" json:"KblkCosiAggr,omitempty"`
}

func (m *TxBlock) Reset()                    { *m = TxBlock{} }
func (m *TxBlock) String() string            { return proto.CompactTextString(m) }
func (*TxBlock) ProtoMessage()               {}
func (*TxBlock) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *TxBlock) GetTbh() *TxBlockHdr {
	if m != nil {
		return m.Tbh
	}
	return nil
}

func (m *TxBlock) GetTbd() *TxBlockData {
	if m != nil {
		return m.Tbd
	}
	return nil
}

func (m *TxBlock) GetCoinbase() *Coinbase {
	if m != nil {
		return m.Coinbase
	}
	return nil
}

func (m *TxBlock) GetKblkCosiSign() []byte {
	if m != nil {
		return m.KblkCosiSign
	}
	return nil
}

func (m *TxBlock) GetKblkCosiAggr() []byte {
	if m != nil {
		return m.KblkCosiAggr
	}
	return nil
}

type PoWHdr struct {
	PKblkHash  []byte `protobuf:"bytes,1,opt,name=PKblkHash,proto3" json:"PKblkHash,omitempty"`
	Difficulty uint64 `protobuf:"varint,2,opt,name=Difficulty" json:"Difficulty,omitempty"`
	Timestamp  uint64 `protobuf:"varint,3,opt,name=Timestamp" json:"Timestamp,omitempty"`
	Worker     []byte `protobuf:"bytes,4,opt,name=Worker,proto3" json:"Worker,omitempty"`
	Nonce      uint64 `protobuf:"varint,5,opt,name=Nonce" json:"Nonce,omitempty"`
	MixHash    []byte `protobuf:"bytes,6,opt,name=mixHash,proto3" json:"mixHash,omitempty"`
}

func (m *PoWHdr) Reset()                    { *m = PoWHdr{} }
func (m *PoWHdr) String() string            { return proto.CompactTextString(m) }
func (*PoWHdr) ProtoMessage()               {}
func (*PoWHdr) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *PoWHdr) GetPKblkHash() []byte {
	if m != nil {
		return m.PKblkHash
	}
	return nil
}

func (m *PoWHdr) GetDifficulty() uint64 {
	if m != nil {
		return m.Difficulty
	}
	return 0
}

func (m *PoWHdr) GetTimestamp() uint64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

func (m *PoWHdr) GetWorker() []byte {
	if m != nil {
		return m.Worker
	}
	return nil
}

func (m *PoWHdr) GetNonce() uint64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *PoWHdr) GetMixHash() []byte {
	if m != nil {
		return m.MixHash
	}
	return nil
}

type PoW struct {
	Ph  *PoWHdr `protobuf:"bytes,1,opt,name=ph" json:"ph,omitempty"`
	Sig []byte  `protobuf:"bytes,2,opt,name=sig,proto3" json:"sig,omitempty"`
}

func (m *PoW) Reset()                    { *m = PoW{} }
func (m *PoW) String() string            { return proto.CompactTextString(m) }
func (*PoW) ProtoMessage()               {}
func (*PoW) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{8} }

func (m *PoW) GetPh() *PoWHdr {
	if m != nil {
		return m.Ph
	}
	return nil
}

func (m *PoW) GetSig() []byte {
	if m != nil {
		return m.Sig
	}
	return nil
}

type KeyBlockHdr struct {
	Version   uint32 `protobuf:"varint,1,opt,name=Version" json:"Version,omitempty"`
	PKblkHash []byte `protobuf:"bytes,2,opt,name=PKblkHash,proto3" json:"PKblkHash,omitempty"`
	Timestamp uint64 `protobuf:"varint,3,opt,name=Timestamp" json:"Timestamp,omitempty"`
}

func (m *KeyBlockHdr) Reset()                    { *m = KeyBlockHdr{} }
func (m *KeyBlockHdr) String() string            { return proto.CompactTextString(m) }
func (*KeyBlockHdr) ProtoMessage()               {}
func (*KeyBlockHdr) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{9} }

func (m *KeyBlockHdr) GetVersion() uint32 {
	if m != nil {
		return m.Version
	}
	return 0
}

func (m *KeyBlockHdr) GetPKblkHash() []byte {
	if m != nil {
		return m.PKblkHash
	}
	return nil
}

func (m *KeyBlockHdr) GetTimestamp() uint64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

type KeyBlock struct {
	Kbh          *KeyBlockHdr `protobuf:"bytes,1,opt,name=kbh" json:"kbh,omitempty"`
	KblkCosiSign []byte       `protobuf:"bytes,2,opt,name=KblkCosiSign,proto3" json:"KblkCosiSign,omitempty"`
	KblkCosiAggr []byte       `protobuf:"bytes,3,opt,name=KblkCosiAggr,proto3" json:"KblkCosiAggr,omitempty"`
	NumPow       uint64       `protobuf:"varint,4,opt,name=NumPow" json:"NumPow,omitempty"`
	Pows         []*PoW       `protobuf:"bytes,5,rep,name=Pows" json:"Pows,omitempty"`
}

func (m *KeyBlock) Reset()                    { *m = KeyBlock{} }
func (m *KeyBlock) String() string            { return proto.CompactTextString(m) }
func (*KeyBlock) ProtoMessage()               {}
func (*KeyBlock) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{10} }

func (m *KeyBlock) GetKbh() *KeyBlockHdr {
	if m != nil {
		return m.Kbh
	}
	return nil
}

func (m *KeyBlock) GetKblkCosiSign() []byte {
	if m != nil {
		return m.KblkCosiSign
	}
	return nil
}

func (m *KeyBlock) GetKblkCosiAggr() []byte {
	if m != nil {
		return m.KblkCosiAggr
	}
	return nil
}

func (m *KeyBlock) GetNumPow() uint64 {
	if m != nil {
		return m.NumPow
	}
	return 0
}

func (m *KeyBlock) GetPows() []*PoW {
	if m != nil {
		return m.Pows
	}
	return nil
}

func init() {
	proto.RegisterType((*Account)(nil), "blockchain.Account")
	proto.RegisterType((*Transaction)(nil), "blockchain.Transaction")
	proto.RegisterType((*STransaction)(nil), "blockchain.STransaction")
	proto.RegisterType((*Coinbase)(nil), "blockchain.Coinbase")
	proto.RegisterType((*TxBlockHdr)(nil), "blockchain.TxBlockHdr")
	proto.RegisterType((*TxBlockData)(nil), "blockchain.TxBlockData")
	proto.RegisterType((*TxBlock)(nil), "blockchain.TxBlock")
	proto.RegisterType((*PoWHdr)(nil), "blockchain.PoWHdr")
	proto.RegisterType((*PoW)(nil), "blockchain.PoW")
	proto.RegisterType((*KeyBlockHdr)(nil), "blockchain.KeyBlockHdr")
	proto.RegisterType((*KeyBlock)(nil), "blockchain.KeyBlock")
}

func init() { proto.RegisterFile("common.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 730 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x55, 0x4b, 0x6e, 0xdb, 0x48,
	0x10, 0x05, 0x3f, 0x96, 0xec, 0x92, 0x8c, 0x19, 0x34, 0x0c, 0x0d, 0x31, 0x18, 0x0c, 0x0c, 0xce,
	0x62, 0x34, 0xb3, 0x30, 0x02, 0x67, 0x99, 0x95, 0x2c, 0x03, 0x36, 0x60, 0xc3, 0x90, 0x9b, 0x72,
	0xbc, 0x6e, 0x91, 0x6d, 0xa9, 0x21, 0x91, 0x2d, 0x90, 0xad, 0x58, 0xba, 0x4a, 0xce, 0x90, 0x13,
	0x24, 0x47, 0xc8, 0x05, 0x72, 0x9c, 0xa0, 0x9a, 0xdd, 0xfc, 0xc8, 0x48, 0xe0, 0xec, 0xf8, 0xea,
	0x15, 0xcb, 0x55, 0xef, 0x3d, 0x5a, 0xd0, 0x8f, 0x65, 0x9a, 0xca, 0xec, 0x6c, 0x9d, 0x4b, 0x25,
	0x09, 0xcc, 0x56, 0x32, 0x5e, 0xc6, 0x0b, 0x26, 0xb2, 0xf0, 0x8b, 0x03, 0xdd, 0x51, 0x1c, 0xcb,
	0x4d, 0xa6, 0x48, 0x00, 0xdd, 0xf7, 0x3c, 0x2f, 0x84, 0xcc, 0x02, 0xe7, 0xd4, 0x19, 0x1e, 0x53,
	0x0b, 0x09, 0x01, 0x7f, 0xba, 0x5b, 0xf3, 0xc0, 0xd5, 0x65, 0xfd, 0x8c, 0xdd, 0xa3, 0x24, 0xc9,
	0x79, 0x51, 0x04, 0xde, 0xa9, 0x33, 0xec, 0x53, 0x0b, 0x91, 0xb9, 0x65, 0x8a, 0x67, 0xf1, 0x2e,
	0xf0, 0xcb, 0x39, 0x06, 0x22, 0x73, 0xc1, 0x56, 0x2c, 0x8b, 0x79, 0x70, 0x50, 0xbe, 0x63, 0x20,
	0xf9, 0x13, 0x0e, 0xc7, 0x32, 0xe1, 0xd7, 0xac, 0x58, 0x04, 0x1d, 0x4d, 0x55, 0x18, 0xdf, 0x8a,
	0xd4, 0x9c, 0x4a, 0xa9, 0x82, 0x6e, 0xf9, 0x96, 0x81, 0xe1, 0x57, 0x07, 0x7a, 0xd3, 0x9c, 0x65,
	0x05, 0x8b, 0x15, 0xee, 0xf9, 0xe3, 0x0b, 0xfe, 0x82, 0xa3, 0x88, 0x67, 0x09, 0xcf, 0x6f, 0xf8,
	0xce, 0xec, 0x5b, 0x17, 0x90, 0xa5, 0x3c, 0x16, 0x6b, 0xc1, 0x33, 0xa5, 0x77, 0xee, 0xd3, 0xba,
	0x80, 0xbb, 0xdd, 0x6f, 0x58, 0xa6, 0x84, 0xda, 0x99, 0xb5, 0x2b, 0x8c, 0xca, 0x5c, 0x32, 0xc5,
	0xcc, 0xce, 0xfa, 0x19, 0xfb, 0x47, 0x1f, 0x98, 0x58, 0x5d, 0xb1, 0x42, 0x2f, 0xec, 0xd3, 0x0a,
	0x23, 0x77, 0xc5, 0x8a, 0x49, 0x2e, 0x62, 0x1e, 0x1c, 0x96, 0xb3, 0x2c, 0x0e, 0x1f, 0xa0, 0x1f,
	0x35, 0xaf, 0xf9, 0x17, 0x5c, 0xb5, 0xd5, 0x87, 0xf4, 0xce, 0xff, 0x38, 0xab, 0x4d, 0x3b, 0x6b,
	0x34, 0x51, 0x57, 0x6d, 0xeb, 0xe3, 0x22, 0x31, 0xd7, 0x1e, 0x55, 0xc7, 0x45, 0x62, 0x1e, 0x2e,
	0x50, 0x5a, 0x91, 0xcd, 0x58, 0xc1, 0x49, 0x08, 0xfd, 0xbb, 0x4d, 0x3a, 0x96, 0x69, 0x2a, 0x94,
	0xe2, 0x5c, 0x0f, 0xf7, 0x69, 0xab, 0x86, 0xd3, 0xea, 0x06, 0x33, 0xad, 0x66, 0x9b, 0x62, 0x78,
	0x6d, 0x31, 0xc2, 0x8f, 0x2e, 0xc0, 0x74, 0x7b, 0x81, 0x8b, 0x5e, 0x27, 0xf9, 0x2f, 0xe6, 0xa9,
	0x54, 0xe6, 0x56, 0xa4, 0x42, 0xd9, 0xc1, 0x16, 0xe3, 0xa4, 0x2b, 0x56, 0x3c, 0x14, 0x3c, 0x31,
	0xee, 0x58, 0x88, 0x6f, 0xdd, 0xcc, 0x56, 0x4b, 0x9d, 0x1b, 0xe3, 0x8d, 0xc5, 0xc8, 0x4d, 0x2c,
	0x67, 0x32, 0x65, 0x31, 0x1e, 0x39, 0x99, 0x5a, 0xb2, 0x4c, 0x55, 0x5d, 0x40, 0x76, 0x2a, 0x52,
	0x5e, 0x28, 0x96, 0xae, 0xb5, 0x4d, 0x3e, 0xad, 0x0b, 0x5a, 0x6e, 0xc5, 0x14, 0xd7, 0x89, 0x3c,
	0x32, 0x72, 0xdb, 0x02, 0x19, 0x40, 0x67, 0xba, 0xd5, 0x14, 0x68, 0xca, 0xa0, 0xf0, 0x1e, 0x7a,
	0x46, 0x1b, 0x1d, 0x92, 0x01, 0x74, 0x22, 0xb5, 0x1d, 0x67, 0xe5, 0x84, 0x63, 0x6a, 0x10, 0xf9,
	0x1f, 0xbc, 0x48, 0x6d, 0x03, 0x38, 0xf5, 0x86, 0xbd, 0xf3, 0xa0, 0xe9, 0x7a, 0x33, 0x1b, 0x14,
	0x9b, 0xc2, 0x6f, 0x0e, 0x74, 0xcd, 0x4c, 0x32, 0x04, 0x4f, 0xcd, 0x16, 0x26, 0x2d, 0x83, 0x56,
	0x5a, 0x2a, 0x47, 0x28, 0xb6, 0x90, 0xff, 0xb0, 0x33, 0xd1, 0xda, 0xef, 0xe7, 0xaa, 0xde, 0x0f,
	0x5b, 0x13, 0xf2, 0xa6, 0x8e, 0x8e, 0xf6, 0xa4, 0x77, 0x7e, 0xd2, 0xec, 0xb7, 0x1c, 0x6d, 0x05,
	0x0c, 0xf5, 0x1f, 0xcb, 0x42, 0x44, 0x62, 0x9e, 0x19, 0xbb, 0x5a, 0xb5, 0x66, 0xcf, 0x68, 0x3e,
	0xcf, 0x8d, 0x6f, 0xad, 0x5a, 0xf8, 0xc9, 0x81, 0xce, 0x44, 0x3e, 0x62, 0x8c, 0xd0, 0xaa, 0xca,
	0x63, 0xc7, 0x58, 0x55, 0x99, 0xfc, 0x37, 0xc0, 0xa5, 0x78, 0x7a, 0x12, 0xf1, 0x66, 0xa5, 0x76,
	0xfa, 0x28, 0x9f, 0x36, 0x2a, 0x6d, 0x2b, 0xbd, 0x7d, 0x2b, 0x07, 0xd0, 0x79, 0x94, 0xf9, 0x92,
	0xe7, 0x66, 0x51, 0x83, 0xc8, 0x09, 0x1c, 0xdc, 0x49, 0xfb, 0x6f, 0xca, 0xa7, 0x25, 0xc0, 0x18,
	0xa6, 0x62, 0xdb, 0xc8, 0x93, 0x85, 0xe1, 0x3b, 0xf0, 0x26, 0xf2, 0x91, 0x84, 0xe0, 0xae, 0xad,
	0x07, 0xa4, 0xa9, 0x54, 0x79, 0x0a, 0x75, 0xd7, 0x0b, 0xf2, 0x3b, 0x78, 0x45, 0xf5, 0x99, 0xe2,
	0x63, 0x18, 0x43, 0xef, 0x86, 0xef, 0x5e, 0xf1, 0xd9, 0xb4, 0x94, 0x70, 0xf7, 0x95, 0xf8, 0xe9,
	0xa5, 0xe1, 0x67, 0x07, 0x0e, 0xed, 0x5f, 0xc1, 0x08, 0x2c, 0xab, 0xb0, 0xb4, 0x22, 0xd0, 0x58,
	0x84, 0x62, 0xcf, 0x0b, 0x43, 0xdd, 0x57, 0x18, 0xea, 0xbd, 0x34, 0x14, 0x95, 0xbe, 0xdb, 0xa4,
	0x13, 0xf9, 0xac, 0x95, 0xf6, 0xa9, 0x41, 0xe4, 0x1f, 0xf0, 0x27, 0xf2, 0xb9, 0x08, 0x0e, 0x74,
	0xe0, 0x7f, 0xdb, 0x13, 0x8d, 0x6a, 0x72, 0xd6, 0xd1, 0x3f, 0x5c, 0x6f, 0xbf, 0x07, 0x00, 0x00,
	0xff, 0xff, 0xa8, 0x81, 0x14, 0x0c, 0xc8, 0x06, 0x00, 0x00,
}
