package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	mrand "math/rand"
	"time"

	"github.com/cypherium_private/mvp/blockchain"
	"github.com/golang/protobuf/proto"
	"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
)

func init() {
	mrand.Seed(time.Now().Unix())
}

func genAddr(cnt int) ([]blockchain.Address, []blockchain.PubKey, []blockchain.PrvKey) {
	addrs := make([]blockchain.Address, cnt)
	pubkeys := make([]blockchain.PubKey, cnt)
	prvkeys := make([]blockchain.PrvKey, cnt)

	for i := 0; i < cnt; i++ {
		pub, prv, err := ed25519.GenerateKey(rand.Reader)
		fmt.Println("pub, prv,", pub, prv)
		fmt.Println("pub, prv,", len(pub), len(prv))
		if err == nil {
			addrSha := sha3.Sum256(pub)
			addr160 := ripemd160.New()
			addr160.Write(addrSha[:])
			addr := addr160.Sum(nil)

			copy(addrs[i][:], addr)
			copy(pubkeys[i][:], pub)
			copy(prvkeys[i][:], prv)
		}
	}
	return addrs, pubkeys, prvkeys
}

func genTxs(cnt int) ([]blockchain.Transaction, []blockchain.PrvKey, []blockchain.Address) {
	addrs, pubkeys, prvkeys := genAddr(cnt * 2)
	txs := make([]blockchain.Transaction, cnt)
	for i := 0; i < cnt; i++ {
		txs[i].Version = 1
		txs[i].SenderKey = pubkeys[i][:]
		txs[i].Recipient = addrs[i+cnt][:]
		txs[i].Quantity = new(big.Int).SetUint64(mrand.Uint64() % 90000).Bytes()
		txs[i].Data = []byte("")
		txs[i].AvailGas = mrand.Uint64() % 90000
		txs[i].GasPrice = new(big.Int).SetUint64(mrand.Uint64() % 90000).Bytes()
	}
	fmt.Printf("%T\n", txs[0].Quantity)
	return txs, prvkeys, addrs
}

func GenStxs(cnt int) error {
	db, err := leveldb.OpenFile("./mock_stx_db", nil)
	defer db.Close()
	stxsBatch := new(leveldb.Batch)

	txs, prvkeys, addrs := genTxs(cnt)
	stxs := make([]blockchain.STransaction, cnt)
	for i := 0; i < cnt; i++ {
		stxs[i].Tx = &txs[i]
		txsStr := txs[i].String()
		stxs[i].SenderSig = ed25519.Sign(prvkeys[i][:], []byte(txsStr))
		stxsOut, err := proto.Marshal(&stxs[i])
		shaStxs := sha3.Sum256(stxsOut)
		if err == nil {
			stxsBatch.Put(shaStxs[:], stxsOut)
		}
	}
	err = db.Write(stxsBatch, nil)
	if err != nil {
		fmt.Printf("Error in writing stxs db\n")
	}

	GenAccount(addrs)
	return err
}

func GetStxs(cnt int) []blockchain.STransaction {
	stxs := make([]blockchain.STransaction, cnt)
	db, err := leveldb.OpenFile("./mock_stx_db", nil)
	defer db.Close()
	if err != nil {
		fmt.Printf("GetStxs: %s\n", err.Error())
	}
	iter := db.NewIterator(nil, nil)
	for i := 0; i < cnt; i++ {
		iter.Next()
		value := iter.Value()
		if err := proto.Unmarshal(value, &stxs[i]); err != nil {
			fmt.Printf("Error unmarshalling %d %s\n", i, err.Error())
		}
	}
	iter.Release()
	return stxs
}

func GenAccount(addrs []blockchain.Address) {
	code0, _ := hex.DecodeString("51000151000251000155000100010002")
	storage0 := []byte{10, 20, 0, 50, 100}

	db, err := leveldb.OpenFile("./mock_state_db", nil)
	defer db.Close()
	accBatch := new(leveldb.Batch)

	accounts := make([]blockchain.Account, len(addrs))
	for i, addr := range addrs {
		accounts[i].Version = 1
		accounts[i].Type = mrand.Uint32() % 2
		accounts[i].Address = addr[:]
		accounts[i].Latency = 0
		accounts[i].Balance = new(big.Int).SetUint64(mrand.Uint64() % 90000).Bytes()
		accounts[i].CodeHash = code0
		accounts[i].StgRoot = storage0
		accOut, err := proto.Marshal(&accounts[i])
		if err == nil {
			accBatch.Put(addr[:], accOut)
		}
	}

	err = db.Write(accBatch, nil)
	if err != nil {
		fmt.Printf("Error in writing state db\n")
	}
}
