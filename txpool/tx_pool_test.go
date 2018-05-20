// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package txpool

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/cypherium_private/mvp/common"
	"github.com/cypherium_private/mvp/crypto"
	"github.com/cypherium_private/mvp/ethdb"
	"github.com/cypherium_private/mvp/event"
	"github.com/cypherium_private/mvp/params"
	"github.com/cypherium_private/mvp/txpool/state"
	"github.com/cypherium_private/mvp/txpool/types"
	"golang.org/x/crypto/ed25519"
)

// testTxPoolConfig is a transaction pool configuration without stateful disk
// sideeffects used during testing.
var testTxPoolConfig TxPoolConfig

func init() {
	testTxPoolConfig = DefaultTxPoolConfig
	testTxPoolConfig.Journal = ""
}

type testBlockChain struct {
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed
}

func (bc *testBlockChain) CurrentBlock() *types.Block {
	return types.NewBlock(&types.Header{
		GasLimit: bc.gasLimit,
	}, nil, nil, nil)
}

func (bc *testBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return bc.CurrentBlock()
}

func (bc *testBlockChain) StateAt(common.Hash) (*state.StateDB, error) {
	return bc.statedb, nil
}

func (bc *testBlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

func transaction(nonce uint64, gaslimit uint64, key *ecdsa.PrivateKey) *types.Transaction {
	return pricedTransaction(nonce, gaslimit, big.NewInt(1), key)
}

func pricedTransaction(nonce uint64, gaslimit uint64, gasprice *big.Int, key *ecdsa.PrivateKey) *types.Transaction {
	tx, _ := types.SignTx(types.NewTransaction(nonce, common.Address{}, big.NewInt(100), gaslimit, gasprice, nil), types.HomesteadSigner{}, key)
	return tx
}

func transactionCypherium(nonce uint64, gaslimit uint64, pub ed25519.PublicKey, key ed25519.PrivateKey) *types.Transaction {
	return pricedTransactionCypherium(nonce, gaslimit, big.NewInt(1), pub, key)
}

func pricedTransactionCypherium(nonce uint64, gaslimit uint64, gasprice *big.Int, pub ed25519.PublicKey, key ed25519.PrivateKey) *types.Transaction {
	tx, _ := types.SignTxCypherium(types.NewTransactionCypherium(nonce, common.Address{}, big.NewInt(100), gaslimit, gasprice, nil, pub), types.ED25519Signer{}, key)
	return tx
}

func setupTxPool() (*TxPool, *ecdsa.PrivateKey) {
	diskdb, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(diskdb))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	key, _ := crypto.GenerateKey()
	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)

	return pool, key
}

func setupTxPoolCypherium() (*TxPool, ed25519.PublicKey, ed25519.PrivateKey) {
	diskdb, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(diskdb))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pub, key, _ := crypto.GenerateKeyCypherium()
	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)

	return pool, pub, key
}

// validateTxPoolInternals checks various consistency invariants within the pool.
func validateTxPoolInternals(pool *TxPool) error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	// Ensure the total transaction set is consistent with pending + queued
	pending, queued := pool.stats()
	if total := len(pool.all); total != pending+queued {
		return fmt.Errorf("total transaction count %d != %d pending + %d queued", total, pending, queued)
	}
	if priced := pool.priced.items.Len() - pool.priced.stales; priced != pending+queued {
		return fmt.Errorf("total priced transaction count %d != %d pending + %d queued", priced, pending, queued)
	}
	// Ensure the next nonce to assign is the correct one
	for addr, txs := range pool.pending {
		// Find the last transaction
		var last uint64
		for nonce := range txs.txs.items {
			if last < nonce {
				last = nonce
			}
		}
		if nonce := pool.pendingState.GetNonce(addr); nonce != last+1 {
			return fmt.Errorf("pending nonce mismatch: have %v, want %v", nonce, last+1)
		}
	}
	return nil
}

// validateEvents checks that the correct number of transaction addition events
// were fired on the pool's event feed.
func validateEvents(events chan TxPreEvent, count int) error {
	for i := 0; i < count; i++ {
		select {
		case <-events:
		case <-time.After(time.Second):
			return fmt.Errorf("event #%d not fired", i)
		}
	}
	select {
	case tx := <-events:
		return fmt.Errorf("more than %d events fired: %v", count, tx.Tx)

	case <-time.After(50 * time.Millisecond):
		// This branch should be "default", but it's a data race between goroutines,
		// reading the event channel and pushng into it, so better wait a bit ensuring
		// really nothing gets injected.
	}
	return nil
}

func deriveSender(tx *types.Transaction) (common.Address, error) {
	return types.Sender(types.HomesteadSigner{}, tx)
}

func deriveSenderCypherium(tx *types.Transaction) (common.Address, error) {
	return types.Sender(types.ED25519Signer{}, tx)
}

type testChain struct {
	*testBlockChain
	address common.Address
	trigger *bool
}

// testChain.State() is used multiple times to reset the pending state.
// when simulate is true it will create a state that indicates
// that tx0 and tx1 are included in the chain.
func (c *testChain) State() (*state.StateDB, error) {
	// delay "state change" by one. The tx pool fetches the
	// state multiple times and by delaying it a bit we simulate
	// a state change between those fetches.
	stdb := c.statedb
	if *c.trigger {
		db, _ := ethdb.NewMemDatabase()
		c.statedb, _ = state.New(common.Hash{}, state.NewDatabase(db))
		// simulate that the new head block included tx0 and tx1
		c.statedb.SetNonce(c.address, 2)
		c.statedb.SetBalance(c.address, new(big.Int).SetUint64(params.Ether))
		*c.trigger = false
	}
	return stdb, nil
}

// This test simulates a scenario where a new block is imported during a
// state reset and tests whether the pending state is in sync with the
// block head event that initiated the resetState().
func TestStateChangeDuringTransactionPoolReset(t *testing.T) {
	t.Parallel()

	var (
		db, _       = ethdb.NewMemDatabase()
		pub, key, _ = crypto.GenerateKeyCypherium()
		address     = crypto.PubkeyToAddressCypherium(pub)
		//key, _     = crypto.GenerateKey()
		//address    = crypto.PubkeyToAddress(key.PublicKey)
		statedb, _ = state.New(common.Hash{}, state.NewDatabase(db))
		trigger    = false
	)

	// setup pool with 2 transaction in it
	statedb.SetBalance(address, new(big.Int).SetUint64(params.Ether))
	blockchain := &testChain{&testBlockChain{statedb, 1000000000, new(event.Feed)}, address, &trigger}

	tx0 := transactionCypherium(0, 100000, pub, key)
	tx1 := transactionCypherium(1, 100000, pub, key)

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)
	defer pool.Stop()

	nonce := pool.State().GetNonce(address)
	if nonce != 0 {
		t.Fatalf("Invalid nonce, want 0, got %d", nonce)
	}

	pool.AddRemotes(types.Transactions{tx0, tx1})

	nonce = pool.State().GetNonce(address)
	if nonce != 2 {
		t.Fatalf("Invalid nonce, want 2, got %d", nonce)
	}

	// trigger state change in the background
	trigger = true

	pool.lockedReset(nil, nil)

	_, err := pool.Pending()
	if err != nil {
		t.Fatalf("Could not fetch pending transactions: %v", err)
	}
	nonce = pool.State().GetNonce(address)
	if nonce != 2 {
		t.Fatalf("Invalid nonce, want 2, got %d", nonce)
	}
}

func TestInvalidTransactions(t *testing.T) {
	t.Parallel()

	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	tx := transactionCypherium(0, 100, pub, key)
	from, _ := deriveSenderCypherium(tx)

	pool.currentState.AddBalance(from, big.NewInt(1))
	if err := pool.AddRemote(tx); err != ErrInsufficientFunds {
		t.Error("expected", ErrInsufficientFunds)
	}

	balance := new(big.Int).Add(tx.Value(), new(big.Int).Mul(new(big.Int).SetUint64(tx.Gas()), tx.GasPrice()))
	pool.currentState.AddBalance(from, balance)
	if err := pool.AddRemote(tx); err != ErrIntrinsicGas {
		t.Error("expected", ErrIntrinsicGas, "got", err)
	}

	pool.currentState.SetNonce(from, 1)
	pool.currentState.AddBalance(from, big.NewInt(0xffffffffffffff))
	tx = transactionCypherium(0, 100000, pub, key)
	if err := pool.AddRemote(tx); err != ErrNonceTooLow {
		t.Error("expected", ErrNonceTooLow)
	}

	tx = transactionCypherium(1, 100000, pub, key)
	pool.gasPrice = big.NewInt(1000)
	if err := pool.AddRemote(tx); err != ErrUnderpriced {
		t.Error("expected", ErrUnderpriced, "got", err)
	}
	if err := pool.AddLocal(tx); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

func TestTransactionQueue(t *testing.T) {
	t.Parallel()

	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	tx := transactionCypherium(0, 100, pub, key)
	from, _ := deriveSenderCypherium(tx)
	fmt.Printf("%x\n", from)
	pool.currentState.AddBalance(from, big.NewInt(1000))
	pool.lockedReset(nil, nil)
	pool.enqueueTx(tx.Hash(), tx)

	pool.promoteExecutables([]common.Address{from})
	if len(pool.pending) != 1 {
		t.Error("expected valid txs to be 1 is", len(pool.pending))
	}

	tx = transactionCypherium(1, 100, pub, key)
	from, _ = deriveSenderCypherium(tx)
	pool.currentState.SetNonce(from, 2)
	pool.enqueueTx(tx.Hash(), tx)
	pool.promoteExecutables([]common.Address{from})
	if _, ok := pool.pending[from].txs.items[tx.Nonce()]; ok {
		t.Error("expected transaction to be in tx pool")
	}

	if len(pool.queue) > 0 {
		t.Error("expected transaction queue to be empty. is", len(pool.queue))
	}

	pool, pub, key = setupTxPoolCypherium()
	defer pool.Stop()

	tx1 := transactionCypherium(0, 100, pub, key)
	tx2 := transactionCypherium(10, 100, pub, key)
	tx3 := transactionCypherium(11, 100, pub, key)
	from, _ = deriveSenderCypherium(tx1)
	pool.currentState.AddBalance(from, big.NewInt(1000))
	pool.lockedReset(nil, nil)

	pool.enqueueTx(tx1.Hash(), tx1)
	pool.enqueueTx(tx2.Hash(), tx2)
	pool.enqueueTx(tx3.Hash(), tx3)

	pool.promoteExecutables([]common.Address{from})

	if len(pool.pending) != 1 {
		t.Error("expected tx pool to be 1, got", len(pool.pending))
	}
	if pool.queue[from].Len() != 2 {
		t.Error("expected len(queue) == 2, got", pool.queue[from].Len())
	}
}

func TestTransactionNegativeValue(t *testing.T) {
	t.Parallel()

	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	tx, _ := types.SignTxCypherium(types.NewTransactionCypherium(0, common.Address{}, big.NewInt(-1), 100, big.NewInt(1), nil, pub), types.ED25519Signer{}, key)
	from, _ := deriveSenderCypherium(tx)
	pool.currentState.AddBalance(from, big.NewInt(1))
	if err := pool.AddRemote(tx); err != ErrNegativeValue {
		t.Error("expected", ErrNegativeValue, "got", err)
	}
}

func TestTransactionChainFork(t *testing.T) {
	t.Parallel()

	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	addr := crypto.PubkeyToAddressCypherium(pub)
	resetState := func() {
		db, _ := ethdb.NewMemDatabase()
		statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
		statedb.AddBalance(addr, big.NewInt(100000000000000))

		pool.chain = &testBlockChain{statedb, 1000000, new(event.Feed)}
		pool.lockedReset(nil, nil)
	}
	resetState()

	tx := transactionCypherium(0, 100000, pub, key)
	if _, err := pool.add(tx, false); err != nil {
		t.Error("didn't expect error", err)
	}
	pool.removeTx(tx.Hash(), true)

	// reset the pool's internal state
	resetState()
	if _, err := pool.add(tx, false); err != nil {
		t.Error("didn't expect error", err)
	}
}

func TestTransactionDoubleNonce(t *testing.T) {
	t.Parallel()

	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	addr := crypto.PubkeyToAddressCypherium(pub)
	resetState := func() {
		db, _ := ethdb.NewMemDatabase()
		statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
		statedb.AddBalance(addr, big.NewInt(100000000000000))

		pool.chain = &testBlockChain{statedb, 1000000, new(event.Feed)}
		pool.lockedReset(nil, nil)
	}
	resetState()

	signer := types.ED25519Signer{}
	tx1, _ := types.SignTxCypherium(types.NewTransactionCypherium(0, common.Address{}, big.NewInt(100), 100000, big.NewInt(1), nil, pub), signer, key)
	tx2, _ := types.SignTxCypherium(types.NewTransactionCypherium(0, common.Address{}, big.NewInt(100), 1000000, big.NewInt(2), nil, pub), signer, key)
	tx3, _ := types.SignTxCypherium(types.NewTransactionCypherium(0, common.Address{}, big.NewInt(100), 1000000, big.NewInt(1), nil, pub), signer, key)

	// Add the first two transaction, ensure higher priced stays only
	if replace, err := pool.add(tx1, false); err != nil || replace {
		t.Errorf("first transaction insert failed (%v) or reported replacement (%v)", err, replace)
	}
	if replace, err := pool.add(tx2, false); err != nil || !replace {
		t.Errorf("second transaction insert failed (%v) or not reported replacement (%v)", err, replace)
	}
	pool.promoteExecutables([]common.Address{addr})
	if pool.pending[addr].Len() != 1 {
		t.Error("expected 1 pending transactions, got", pool.pending[addr].Len())
	}
	if tx := pool.pending[addr].txs.items[0]; tx.Hash() != tx2.Hash() {
		t.Errorf("transaction mismatch: have %x, want %x", tx.Hash(), tx2.Hash())
	}
	// Add the third transaction and ensure it's not saved (smaller price)
	pool.add(tx3, false)
	pool.promoteExecutables([]common.Address{addr})
	if pool.pending[addr].Len() != 1 {
		t.Error("expected 1 pending transactions, got", pool.pending[addr].Len())
	}
	if tx := pool.pending[addr].txs.items[0]; tx.Hash() != tx2.Hash() {
		t.Errorf("transaction mismatch: have %x, want %x", tx.Hash(), tx2.Hash())
	}
	// Ensure the total transaction count is correct
	if len(pool.all) != 1 {
		t.Error("expected 1 total transactions, got", len(pool.all))
	}
}

func TestTransactionMissingNonce(t *testing.T) {
	t.Parallel()

	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	addr := crypto.PubkeyToAddressCypherium(pub)
	pool.currentState.AddBalance(addr, big.NewInt(100000000000000))
	tx := transactionCypherium(1, 100000, pub, key)
	if _, err := pool.add(tx, false); err != nil {
		t.Error("didn't expect error", err)
	}
	if len(pool.pending) != 0 {
		t.Error("expected 0 pending transactions, got", len(pool.pending))
	}
	if pool.queue[addr].Len() != 1 {
		t.Error("expected 1 queued transaction, got", pool.queue[addr].Len())
	}
	if len(pool.all) != 1 {
		t.Error("expected 1 total transactions, got", len(pool.all))
	}
}

func TestTransactionNonceRecovery(t *testing.T) {
	t.Parallel()

	const n = 10
	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	addr := crypto.PubkeyToAddressCypherium(pub)
	pool.currentState.SetNonce(addr, n)
	pool.currentState.AddBalance(addr, big.NewInt(100000000000000))
	pool.lockedReset(nil, nil)

	tx := transactionCypherium(n, 100000, pub, key)
	if err := pool.AddRemote(tx); err != nil {
		t.Error(err)
	}
	// simulate some weird re-order of transactions and missing nonce(s)
	pool.currentState.SetNonce(addr, n-1)
	pool.lockedReset(nil, nil)
	if fn := pool.pendingState.GetNonce(addr); fn != n-1 {
		t.Errorf("expected nonce to be %d, got %d", n-1, fn)
	}
}

// Tests that if an account runs out of funds, any pending and queued transactions
// are dropped.
func TestTransactionDropping(t *testing.T) {
	t.Parallel()

	// Create a test account and fund it
	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	account, _ := deriveSenderCypherium(transactionCypherium(0, 0, pub, key))
	pool.currentState.AddBalance(account, big.NewInt(1000))

	// Add some pending and some queued transactions
	var (
		tx0  = transactionCypherium(0, 100, pub, key)
		tx1  = transactionCypherium(1, 200, pub, key)
		tx2  = transactionCypherium(2, 300, pub, key)
		tx10 = transactionCypherium(10, 100, pub, key)
		tx11 = transactionCypherium(11, 200, pub, key)
		tx12 = transactionCypherium(12, 300, pub, key)
	)
	pool.promoteTx(account, tx0.Hash(), tx0)
	pool.promoteTx(account, tx1.Hash(), tx1)
	pool.promoteTx(account, tx2.Hash(), tx2)
	pool.enqueueTx(tx10.Hash(), tx10)
	pool.enqueueTx(tx11.Hash(), tx11)
	pool.enqueueTx(tx12.Hash(), tx12)

	// Check that pre and post validations leave the pool as is
	if pool.pending[account].Len() != 3 {
		t.Errorf("pending transaction mismatch: have %d, want %d", pool.pending[account].Len(), 3)
	}
	if pool.queue[account].Len() != 3 {
		t.Errorf("queued transaction mismatch: have %d, want %d", pool.queue[account].Len(), 3)
	}
	if len(pool.all) != 6 {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), 6)
	}
	pool.lockedReset(nil, nil)
	if pool.pending[account].Len() != 3 {
		t.Errorf("pending transaction mismatch: have %d, want %d", pool.pending[account].Len(), 3)
	}
	if pool.queue[account].Len() != 3 {
		t.Errorf("queued transaction mismatch: have %d, want %d", pool.queue[account].Len(), 3)
	}
	if len(pool.all) != 6 {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), 6)
	}
	// Reduce the balance of the account, and check that invalidated transactions are dropped
	pool.currentState.AddBalance(account, big.NewInt(-650))
	pool.lockedReset(nil, nil)

	if _, ok := pool.pending[account].txs.items[tx0.Nonce()]; !ok {
		t.Errorf("funded pending transaction missing: %v", tx0)
	}
	if _, ok := pool.pending[account].txs.items[tx1.Nonce()]; !ok {
		t.Errorf("funded pending transaction missing: %v", tx0)
	}
	if _, ok := pool.pending[account].txs.items[tx2.Nonce()]; ok {
		t.Errorf("out-of-fund pending transaction present: %v", tx1)
	}
	if _, ok := pool.queue[account].txs.items[tx10.Nonce()]; !ok {
		t.Errorf("funded queued transaction missing: %v", tx10)
	}
	if _, ok := pool.queue[account].txs.items[tx11.Nonce()]; !ok {
		t.Errorf("funded queued transaction missing: %v", tx10)
	}
	if _, ok := pool.queue[account].txs.items[tx12.Nonce()]; ok {
		t.Errorf("out-of-fund queued transaction present: %v", tx11)
	}
	if len(pool.all) != 4 {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), 4)
	}
	// Reduce the block gas limit, check that invalidated transactions are dropped
	pool.chain.(*testBlockChain).gasLimit = 100
	pool.lockedReset(nil, nil)

	if _, ok := pool.pending[account].txs.items[tx0.Nonce()]; !ok {
		t.Errorf("funded pending transaction missing: %v", tx0)
	}
	if _, ok := pool.pending[account].txs.items[tx1.Nonce()]; ok {
		t.Errorf("over-gased pending transaction present: %v", tx1)
	}
	if _, ok := pool.queue[account].txs.items[tx10.Nonce()]; !ok {
		t.Errorf("funded queued transaction missing: %v", tx10)
	}
	if _, ok := pool.queue[account].txs.items[tx11.Nonce()]; ok {
		t.Errorf("over-gased queued transaction present: %v", tx11)
	}
	if len(pool.all) != 2 {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), 2)
	}
}

// Tests that if a transaction is dropped from the current pending pool (e.g. out
// of fund), all consecutive (still valid, but not executable) transactions are
// postponed back into the future queue to prevent broadcasting them.
func TestTransactionPostponing(t *testing.T) {
	t.Parallel()

	// Create the pool to test the postponing with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create two test accounts to produce different gap profiles with
	keys := make([]ed25519.PrivateKey, 2)
	pubs := make([]ed25519.PublicKey, 2)
	accs := make([]common.Address, len(keys))

	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		accs[i] = crypto.PubkeyToAddressCypherium(pubs[i])

		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(50100))
	}
	// Add a batch consecutive pending transactions for validation
	txs := []*types.Transaction{}
	for i, key := range keys {

		for j := 0; j < 100; j++ {
			var tx *types.Transaction
			if (i+j)%2 == 0 {
				tx = transactionCypherium(uint64(j), 25000, pubs[i], key)
			} else {
				tx = transactionCypherium(uint64(j), 50000, pubs[i], key)
			}
			txs = append(txs, tx)
		}
	}
	for i, err := range pool.AddRemotes(txs) {
		if err != nil {
			t.Fatalf("tx %d: failed to add transactions: %v", i, err)
		}
	}
	// Check that pre and post validations leave the pool as is
	if pending := pool.pending[accs[0]].Len() + pool.pending[accs[1]].Len(); pending != len(txs) {
		t.Errorf("pending transaction mismatch: have %d, want %d", pending, len(txs))
	}
	if len(pool.queue) != 0 {
		t.Errorf("queued accounts mismatch: have %d, want %d", len(pool.queue), 0)
	}
	if len(pool.all) != len(txs) {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), len(txs))
	}
	pool.lockedReset(nil, nil)
	if pending := pool.pending[accs[0]].Len() + pool.pending[accs[1]].Len(); pending != len(txs) {
		t.Errorf("pending transaction mismatch: have %d, want %d", pending, len(txs))
	}
	if len(pool.queue) != 0 {
		t.Errorf("queued accounts mismatch: have %d, want %d", len(pool.queue), 0)
	}
	if len(pool.all) != len(txs) {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), len(txs))
	}
	// Reduce the balance of the account, and check that transactions are reorganised
	for _, addr := range accs {
		pool.currentState.AddBalance(addr, big.NewInt(-1))
	}
	pool.lockedReset(nil, nil)

	// The first account's first transaction remains valid, check that subsequent
	// ones are either filtered out, or queued up for later.
	if _, ok := pool.pending[accs[0]].txs.items[txs[0].Nonce()]; !ok {
		t.Errorf("tx %d: valid and funded transaction missing from pending pool: %v", 0, txs[0])
	}
	if _, ok := pool.queue[accs[0]].txs.items[txs[0].Nonce()]; ok {
		t.Errorf("tx %d: valid and funded transaction present in future queue: %v", 0, txs[0])
	}
	for i, tx := range txs[1:100] {
		if i%2 == 1 {
			if _, ok := pool.pending[accs[0]].txs.items[tx.Nonce()]; ok {
				t.Errorf("tx %d: valid but future transaction present in pending pool: %v", i+1, tx)
			}
			if _, ok := pool.queue[accs[0]].txs.items[tx.Nonce()]; !ok {
				t.Errorf("tx %d: valid but future transaction missing from future queue: %v", i+1, tx)
			}
		} else {
			if _, ok := pool.pending[accs[0]].txs.items[tx.Nonce()]; ok {
				t.Errorf("tx %d: out-of-fund transaction present in pending pool: %v", i+1, tx)
			}
			if _, ok := pool.queue[accs[0]].txs.items[tx.Nonce()]; ok {
				t.Errorf("tx %d: out-of-fund transaction present in future queue: %v", i+1, tx)
			}
		}
	}
	// The second account's first transaction got invalid, check that all transactions
	// are either filtered out, or queued up for later.
	if pool.pending[accs[1]] != nil {
		t.Errorf("invalidated account still has pending transactions")
	}
	for i, tx := range txs[100:] {
		if i%2 == 1 {
			if _, ok := pool.queue[accs[1]].txs.items[tx.Nonce()]; !ok {
				t.Errorf("tx %d: valid but future transaction missing from future queue: %v", 100+i, tx)
			}
		} else {
			if _, ok := pool.queue[accs[1]].txs.items[tx.Nonce()]; ok {
				t.Errorf("tx %d: out-of-fund transaction present in future queue: %v", 100+i, tx)
			}
		}
	}
	if len(pool.all) != len(txs)/2 {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), len(txs)/2)
	}
}

// Tests that if the transaction pool has both executable and non-executable
// transactions from an origin account, filling the nonce gap moves all queued
// ones into the pending pool.
func TestTransactionGapFilling(t *testing.T) {
	t.Parallel()

	// Create a test account and fund it
	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	account, _ := deriveSenderCypherium(transactionCypherium(0, 0, pub, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	// Keep track of transaction events to ensure all executables get announced
	events := make(chan TxPreEvent, testTxPoolConfig.AccountQueue+5)
	sub := pool.txFeed.Subscribe(events)
	defer sub.Unsubscribe()

	// Create a pending and a queued transaction with a nonce-gap in between
	if err := pool.AddRemote(transactionCypherium(0, 100000, pub, key)); err != nil {
		t.Fatalf("failed to add pending transaction: %v", err)
	}
	if err := pool.AddRemote(transactionCypherium(2, 100000, pub, key)); err != nil {
		t.Fatalf("failed to add queued transaction: %v", err)
	}
	pending, queued := pool.Stats()
	if pending != 1 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 1)
	}
	if queued != 1 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 1)
	}
	if err := validateEvents(events, 1); err != nil {
		t.Fatalf("original event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Fill the nonce gap and ensure all transactions become pending
	if err := pool.AddRemote(transactionCypherium(1, 100000, pub, key)); err != nil {
		t.Fatalf("failed to add gapped transaction: %v", err)
	}
	pending, queued = pool.Stats()
	if pending != 3 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 3)
	}
	if queued != 0 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
	}
	if err := validateEvents(events, 2); err != nil {
		t.Fatalf("gap-filling event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that if the transaction count belonging to a single account goes above
// some threshold, the higher transactions are dropped to prevent DOS attacks.
func TestTransactionQueueAccountLimiting(t *testing.T) {
	t.Parallel()

	// Create a test account and fund it
	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	account, _ := deriveSenderCypherium(transactionCypherium(0, 0, pub, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	// Keep queuing up transactions and make sure all above a limit are dropped
	for i := uint64(1); i <= testTxPoolConfig.AccountQueue+5; i++ {
		if err := pool.AddRemote(transactionCypherium(i, 100000, pub, key)); err != nil {
			t.Fatalf("tx %d: failed to add transaction: %v", i, err)
		}
		if len(pool.pending) != 0 {
			t.Errorf("tx %d: pending pool size mismatch: have %d, want %d", i, len(pool.pending), 0)
		}
		if i <= testTxPoolConfig.AccountQueue {
			if pool.queue[account].Len() != int(i) {
				t.Errorf("tx %d: queue size mismatch: have %d, want %d", i, pool.queue[account].Len(), i)
			}
		} else {
			if pool.queue[account].Len() != int(testTxPoolConfig.AccountQueue) {
				t.Errorf("tx %d: queue limit mismatch: have %d, want %d", i, pool.queue[account].Len(), testTxPoolConfig.AccountQueue)
			}
		}
	}
	if len(pool.all) != int(testTxPoolConfig.AccountQueue) {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), testTxPoolConfig.AccountQueue)
	}
}

// Tests that if the transaction count belonging to multiple accounts go above
// some threshold, the higher transactions are dropped to prevent DOS attacks.
//
// This logic should not hold for local transactions, unless the local tracking
// mechanism is disabled.
func TestTransactionQueueGlobalLimiting(t *testing.T) {
	testTransactionQueueGlobalLimiting(t, false)
}
func TestTransactionQueueGlobalLimitingNoLocals(t *testing.T) {
	testTransactionQueueGlobalLimiting(t, true)
}

func testTransactionQueueGlobalLimiting(t *testing.T, nolocals bool) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.NoLocals = nolocals
	config.GlobalQueue = config.AccountQueue*3 - 1 // reduce the queue limits to shorten test time (-1 to make it non divisible)

	pool := NewTxPool(config, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create a number of test accounts and fund them (last one will be the local)
	keys := make([]ed25519.PrivateKey, 5)
	pubs := make([]ed25519.PublicKey, 5)

	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000000))
	}
	local := keys[len(keys)-1]
	localp := pubs[len(pubs)-1]

	// Generate and queue a batch of transactions
	nonces := make(map[common.Address]uint64)

	txs := make(types.Transactions, 0, 3*config.GlobalQueue)
	for len(txs) < cap(txs) {
		num := rand.Intn(len(keys) - 1)
		key := keys[num] // skip adding transactions with the local account
		pub := pubs[num]
		addr := crypto.PubkeyToAddressCypherium(pub)

		txs = append(txs, transactionCypherium(nonces[addr]+1, 100000, pub, key))
		nonces[addr]++
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)

	queued := 0
	for addr, list := range pool.queue {
		if list.Len() > int(config.AccountQueue) {
			t.Errorf("addr %x: queued accounts overflown allowance: %d > %d", addr, list.Len(), config.AccountQueue)
		}
		queued += list.Len()
	}
	if queued > int(config.GlobalQueue) {
		t.Fatalf("total transactions overflow allowance: %d > %d", queued, config.GlobalQueue)
	}
	// Generate a batch of transactions from the local account and import them
	txs = txs[:0]
	for i := uint64(0); i < 3*config.GlobalQueue; i++ {
		txs = append(txs, transactionCypherium(i+1, 100000, localp, local))
	}
	pool.AddLocals(txs)

	// If locals are disabled, the previous eviction algorithm should apply here too
	if nolocals {
		queued := 0
		for addr, list := range pool.queue {
			if list.Len() > int(config.AccountQueue) {
				t.Errorf("addr %x: queued accounts overflown allowance: %d > %d", addr, list.Len(), config.AccountQueue)
			}
			queued += list.Len()
		}
		if queued > int(config.GlobalQueue) {
			t.Fatalf("total transactions overflow allowance: %d > %d", queued, config.GlobalQueue)
		}
	} else {
		// Local exemptions are enabled, make sure the local account owned the queue
		if len(pool.queue) != 1 {
			t.Errorf("multiple accounts in queue: have %v, want %v", len(pool.queue), 1)
		}
		// Also ensure no local transactions are ever dropped, even if above global limits
		if queued := pool.queue[crypto.PubkeyToAddressCypherium(localp)].Len(); uint64(queued) != 3*config.GlobalQueue {
			t.Fatalf("local account queued transaction count mismatch: have %v, want %v", queued, 3*config.GlobalQueue)
		}
	}
}

// Tests that if an account remains idle for a prolonged amount of time, any
// non-executable transactions queued up are dropped to prevent wasting resources
// on shuffling them around.
//
// This logic should not hold for local transactions, unless the local tracking
// mechanism is disabled.
func TestTransactionQueueTimeLimiting(t *testing.T)         { testTransactionQueueTimeLimiting(t, false) }
func TestTransactionQueueTimeLimitingNoLocals(t *testing.T) { testTransactionQueueTimeLimiting(t, true) }

func testTransactionQueueTimeLimiting(t *testing.T, nolocals bool) {
	// Reduce the eviction interval to a testable amount
	defer func(old time.Duration) { evictionInterval = old }(evictionInterval)
	evictionInterval = time.Second

	// Create the pool to test the non-expiration enforcement
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.Lifetime = time.Second
	config.NoLocals = nolocals

	pool := NewTxPool(config, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create two test accounts to ensure remotes expire but locals do not
	localp, local, _ := crypto.GenerateKeyCypherium()
	remotep, remote, _ := crypto.GenerateKeyCypherium()

	pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(localp), big.NewInt(1000000000))
	pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(remotep), big.NewInt(1000000000))

	// Add the two transactions and ensure they both are queued up
	if err := pool.AddLocal(pricedTransactionCypherium(1, 100000, big.NewInt(1), localp, local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(1, 100000, big.NewInt(1), remotep, remote)); err != nil {
		t.Fatalf("failed to add remote transaction: %v", err)
	}
	pending, queued := pool.Stats()
	if pending != 0 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
	}
	if queued != 2 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 2)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Wait a bit for eviction to run and clean up any leftovers, and ensure only the local remains
	time.Sleep(2 * config.Lifetime)

	pending, queued = pool.Stats()
	if pending != 0 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
	}
	if nolocals {
		if queued != 0 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
		}
	} else {
		if queued != 1 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 1)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that even if the transaction count belonging to a single account goes
// above some threshold, as long as the transactions are executable, they are
// accepted.
func TestTransactionPendingLimiting(t *testing.T) {
	t.Parallel()

	// Create a test account and fund it
	pool, pub, key := setupTxPoolCypherium()
	defer pool.Stop()

	account, _ := deriveSenderCypherium(transactionCypherium(0, 0, pub, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	// Keep track of transaction events to ensure all executables get announced
	events := make(chan TxPreEvent, testTxPoolConfig.AccountQueue+5)
	sub := pool.txFeed.Subscribe(events)
	defer sub.Unsubscribe()

	// Keep queuing up transactions and make sure all above a limit are dropped
	for i := uint64(0); i < testTxPoolConfig.AccountQueue+5; i++ {
		if err := pool.AddRemote(transactionCypherium(i, 100000, pub, key)); err != nil {
			t.Fatalf("tx %d: failed to add transaction: %v", i, err)
		}
		if pool.pending[account].Len() != int(i)+1 {
			t.Errorf("tx %d: pending pool size mismatch: have %d, want %d", i, pool.pending[account].Len(), i+1)
		}
		if len(pool.queue) != 0 {
			t.Errorf("tx %d: queue size mismatch: have %d, want %d", i, pool.queue[account].Len(), 0)
		}
	}
	if len(pool.all) != int(testTxPoolConfig.AccountQueue+5) {
		t.Errorf("total transaction mismatch: have %d, want %d", len(pool.all), testTxPoolConfig.AccountQueue+5)
	}
	if err := validateEvents(events, int(testTxPoolConfig.AccountQueue+5)); err != nil {
		t.Fatalf("event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that the transaction limits are enforced the same way irrelevant whether
// the transactions are added one by one or in batches.
func TestTransactionQueueLimitingEquivalency(t *testing.T)   { testTransactionLimitingEquivalency(t, 1) }
func TestTransactionPendingLimitingEquivalency(t *testing.T) { testTransactionLimitingEquivalency(t, 0) }

func testTransactionLimitingEquivalency(t *testing.T, origin uint64) {
	t.Parallel()

	// Add a batch of transactions to a pool one by one
	pool1, pub1, key1 := setupTxPoolCypherium()
	defer pool1.Stop()

	account1, _ := deriveSenderCypherium(transactionCypherium(0, 0, pub1, key1))
	pool1.currentState.AddBalance(account1, big.NewInt(1000000))

	for i := uint64(0); i < testTxPoolConfig.AccountQueue+5; i++ {
		if err := pool1.AddRemote(transactionCypherium(origin+i, 100000, pub1, key1)); err != nil {
			t.Fatalf("tx %d: failed to add transaction: %v", i, err)
		}
	}
	// Add a batch of transactions to a pool in one big batch
	pool2, pub2, key2 := setupTxPoolCypherium()
	defer pool2.Stop()

	account2, _ := deriveSenderCypherium(transactionCypherium(0, 0, pub2, key2))
	pool2.currentState.AddBalance(account2, big.NewInt(1000000))

	txs := []*types.Transaction{}
	for i := uint64(0); i < testTxPoolConfig.AccountQueue+5; i++ {
		txs = append(txs, transactionCypherium(origin+i, 100000, pub2, key2))
	}
	pool2.AddRemotes(txs)

	// Ensure the batch optimization honors the same pool mechanics
	if len(pool1.pending) != len(pool2.pending) {
		t.Errorf("pending transaction count mismatch: one-by-one algo: %d, batch algo: %d", len(pool1.pending), len(pool2.pending))
	}
	if len(pool1.queue) != len(pool2.queue) {
		t.Errorf("queued transaction count mismatch: one-by-one algo: %d, batch algo: %d", len(pool1.queue), len(pool2.queue))
	}
	if len(pool1.all) != len(pool2.all) {
		t.Errorf("total transaction count mismatch: one-by-one algo %d, batch algo %d", len(pool1.all), len(pool2.all))
	}
	if err := validateTxPoolInternals(pool1); err != nil {
		t.Errorf("pool 1 internal state corrupted: %v", err)
	}
	if err := validateTxPoolInternals(pool2); err != nil {
		t.Errorf("pool 2 internal state corrupted: %v", err)
	}
}

// Tests that if the transaction count belonging to multiple accounts go above
// some hard threshold, the higher transactions are dropped to prevent DOS
// attacks.
func TestTransactionPendingGlobalLimiting(t *testing.T) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.GlobalSlots = config.AccountSlots * 10

	pool := NewTxPool(config, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	keys := make([]ed25519.PrivateKey, 5)
	pubs := make([]ed25519.PublicKey, 5)

	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000000))
	}
	// Generate and queue a batch of transactions
	nonces := make(map[common.Address]uint64)

	txs := types.Transactions{}
	for i, key := range keys {
		addr := crypto.PubkeyToAddressCypherium(pubs[i])
		for j := 0; j < int(config.GlobalSlots)/len(keys)*2; j++ {
			txs = append(txs, transactionCypherium(nonces[addr], 100000, pubs[i], key))
			nonces[addr]++
		}
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)

	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	if pending > int(config.GlobalSlots) {
		t.Fatalf("total pending transactions overflow allowance: %d > %d", pending, config.GlobalSlots)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that if transactions start being capped, transactions are also removed from 'all'
func TestTransactionCapClearsFromAll(t *testing.T) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.AccountSlots = 2
	config.AccountQueue = 2
	config.GlobalSlots = 8

	pool := NewTxPool(config, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	pub, key, _ := crypto.GenerateKeyCypherium()
	addr := crypto.PubkeyToAddressCypherium(pub)
	pool.currentState.AddBalance(addr, big.NewInt(1000000))

	txs := types.Transactions{}
	for j := 0; j < int(config.GlobalSlots)*2; j++ {
		txs = append(txs, transactionCypherium(uint64(j), 100000, pub, key))
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that if the transaction count belonging to multiple accounts go above
// some hard threshold, if they are under the minimum guaranteed slot count then
// the transactions are still kept.
func TestTransactionPendingMinimumAllowance(t *testing.T) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.GlobalSlots = 0

	pool := NewTxPool(config, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	keys := make([]ed25519.PrivateKey, 5)
	pubs := make([]ed25519.PublicKey, 5)

	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000000))
	}
	// Generate and queue a batch of transactions
	nonces := make(map[common.Address]uint64)

	txs := types.Transactions{}
	for i, key := range keys {
		addr := crypto.PubkeyToAddressCypherium(pubs[i])
		for j := 0; j < int(config.AccountSlots)*2; j++ {
			txs = append(txs, transactionCypherium(nonces[addr], 100000, pubs[i], key))
			nonces[addr]++
		}
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)

	for addr, list := range pool.pending {
		if list.Len() != int(config.AccountSlots) {
			t.Errorf("addr %x: total pending transactions mismatch: have %d, want %d", addr, list.Len(), config.AccountSlots)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that setting the transaction pool gas price to a higher value correctly
// discards everything cheaper than that and moves any gapped transactions back
// from the pending pool to the queue.
//
// Note, local transactions are never allowed to be dropped.
func TestTransactionPoolRepricing(t *testing.T) {
	t.Parallel()

	// Create the pool to test the pricing enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Keep track of transaction events to ensure all executables get announced
	events := make(chan TxPreEvent, 32)
	sub := pool.txFeed.Subscribe(events)
	defer sub.Unsubscribe()

	// Create a number of test accounts and fund them
	keys := make([]ed25519.PrivateKey, 4)
	pubs := make([]ed25519.PublicKey, 4)

	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000000))
	}
	// Generate and queue a batch of transactions, both pending and queued
	txs := types.Transactions{}

	txs = append(txs, pricedTransactionCypherium(0, 100000, big.NewInt(2), pubs[0], keys[0]))
	txs = append(txs, pricedTransactionCypherium(1, 100000, big.NewInt(1), pubs[0], keys[0]))
	txs = append(txs, pricedTransactionCypherium(2, 100000, big.NewInt(2), pubs[0], keys[0]))

	txs = append(txs, pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[1], keys[1]))
	txs = append(txs, pricedTransactionCypherium(1, 100000, big.NewInt(2), pubs[1], keys[1]))
	txs = append(txs, pricedTransactionCypherium(2, 100000, big.NewInt(2), pubs[1], keys[1]))

	txs = append(txs, pricedTransactionCypherium(1, 100000, big.NewInt(2), pubs[2], keys[2]))
	txs = append(txs, pricedTransactionCypherium(2, 100000, big.NewInt(1), pubs[2], keys[2]))
	txs = append(txs, pricedTransactionCypherium(3, 100000, big.NewInt(2), pubs[2], keys[2]))

	ltx := pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[3], keys[3])

	// Import the batch and that both pending and queued transactions match up
	pool.AddRemotes(txs)
	pool.AddLocal(ltx)

	pending, queued := pool.Stats()
	if pending != 7 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 7)
	}
	if queued != 3 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 3)
	}
	if err := validateEvents(events, 7); err != nil {
		t.Fatalf("original event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Reprice the pool and check that underpriced transactions get dropped
	pool.SetGasPrice(big.NewInt(2))

	pending, queued = pool.Stats()
	if pending != 2 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 2)
	}
	if queued != 5 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 5)
	}
	if err := validateEvents(events, 0); err != nil {
		t.Fatalf("reprice event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Check that we can't add the old transactions back
	if err := pool.AddRemote(pricedTransactionCypherium(1, 100000, big.NewInt(1), pubs[0], keys[0])); err != ErrUnderpriced {
		t.Fatalf("adding underpriced pending transaction error mismatch: have %v, want %v", err, ErrUnderpriced)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[1], keys[1])); err != ErrUnderpriced {
		t.Fatalf("adding underpriced pending transaction error mismatch: have %v, want %v", err, ErrUnderpriced)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100000, big.NewInt(1), pubs[2], keys[2])); err != ErrUnderpriced {
		t.Fatalf("adding underpriced queued transaction error mismatch: have %v, want %v", err, ErrUnderpriced)
	}
	if err := validateEvents(events, 0); err != nil {
		t.Fatalf("post-reprice event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// However we can add local underpriced transactions
	tx := pricedTransactionCypherium(1, 100000, big.NewInt(1), pubs[3], keys[3])
	if err := pool.AddLocal(tx); err != nil {
		t.Fatalf("failed to add underpriced local transaction: %v", err)
	}
	if pending, _ = pool.Stats(); pending != 3 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 3)
	}
	if err := validateEvents(events, 1); err != nil {
		t.Fatalf("post-reprice local event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// And we can fill gaps with properly priced transactions
	if err := pool.AddRemote(pricedTransactionCypherium(1, 100000, big.NewInt(2), pubs[0], keys[0])); err != nil {
		t.Fatalf("failed to add pending transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(2), pubs[1], keys[1])); err != nil {
		t.Fatalf("failed to add pending transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100000, big.NewInt(2), pubs[2], keys[2])); err != nil {
		t.Fatalf("failed to add queued transaction: %v", err)
	}
	if err := validateEvents(events, 5); err != nil {
		t.Fatalf("post-reprice event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that setting the transaction pool gas price to a higher value does not
// remove local transactions.
func TestTransactionPoolRepricingKeepsLocals(t *testing.T) {
	t.Parallel()

	// Create the pool to test the pricing enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	keys := make([]ed25519.PrivateKey, 3)
	pubs := make([]ed25519.PublicKey, 3)

	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000*1000000))
	}
	// Create transaction (both pending and queued) with a linearly growing gasprice
	for i := uint64(0); i < 500; i++ {
		// Add pending
		p_tx := pricedTransactionCypherium(i, 100000, big.NewInt(int64(i)), pubs[2], keys[2])
		if err := pool.AddLocal(p_tx); err != nil {
			t.Fatal(err)
		}
		// Add queued
		q_tx := pricedTransactionCypherium(i+501, 100000, big.NewInt(int64(i)), pubs[2], keys[2])
		if err := pool.AddLocal(q_tx); err != nil {
			t.Fatal(err)
		}
	}
	pending, queued := pool.Stats()
	expPending, expQueued := 500, 500
	validate := func() {
		pending, queued = pool.Stats()
		if pending != expPending {
			t.Fatalf("pending transactions mismatched: have %d, want %d", pending, expPending)
		}
		if queued != expQueued {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, expQueued)
		}

		if err := validateTxPoolInternals(pool); err != nil {
			t.Fatalf("pool internal state corrupted: %v", err)
		}
	}
	validate()

	// Reprice the pool and check that nothing is dropped
	pool.SetGasPrice(big.NewInt(2))
	validate()

	pool.SetGasPrice(big.NewInt(2))
	pool.SetGasPrice(big.NewInt(4))
	pool.SetGasPrice(big.NewInt(8))
	pool.SetGasPrice(big.NewInt(100))
	validate()
}

// Tests that when the pool reaches its global transaction limit, underpriced
// transactions are gradually shifted out for more expensive ones and any gapped
// pending transactions are moved into the queue.
//
// Note, local transactions are never allowed to be dropped.
func TestTransactionPoolUnderpricing(t *testing.T) {
	t.Parallel()

	// Create the pool to test the pricing enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.GlobalSlots = 2
	config.GlobalQueue = 2

	pool := NewTxPool(config, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Keep track of transaction events to ensure all executables get announced
	events := make(chan TxPreEvent, 32)
	sub := pool.txFeed.Subscribe(events)
	defer sub.Unsubscribe()

	// Create a number of test accounts and fund them
	keys := make([]ed25519.PrivateKey, 3)
	pubs := make([]ed25519.PublicKey, 3)
	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000000))
	}
	// Generate and queue a batch of transactions, both pending and queued
	txs := types.Transactions{}

	txs = append(txs, pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[0], keys[0]))
	txs = append(txs, pricedTransactionCypherium(1, 100000, big.NewInt(2), pubs[0], keys[0]))

	txs = append(txs, pricedTransactionCypherium(1, 100000, big.NewInt(1), pubs[1], keys[1]))

	ltx := pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[2], keys[2])

	// Import the batch and that both pending and queued transactions match up
	pool.AddRemotes(txs)
	pool.AddLocal(ltx)

	pending, queued := pool.Stats()
	if pending != 3 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 3)
	}
	if queued != 1 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 1)
	}
	if err := validateEvents(events, 3); err != nil {
		t.Fatalf("original event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Ensure that adding an underpriced transaction on block limit fails
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[1], keys[1])); err != ErrUnderpriced {
		t.Fatalf("adding underpriced pending transaction error mismatch: have %v, want %v", err, ErrUnderpriced)
	}
	// Ensure that adding high priced transactions drops cheap ones, but not own
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(3), pubs[1], keys[1])); err != nil { // +K1:0 => -K1:1 => Pend K0:0, K0:1, K1:0, K2:0; Que -
		t.Fatalf("failed to add well priced transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100000, big.NewInt(4), pubs[1], keys[1])); err != nil { // +K1:2 => -K0:0 => Pend K1:0, K2:0; Que K0:1 K1:2
		t.Fatalf("failed to add well priced transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(3, 100000, big.NewInt(5), pubs[1], keys[1])); err != nil { // +K1:3 => -K0:1 => Pend K1:0, K2:0; Que K1:2 K1:3
		t.Fatalf("failed to add well priced transaction: %v", err)
	}
	pending, queued = pool.Stats()
	if pending != 2 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 2)
	}
	if queued != 2 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 2)
	}
	if err := validateEvents(events, 1); err != nil {
		t.Fatalf("additional event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Ensure that adding local transactions can push out even higher priced ones
	tx := pricedTransactionCypherium(1, 100000, big.NewInt(0), pubs[2], keys[2])
	if err := pool.AddLocal(tx); err != nil {
		t.Fatalf("failed to add underpriced local transaction: %v", err)
	}
	pending, queued = pool.Stats()
	if pending != 2 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 2)
	}
	if queued != 2 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 2)
	}
	if err := validateEvents(events, 1); err != nil {
		t.Fatalf("local event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that more expensive transactions push out cheap ones from the pool, but
// without producing instability by creating gaps that start jumping transactions
// back and forth between queued/pending.
func TestTransactionPoolStableUnderpricing(t *testing.T) {
	t.Parallel()

	// Create the pool to test the pricing enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.GlobalSlots = 128
	config.GlobalQueue = 0

	pool := NewTxPool(config, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Keep track of transaction events to ensure all executables get announced
	events := make(chan TxPreEvent, 32)
	sub := pool.txFeed.Subscribe(events)
	defer sub.Unsubscribe()

	// Create a number of test accounts and fund them
	keys := make([]ed25519.PrivateKey, 2)
	pubs := make([]ed25519.PublicKey, 2)
	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000000))
	}
	// Fill up the entire queue with the same transaction price points
	txs := types.Transactions{}
	for i := uint64(0); i < config.GlobalSlots; i++ {
		txs = append(txs, pricedTransactionCypherium(i, 100000, big.NewInt(1), pubs[0], keys[0]))
	}
	pool.AddRemotes(txs)

	pending, queued := pool.Stats()
	if pending != int(config.GlobalSlots) {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, config.GlobalSlots)
	}
	if queued != 0 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
	}
	if err := validateEvents(events, int(config.GlobalSlots)); err != nil {
		t.Fatalf("original event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Ensure that adding high priced transactions drops a cheap, but doesn't produce a gap
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(3), pubs[1], keys[1])); err != nil {
		t.Fatalf("failed to add well priced transaction: %v", err)
	}
	pending, queued = pool.Stats()
	if pending != int(config.GlobalSlots) {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, config.GlobalSlots)
	}
	if queued != 0 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
	}
	if err := validateEvents(events, 1); err != nil {
		t.Fatalf("additional event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that the pool rejects replacement transactions that don't meet the minimum
// price bump required.
func TestTransactionReplacement(t *testing.T) {
	t.Parallel()

	// Create the pool to test the pricing enforcement with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Keep track of transaction events to ensure all executables get announced
	events := make(chan TxPreEvent, 32)
	sub := pool.txFeed.Subscribe(events)
	defer sub.Unsubscribe()

	// Create a test account to add transactions with
	pub, key, _ := crypto.GenerateKeyCypherium()
	pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pub), big.NewInt(1000000000))

	// Add pending transactions, ensuring the minimum price bump is enforced for replacement (for ultra low prices too)
	price := int64(100)
	threshold := (price * (100 + int64(testTxPoolConfig.PriceBump))) / 100

	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(1), pub, key)); err != nil {
		t.Fatalf("failed to add original cheap pending transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100001, big.NewInt(1), pub, key)); err != ErrReplaceUnderpriced {
		t.Fatalf("original cheap pending transaction replacement error mismatch: have %v, want %v", err, ErrReplaceUnderpriced)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(2), pub, key)); err != nil {
		t.Fatalf("failed to replace original cheap pending transaction: %v", err)
	}
	if err := validateEvents(events, 2); err != nil {
		t.Fatalf("cheap replacement event firing failed: %v", err)
	}

	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(price), pub, key)); err != nil {
		t.Fatalf("failed to add original proper pending transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100001, big.NewInt(threshold-1), pub, key)); err != ErrReplaceUnderpriced {
		t.Fatalf("original proper pending transaction replacement error mismatch: have %v, want %v", err, ErrReplaceUnderpriced)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(threshold), pub, key)); err != nil {
		t.Fatalf("failed to replace original proper pending transaction: %v", err)
	}
	if err := validateEvents(events, 2); err != nil {
		t.Fatalf("proper replacement event firing failed: %v", err)
	}
	// Add queued transactions, ensuring the minimum price bump is enforced for replacement (for ultra low prices too)
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100000, big.NewInt(1), pub, key)); err != nil {
		t.Fatalf("failed to add original cheap queued transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100001, big.NewInt(1), pub, key)); err != ErrReplaceUnderpriced {
		t.Fatalf("original cheap queued transaction replacement error mismatch: have %v, want %v", err, ErrReplaceUnderpriced)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100000, big.NewInt(2), pub, key)); err != nil {
		t.Fatalf("failed to replace original cheap queued transaction: %v", err)
	}

	if err := pool.AddRemote(pricedTransactionCypherium(2, 100000, big.NewInt(price), pub, key)); err != nil {
		t.Fatalf("failed to add original proper queued transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100001, big.NewInt(threshold-1), pub, key)); err != ErrReplaceUnderpriced {
		t.Fatalf("original proper queued transaction replacement error mismatch: have %v, want %v", err, ErrReplaceUnderpriced)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(2, 100000, big.NewInt(threshold), pub, key)); err != nil {
		t.Fatalf("failed to replace original proper queued transaction: %v", err)
	}

	if err := validateEvents(events, 0); err != nil {
		t.Fatalf("queued replacement event firing failed: %v", err)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that local transactions are journaled to disk, but remote transactions
// get discarded between restarts.
func TestTransactionJournaling(t *testing.T) { testTransactionJournaling(t, false) }

//func TestTransactionJournalingNoLocals(t *testing.T) { testTransactionJournaling(t, true) }

func testTransactionJournaling(t *testing.T, nolocals bool) {
	t.Parallel()

	// Create a temporary file for the journal
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("failed to create temporary journal: %v", err)
	}
	journal := file.Name()
	defer os.Remove(journal)

	// Clean up the temporary file, we only need the path for now
	file.Close()
	os.Remove(journal)

	// Create the original pool to inject transaction into the journal
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.NoLocals = nolocals
	config.Journal = journal
	config.Rejournal = time.Second

	pool := NewTxPool(config, params.TestChainConfig, blockchain)

	// Create two test accounts to ensure remotes expire but locals do not
	localp, local, _ := crypto.GenerateKeyCypherium()
	remotep, remote, _ := crypto.GenerateKeyCypherium()

	pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(localp), big.NewInt(1000000000))
	pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(remotep), big.NewInt(1000000000))

	// Add three local and a remote transactions and ensure they are queued up
	if err := pool.AddLocal(pricedTransactionCypherium(0, 100000, big.NewInt(1), localp, local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddLocal(pricedTransactionCypherium(1, 100000, big.NewInt(1), localp, local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddLocal(pricedTransactionCypherium(2, 100000, big.NewInt(1), localp, local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransactionCypherium(0, 100000, big.NewInt(1), remotep, remote)); err != nil {
		t.Fatalf("failed to add remote transaction: %v", err)
	}
	pending, queued := pool.Stats()
	if pending != 4 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 4)
	}
	if queued != 0 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Terminate the old pool, bump the local nonce, create a new pool and ensure relevant transaction survive
	pool.Stop()
	statedb.SetNonce(crypto.PubkeyToAddressCypherium(localp), 1)
	blockchain = &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool = NewTxPool(config, params.TestChainConfig, blockchain)

	pending, queued = pool.Stats()
	if queued != 0 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
	}
	if nolocals {
		if pending != 0 {
			t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
		}
	} else {
		if pending != 2 {
			t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 2)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Bump the nonce temporarily and ensure the newly invalidated transaction is removed
	statedb.SetNonce(crypto.PubkeyToAddressCypherium(localp), 2)
	pool.lockedReset(nil, nil)
	time.Sleep(2 * config.Rejournal)
	pool.Stop()

	statedb.SetNonce(crypto.PubkeyToAddressCypherium(localp), 1)
	blockchain = &testBlockChain{statedb, 1000000, new(event.Feed)}
	pool = NewTxPool(config, params.TestChainConfig, blockchain)

	pending, queued = pool.Stats()
	if pending != 0 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
	}
	if nolocals {
		if queued != 0 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
		}
	} else {
		if queued != 1 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 1)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	pool.Stop()
}

// TestTransactionStatusCheck tests that the pool can correctly retrieve the
// pending status of individual transactions.
func TestTransactionStatusCheck(t *testing.T) {
	t.Parallel()

	// Create the pool to test the status retrievals with
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain)
	defer pool.Stop()

	// Create the test accounts to check various transaction statuses with
	keys := make([]ed25519.PrivateKey, 3)
	pubs := make([]ed25519.PublicKey, 3)

	for i := 0; i < len(keys); i++ {
		pubs[i], keys[i], _ = crypto.GenerateKeyCypherium()
		pool.currentState.AddBalance(crypto.PubkeyToAddressCypherium(pubs[i]), big.NewInt(1000000))
	}
	// Generate and queue a batch of transactions, both pending and queued
	txs := types.Transactions{}

	txs = append(txs, pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[0], keys[0])) // Pending only
	txs = append(txs, pricedTransactionCypherium(0, 100000, big.NewInt(1), pubs[1], keys[1])) // Pending and queued
	txs = append(txs, pricedTransactionCypherium(2, 100000, big.NewInt(1), pubs[1], keys[1]))
	txs = append(txs, pricedTransactionCypherium(2, 100000, big.NewInt(1), pubs[2], keys[2])) // Queued only

	// Import the transaction and ensure they are correctly added
	pool.AddRemotes(txs)

	pending, queued := pool.Stats()
	if pending != 2 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 2)
	}
	if queued != 2 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 2)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Retrieve the status of each transaction and validate them
	hashes := make([]common.Hash, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.Hash()
	}
	hashes = append(hashes, common.Hash{})

	statuses := pool.Status(hashes)
	expect := []TxStatus{TxStatusPending, TxStatusPending, TxStatusQueued, TxStatusQueued, TxStatusUnknown}

	for i := 0; i < len(statuses); i++ {
		if statuses[i] != expect[i] {
			t.Errorf("transaction %d: status mismatch: have %v, want %v", i, statuses[i], expect[i])
		}
	}
}

// Benchmarks the speed of validating the contents of the pending queue of the
// transaction pool.
func BenchmarkPendingDemotion100(b *testing.B)   { benchmarkPendingDemotion(b, 100) }
func BenchmarkPendingDemotion1000(b *testing.B)  { benchmarkPendingDemotion(b, 1000) }
func BenchmarkPendingDemotion10000(b *testing.B) { benchmarkPendingDemotion(b, 10000) }

func benchmarkPendingDemotion(b *testing.B, size int) {
	// Add a batch of transactions to a pool one by one
	pool, key := setupTxPool()
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	for i := 0; i < size; i++ {
		tx := transaction(uint64(i), 100000, key)
		pool.promoteTx(account, tx.Hash(), tx)
	}
	// Benchmark the speed of pool validation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.demoteUnexecutables()
	}
}

// Benchmarks the speed of scheduling the contents of the future queue of the
// transaction pool.
func BenchmarkFuturePromotion100(b *testing.B)   { benchmarkFuturePromotion(b, 100) }
func BenchmarkFuturePromotion1000(b *testing.B)  { benchmarkFuturePromotion(b, 1000) }
func BenchmarkFuturePromotion10000(b *testing.B) { benchmarkFuturePromotion(b, 10000) }

func benchmarkFuturePromotion(b *testing.B, size int) {
	// Add a batch of transactions to a pool one by one
	pool, key := setupTxPool()
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	for i := 0; i < size; i++ {
		tx := transaction(uint64(1+i), 100000, key)
		pool.enqueueTx(tx.Hash(), tx)
	}
	// Benchmark the speed of pool validation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.promoteExecutables(nil)
	}
}

// Benchmarks the speed of iterative transaction insertion.
func BenchmarkPoolInsert(b *testing.B) {
	// Generate a batch of transactions to enqueue into the pool
	pool, key := setupTxPool()
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	txs := make(types.Transactions, b.N)
	for i := 0; i < b.N; i++ {
		txs[i] = transaction(uint64(i), 100000, key)
	}
	// Benchmark importing the transactions into the queue
	b.ResetTimer()
	for _, tx := range txs {
		pool.AddRemote(tx)
	}
}

// Benchmarks the speed of batched transaction insertion.
func BenchmarkPoolBatchInsert100(b *testing.B)   { benchmarkPoolBatchInsert(b, 100) }
func BenchmarkPoolBatchInsert1000(b *testing.B)  { benchmarkPoolBatchInsert(b, 1000) }
func BenchmarkPoolBatchInsert10000(b *testing.B) { benchmarkPoolBatchInsert(b, 10000) }

func benchmarkPoolBatchInsert(b *testing.B, size int) {
	// Generate a batch of transactions to enqueue into the pool
	pool, key := setupTxPool()
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	batches := make([]types.Transactions, b.N)
	for i := 0; i < b.N; i++ {
		batches[i] = make(types.Transactions, size)
		for j := 0; j < size; j++ {
			batches[i][j] = transaction(uint64(size*i+j), 100000, key)
		}
	}
	// Benchmark importing the transactions into the queue
	b.ResetTimer()
	for _, batch := range batches {
		pool.AddRemotes(batch)
	}
}
