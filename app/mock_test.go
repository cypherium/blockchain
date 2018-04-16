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
