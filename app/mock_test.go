package main

import (
	"fmt"
	//"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/crypto/ed25519"
	"testing"
)

var num int = 10

func TestStxGen(t *testing.T) {
	if err := GenStxs(num); err != nil {
		t.Error("Failed")
	}
}

func TestStxVerify(t *testing.T) {
	stxs := GetStxs(num)
	for i := 0; i < num; i++ {
		fmt.Printf("%+v\n", stxs[i])
	}
	for i := 0; i < num; i++ {
		sig := stxs[i].GetSenderSig()
		msg := []byte(stxs[i].GetTx().String())
		pk := stxs[i].GetTx().GetSenderKey()
		ok := ed25519.Verify(pk, msg, sig)
		if ok == false {
			t.Errorf("STx %d not valid\n", i)
		}
	}
}
