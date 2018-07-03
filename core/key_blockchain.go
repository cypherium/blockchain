// Package core implements the Cypherium consensus protocol.
package core

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cypherium_private/go-cypherium/common"
	"github.com/cypherium_private/go-cypherium/common/mclock"
	"github.com/cypherium_private/go-cypherium/consensus"
	"github.com/cypherium_private/go-cypherium/core/rawdb"
	"github.com/cypherium_private/go-cypherium/core/state"
	"github.com/cypherium_private/go-cypherium/core/types"
	"github.com/cypherium_private/go-cypherium/core/vm"
	"github.com/cypherium_private/go-cypherium/ethdb"
	"github.com/cypherium_private/go-cypherium/event"
	"github.com/cypherium_private/go-cypherium/log"
	"github.com/cypherium_private/go-cypherium/metrics"
	"github.com/cypherium_private/go-cypherium/params"
	"github.com/hashicorp/golang-lru"
)

var (
	keyBlockInsertTimer = metrics.NewRegisteredTimer("keyblockchain/inserts", nil)

	ErrNoKeyBlockGenesis = errors.New("KeyBlock Genesis not found in chain")
)

const (
	keyBlockBodyCacheLimit = 256
	KeyBlockCacheLimit     = 256
	maxFutureKeyBlocks     = 256
	maxTimeFutureKeyBlocks = 30
	badKeyBlockLimit       = 10
	// triesInMemory          = 128

	// KeyBlockChainVersion ensures that an incompatible database forces a resync from scratch.
	KeyBlockChainVersion = 3
)

// // KeyBlockCacheConfig contains the configuration values for the trie caching/pruning
// // that's resident in a blockchain.
// type KeyBlockCacheConfig struct {
// 	Disabled      bool          // Whether to disable trie write caching (archive node)
// 	TrieNodeLimit int           // Memory limit (MB) at which to flush the current in-memory trie to disk
// 	TrieTimeLimit time.Duration // Time limit after which to flush the current in-memory trie to disk
// }

// KeyBlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
//
// Importing blocks in to the block chain happens according to the set of rules
// defined by the two stage Validator. Processing of blocks is done using the
// Processor which processes the included transaction. The validation of the state
// is done in the second part of the Validator. Failing results in aborting of
// the import.
//
// The KeyBlockChain also helps in returning blocks from **any** chain included
// in the database as well as blocks that represents the canonical chain. It's
// important to note that GetBlock can return any block and does not need to be
// included in the canonical one where as GetBlockByNumber always represents the
// canonical chain.
type KeyBlockChain struct {
	chainConfig *params.KeyBlockChainConfig // Chain & network configuration //TODO
	// cacheConfig *CacheConfig                // Cache configuration for pruning

	db ethdb.Database // Low level persistent database to store final content in
	// triegc *prque.Prque   // Priority queue mapping block numbers to tries to gc
	gcproc time.Duration // Accumulates canonical block processing for trie dumping

	hc *KeyBlockHeaderChain
	// rmLogsFeed    event.Feed
	keyBlockChainFeed     event.Feed
	keyBlockChainSideFeed event.Feed
	keyBlockChainHeadFeed event.Feed
	// logsFeed              event.Feed
	scope        event.SubscriptionScope
	genesisBlock *types.KeyBlock

	mu      sync.RWMutex // global mutex for locking chain operations
	chainmu sync.RWMutex // blockchain insertion lock
	procmu  sync.RWMutex // block processor lock

	checkpoint          int          // checkpoint counts towards the new checkpoint
	currentKeyBlock     atomic.Value // Current head of the block chain
	currentFastKeyBlock atomic.Value // Current head of the fast-sync chain (may be above the block chain!)

	// stateCache      state.Database // State database to reuse between imports (contains state cache)
	bodyCache         *lru.Cache // Cache for the most recent block bodies
	bodyProtoBufCache *lru.Cache // Cache for the most recent block bodies in protobuf encoded format
	keyBlockCache     *lru.Cache // Cache for the most recent entire blocks
	futureKeyBlocks   *lru.Cache // future blocks are blocks added for later processing

	quit    chan struct{} // blockchain quit channel
	running int32         // running must be called atomically
	// procInterrupt must be atomically called
	procInterrupt int32          // interrupt signaler for block processing
	wg            sync.WaitGroup // chain processing wait group for shutting down

	engine    consensus.Engine
	processor Processor         // block processor interface
	validator KeyBlockValidator // block and state validator interface
	vmConfig  vm.Config

	badKeyBlocks *lru.Cache // Bad keyblock cache
}

// NewKeyBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Ethereum Validator and
// Processor.
func NewKeyBlockChain(db ethdb.Database, cacheConfig *CacheConfig, chainConfig *params.ChainConfig, engine consensus.Engine, vmConfig vm.Config) (*KeyBlockChain, error) {
	// if cacheConfig == nil {
	// 	cacheConfig = &CacheConfig{
	// 		TrieNodeLimit: 256 * 1024 * 1024,
	// 		TrieTimeLimit: 5 * time.Minute,
	// 	}
	// }
	bodyCache, _ := lru.New(keyBlockBodyCacheLimit)
	bodyProtoBufCache, _ := lru.New(keyBlockBodyCacheLimit)
	keyBlockCache, _ := lru.New(KeyBlockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureKeyBlocks)
	badKeyBlocks, _ := lru.New(badKeyBlockLimit)

	bc := &KeyBlockChain{
		chainConfig: chainConfig,
		// cacheConfig:  cacheConfig,
		db: db,
		// triegc:       prque.New(),
		// stateCache:   state.NewDatabase(db),
		quit:              make(chan struct{}),
		bodyCache:         bodyCache,
		bodyProtoBufCache: bodyProtoBufCache,
		keyBlockCache:     keyBlockCache,
		futureKeyBlocks:   futureBlocks,
		engine:            engine,
		vmConfig:          vmConfig,
		badKeyBlocks:      badKeyBlocks,
	}
	bc.SetValidator(NewKeyBlockValidator(chainConfig, bc, engine))
	// bc.SetProcessor(NewStateProcessor(chainConfig, bc, engine))

	var err error
	bc.hc, err = NewKeyBlockHeaderChain(db, chainConfig, engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}
	bc.genesisBlock = bc.GetKeyBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
	// Check the current state of the block hashes and make sure that we do not have any of the bad blocks in our chain
	for hash := range BadHashes {
		if header := bc.GetHeaderByHash(hash); header != nil {
			// get the canonical block corresponding to the offending header's number
			headerByNumber := bc.GetHeaderByNumber(types.ProtobufFormartToBigInt(header.Number).Uint64())
			// make sure the headerByNumber (if present) is in our current canonical chain
			if headerByNumber != nil && headerByNumber.Hash() == header.Hash() {
				log.Error("Found bad hash, rewinding chain", "number", header.Number.Abs, "hash", header.PKblkHash)
				bc.SetHead(types.ProtobufFormartToBigInt(header.Number).Uint64() - 1)
				log.Error("Chain rewind was successful, resuming normal operation")
			}
		}
	}
	// Take ownership of this particular state
	go bc.update()
	return bc, nil
}

func (bc *KeyBlockChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&bc.procInterrupt) == 1
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *KeyBlockChain) loadLastState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadKeyBlockHash(bc.db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		log.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlockByHash(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		log.Warn("Head block missing, resetting chain", "hash", head)
		return bc.Reset()
	}
	// Make sure the state associated with the block is available
	// if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
	// 	// Dangling block without a state associated, init from scratch
	// 	log.Warn("Head state missing, repairing chain", "number", currentBlock.Number(), "hash", currentBlock.Hash())
	// 	if err := bc.repair(&currentBlock); err != nil {
	// 		return err
	// 	}
	// }
	// Everything seems to be fine, set as the head block
	bc.currentKeyBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadKeyBlockHeaderHash(bc.db); head != (common.Hash{}) {
		if header := bc.GetHeaderByHash(head); header != nil {
			currentHeader = header
		}
	}
	bc.hc.SetCurrentHeader(currentHeader)

	// Restore the last known head fast block
	bc.currentFastKeyBlock.Store(currentBlock)
	if head := rawdb.ReadHeadFastKeyBlockHash(bc.db); head != (common.Hash{}) {
		if block := bc.GetBlockByHash(head); block != nil {
			bc.currentFastKeyBlock.Store(block)
		}
	}

	// Issue a status log for the user
	// currentFastBlock := bc.CurrentFastBlock()

	// headerTd := bc.GetTd(currentHeader.Hash(), currentHeader.Number.Uint64())
	// blockTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	// fastTd := bc.GetTd(currentFastBlock.Hash(), currentFastBlock.NumberU64())

	// log.Info("Loaded most recent local header", "number", currentHeader.Number, "hash", currentHeader.Hash(), "td", headerTd)
	// log.Info("Loaded most recent local full block", "number", currentBlock.Number(), "hash", currentBlock.Hash(), "td", blockTd)
	// log.Info("Loaded most recent local fast block", "number", currentFastBlock.Number(), "hash", currentFastBlock.Hash(), "td", fastTd)

	return nil
}

// SetHead rewinds the local chain to a new head. In the case of headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (bc *KeyBlockChain) SetHead(head uint64) error {
	log.Warn("Rewinding blockchain", "target", head)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Rewind the header chain, deleting all block bodies until then
	delFn := func(hash common.Hash, num uint64) {
		rawdb.DeleteKeyBlockBody(bc.db, hash, num)
	}
	bc.hc.SetHead(head, delFn)
	currentHeader := bc.hc.CurrentHeader()

	// Clear out any stale content from the caches
	bc.bodyCache.Purge()
	bc.bodyProtoBufCache.Purge()
	bc.keyBlockCache.Purge()
	bc.futureKeyBlocks.Purge()

	// Rewind the block chain, ensuring we don't end up with a stateless head block
	if currentBlock := bc.CurrentBlock(); currentBlock != nil && types.ProtobufFormartToBigInt(currentHeader.Number).Uint64() < currentBlock.NumberU64() {
		bc.currentKeyBlock.Store(bc.GetBlock(currentHeader.Hash(), types.ProtobufFormartToBigInt(currentHeader.Number).Uint64()))
	}
	// if currentBlock := bc.CurrentBlock(); currentBlock != nil {
	// 	if _, err := state.New(currentBlock.Root(), bc.stateCache); err != nil {
	// 		// Rewound state missing, rolled back to before pivot, reset to genesis
	// 		bc.currentBlock.Store(bc.genesisBlock)
	// 	}
	// }
	// Rewind the fast block in a simpleton way to the target head
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock != nil && types.ProtobufFormartToBigInt(currentHeader.Number).Uint64() < currentFastBlock.NumberU64() {
		bc.currentFastKeyBlock.Store(bc.GetBlock(currentHeader.Hash(), types.ProtobufFormartToBigInt(currentHeader.Number).Uint64()))
	}
	// If either blocks reached nil, reset to the genesis state
	if currentBlock := bc.CurrentBlock(); currentBlock == nil {
		bc.currentKeyBlock.Store(bc.genesisBlock)
	}
	if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock == nil {
		bc.currentFastKeyBlock.Store(bc.genesisBlock)
	}
	currentBlock := bc.CurrentBlock()
	currentFastBlock := bc.CurrentFastBlock()

	rawdb.WriteHeadKeyBlockHash(bc.db, currentBlock.Hash())
	rawdb.WriteHeadFastKeyBlockHash(bc.db, currentFastBlock.Hash())

	return bc.loadLastState()
}

// FastSyncCommitHead sets the current head block to the one defined by the hash
// irrelevant what the chain contents were prior.
func (bc *KeyBlockChain) FastSyncCommitHead(hash common.Hash) error {
	// Make sure that both the block as well at its state trie exists
	block := bc.GetBlockByHash(hash)
	if block == nil {
		return fmt.Errorf("non existent block [%x…]", hash[:4])
	}
	// if _, err := trie.NewSecure(block.Root(), bc.stateCache.TrieDB(), 0); err != nil {
	// 	return err
	// }
	// If all checks out, manually set the head block
	bc.mu.Lock()
	bc.currentKeyBlock.Store(block)
	bc.mu.Unlock()

	log.Info("Committed new head block", "number", block.Number(), "hash", hash)
	return nil
}

// GasLimit returns the gas limit of the current HEAD block.
// func (bc *KeyBlockChain) GasLimit() uint64 {
// 	return bc.CurrentBlock().GasLimit()
// }

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (bc *KeyBlockChain) CurrentBlock() *types.KeyBlock {
	return bc.currentKeyBlock.Load().(*types.KeyBlock)
}

// CurrentFastBlock retrieves the current fast-sync head block of the canonical
// chain. The block is retrieved from the blockchain's internal cache.
func (bc *KeyBlockChain) CurrentFastBlock() *types.KeyBlock {
	return bc.currentFastBlock.Load().(*types.KeyBlock)
}

// SetProcessor sets the processor required for making state modifications.
func (bc *KeyBlockChain) SetProcessor(processor Processor) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.processor = processor
}

// SetValidator sets the validator which is used to validate incoming blocks.
func (bc *KeyBlockChain) SetValidator(validator KeyBlockValidator) { //TODO KeyBlockValidator
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.validator = validator
}

// Validator returns the current validator.
func (bc *KeyBlockChain) Validator() KeyBlockValidator {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.validator
}

// Processor returns the current processor.
func (bc *KeyBlockChain) Processor() KeyBlockProcessor {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.processor
}

// // State returns a new mutable state based on the current HEAD block.
// func (bc *KeyBlockChain) State() (*state.StateDB, error) {
// 	return bc.StateAt(bc.CurrentBlock().Root())
// }

// // StateAt returns a new mutable state based on a particular point in time.
// func (bc *KeyBlockChain) StateAt(root common.Hash) (*state.StateDB, error) {
// 	return state.New(root, bc.stateCache)
// }

// Reset purges the entire blockchain, restoring it to its genesis state.
func (bc *KeyBlockChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (bc *KeyBlockChain) ResetWithGenesisBlock(genesis *types.KeyBlock) error {
	// Dump the entire block chain and purge the caches
	if err := bc.SetHead(0); err != nil {
		return err
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Prepare the genesis block and reinitialise the chain
	// if err := bc.hc.WriteTd(genesis.Hash(), genesis.NumberU64(), genesis.Difficulty()); err != nil {
	// 	log.Crit("Failed to write genesis block TD", "err", err)
	// }
	rawdb.WriteKeyBlock(bc.db, genesis)

	bc.genesisBlock = genesis
	bc.insert(bc.genesisBlock)
	bc.currentKeyBlock.Store(bc.genesisBlock)
	bc.hc.SetGenesis(bc.genesisBlock.Header())
	bc.hc.SetCurrentHeader(bc.genesisBlock.Header())
	bc.currentFastKeyBlock.Store(bc.genesisBlock)

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (bc *KeyBlockChain) repair(head **types.KeyBlock) error {
	// for {
	// 	// Abort if we've rewound to a head block that does have associated state
	// 	if _, err := state.New((*head).Root(), bc.stateCache); err == nil {
	// 		log.Info("Rewound blockchain to past state", "number", (*head).Number(), "hash", (*head).Hash())
	// 		return nil
	// 	}
	// 	// Otherwise rewind one block and recheck state availability there
	// 	(*head) = bc.GetBlock((*head).ParentHash(), (*head).NumberU64()-1)
	// }
	return nil
}

// Export writes the active chain to the given writer.
func (bc *KeyBlockChain) Export(w io.Writer) error {
	return bc.ExportN(w, uint64(0), bc.CurrentBlock().NumberU64())
}

// ExportN writes a subset of the active chain to the given writer.
func (bc *KeyBlockChain) ExportN(w io.Writer, first uint64, last uint64) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if first > last {
		return fmt.Errorf("export failed: first (%d) is greater than last (%d)", first, last)
	}
	log.Info("Exporting batch of blocks", "count", last-first+1)

	for nr := first; nr <= last; nr++ {
		block := bc.GetBlockByNumber(nr)
		if block == nil {
			return fmt.Errorf("export failed on #%d: not found", nr)
		}

		if err := block.EncodeProtoBuf(w); err != nil {
			return err
		}
	}

	return nil
}

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *KeyBlockChain) insert(block *types.KeyBlock) {
	// If the block is on a side chain or an unknown one, force other heads onto it too
	updateHeads := rawdb.ReadKeyBlockHash(bc.db, block.NumberU64()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	rawdb.WriteKeyBlockHash(bc.db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadKeyBlockHash(bc.db, block.Hash())

	bc.currentKeyBlock.Store(block)

	// If the block is better than our head or is on a different chain, force update heads
	if updateHeads {
		bc.hc.SetCurrentHeader(block.Header())
		rawdb.WriteHeadFastKeyBlockHash(bc.db, block.Hash())

		bc.currentFastKeyBlock.Store(block)
	}
}

// Genesis retrieves the chain's genesis block.
func (bc *KeyBlockChain) Genesis() *types.KeyBlock {
	return bc.genesisBlock
}

// GetBody retrieves a block body (transactions and uncles) from the database by
// hash, caching it if found.
func (bc *KeyBlockChain) GetBody(hash common.Hash) *types.MainBody {
	// Short circuit if the body's already in the cache, retrieve otherwise
	if cached, ok := bc.bodyCache.Get(hash); ok {
		body := cached.(*types.MainBody)
		return body
	}
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	body := rawdb.ReadKeyBlockBody(bc.db, hash, *number)
	if body == nil {
		return nil
	}
	// Cache the found body for next time and return
	bc.bodyCache.Add(hash, body)
	return body
}

// GetBodyProtoBuf retrieves a block body in RLP encoding from the database by hash,
// caching it if found.
func (bc *KeyBlockChain) GetBodyProtoBuf(hash common.Hash) []byte {
	// Short circuit if the body's already in the cache, retrieve otherwise
	if cached, ok := bc.bodyProtoBufCache.Get(hash); ok {
		return cached.([]byte)
	}
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	body := rawdb.ReadKeyBlockBodyProtoBuf(bc.db, hash, *number)
	if len(body) == 0 {
		return nil
	}
	// Cache the found body for next time and return
	bc.bodyProtoBufCache.Add(hash, body)
	return body
}

// HasBlock checks if a block is fully present in the database or not.
func (bc *KeyBlockChain) HasBlock(hash common.Hash, number uint64) bool {
	if bc.keyBlockCache.Contains(hash) {
		return true
	}
	return rawdb.KeyBlockHasBody(bc.db, hash, number)
}

// // HasState checks if state trie is fully present in the database or not.
// func (bc *KeyBlockChain) HasState(hash common.Hash) bool {
// 	_, err := bc.stateCache.OpenTrie(hash)
// 	return err == nil
// }

// HasBlockAndState checks if a block and associated state trie is fully present
// in the database or not, caching it if present.
func (bc *KeyBlockChain) HasBlockAndState(hash common.Hash, number uint64) bool {
	// Check first that the block itself is known
	block := bc.GetBlock(hash, number)
	if block == nil {
		return false
	}
	// return bc.HasState(block.Root())
	return true
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (bc *KeyBlockChain) GetBlock(hash common.Hash, number uint64) *types.KeyBlock {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.keyBlockCache.Get(hash); ok {
		return block.(*types.KeyBlock)
	}
	block := rawdb.ReadKeyBlock(bc.db, hash, number)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	bc.keyBlockCache.Add(block.Hash(), block)
	return block
}

// GetBlockByHash retrieves a block from the database by hash, caching it if found.
func (bc *KeyBlockChain) GetBlockByHash(hash common.Hash) *types.KeyBlock {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return bc.GetBlock(hash, *number)
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *KeyBlockChain) GetBlockByNumber(number uint64) *types.KeyBlock {
	hash := rawdb.ReadKeyBlockHash(bc.db, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash, number)
}

// GetReceiptsByHash retrieves the receipts for all transactions in a given block.
// func (bc *KeyBlockChain) GetReceiptsByHash(hash common.Hash) types.Receipts {
// 	number := rawdb.ReadHeaderNumber(bc.db, hash)
// 	if number == nil {
// 		return nil
// 	}
// 	return rawdb.ReadReceipts(bc.db, hash, *number)
// }

// GetBlocksFromHash returns the block corresponding to hash and up to n-1 ancestors.
// [deprecated by eth/62]
func (bc *KeyBlockChain) GetBlocksFromHash(hash common.Hash, n int) (blocks []*types.KeyBlock) {
	number := bc.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	for i := 0; i < n; i++ {
		block := bc.GetBlock(hash, *number)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		hash = block.ParentHash()
		*number--
	}
	return
}

// GetUnclesInChain retrieves all the uncles from a given block backwards until
// a specific distance is reached.
// func (bc *KeyBlockChain) GetUnclesInChain(block *types.KeyBlock, length int) []*types.KeyBlockHdr {
// 	uncles := []*types.KeyBlockHdr{}
// 	for i := 0; block != nil && i < length; i++ {
// 		uncles = append(uncles, block.Uncles()...)
// 		block = bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
// 	}
// 	return uncles
// }

// TrieNode retrieves a blob of data associated with a trie node (or code hash)
// either from ephemeral in-memory cache, or from persistent storage.
// func (bc *KeyBlockChain) TrieNode(hash common.Hash) ([]byte, error) {
// 	return bc.stateCache.TrieDB().Node(hash)
// }

// Stop stops the blockchain service. If any imports are currently in progress
// it will abort them using the procInterrupt.
func (bc *KeyBlockChain) Stop() {
	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}
	// Unsubscribe all subscriptions registered from blockchain
	bc.scope.Close()
	close(bc.quit)
	atomic.StoreInt32(&bc.procInterrupt, 1)

	bc.wg.Wait()

	// Ensure the state of a recent block is also stored to disk before exiting.
	// We're writing three different states to catch different restart scenarios:
	//  - HEAD:     So we don't need to reprocess any blocks in the general case
	//  - HEAD-1:   So we don't do large reorgs if our HEAD becomes an uncle
	//  - HEAD-127: So we have a hard limit on the number of blocks reexecuted
	// if !bc.cacheConfig.Disabled {
	// 	triedb := bc.stateCache.TrieDB()

	// 	for _, offset := range []uint64{0, 1, triesInMemory - 1} {
	// 		if number := bc.CurrentBlock().NumberU64(); number > offset {
	// 			recent := bc.GetBlockByNumber(number - offset)

	// 			log.Info("Writing cached state to disk", "block", recent.Number(), "hash", recent.Hash(), "root", recent.Root())
	// 			if err := triedb.Commit(recent.Root(), true); err != nil {
	// 				log.Error("Failed to commit recent state trie", "err", err)
	// 			}
	// 		}
	// 	}
	// 	for !bc.triegc.Empty() {
	// 		triedb.Dereference(bc.triegc.PopItem().(common.Hash), common.Hash{})
	// 	}
	// 	if size := triedb.Size(); size != 0 {
	// 		log.Error("Dangling trie nodes after full cleanup")
	// 	}
	// }
	log.Info("Blockchain manager stopped")
}

func (bc *KeyBlockChain) procFutureBlocks() {
	blocks := make([]*types.KeyBlock, 0, bc.futureKeyBlocks.Len())
	for _, hash := range bc.futureKeyBlocks.Keys() {
		if block, exist := bc.futureKeyBlocks.Peek(hash); exist {
			blocks = append(blocks, block.(*types.KeyBlock))
		}
	}
	if len(blocks) > 0 {
		types.BlockBy(types.KeyBlockNumber).Sort(blocks)

		// Insert one by one as chain insertion needs contiguous ancestry between blocks
		for i := range blocks {
			bc.InsertChain(blocks[i : i+1])
		}
	}
}

// WriteKeyBlockStatus status of write
type WriteKeyBlockStatus byte

const (
	NonStatTy WriteKeyBlockStatus = iota
	CanonStatTy
	SideStatTy
)

// Rollback is designed to remove a chain of links from the database that aren't
// certain enough to be valid.
func (bc *KeyBlockChain) Rollback(chain []common.Hash) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		currentHeader := bc.hc.CurrentHeader()
		if currentHeader.Hash() == hash {
			bc.hc.SetCurrentHeader(bc.GetHeader(currentHeader.PKblkHash, types.ProtobufFormartToBigInt(currentHeader.Number).Uint64()-1))
		}
		if currentFastBlock := bc.CurrentFastBlock(); currentFastBlock.Hash() == hash {
			newFastBlock := bc.GetBlock(currentFastBlock.ParentHash(), currentFastBlock.NumberU64()-1)
			bc.currentFastKeyBlock.Store(newFastBlock)
			rawdb.WriteHeadFastKeyBlockHash(bc.db, newFastBlock.Hash())
		}
		if currentBlock := bc.CurrentBlock(); currentBlock.Hash() == hash {
			newBlock := bc.GetBlock(currentBlock.ParentHash(), currentBlock.NumberU64()-1)
			bc.currentKeyBlock.Store(newBlock)
			rawdb.WriteHeadKeyBlockHash(bc.db, newBlock.Hash())
		}
	}
}

// SetReceiptsData computes all the non-consensus fields of the receipts
// func SetReceiptsData(config *params.ChainConfig, block *types.KeyBlock, receipts types.Receipts) error {
// 	signer := types.MakeSigner(config, block.Number())

// 	transactions, logIndex := block.Transactions(), uint(0)
// 	if len(transactions) != len(receipts) {
// 		return errors.New("transaction and receipt count mismatch")
// 	}

// 	for j := 0; j < len(receipts); j++ {
// 		// The transaction hash can be retrieved from the transaction itself
// 		receipts[j].TxHash = transactions[j].Hash()

// 		// The contract address can be derived from the transaction itself
// 		if transactions[j].To() == nil {
// 			// Deriving the signer is expensive, only do if it's actually needed
// 			from, _ := types.Sender(signer, transactions[j])
// 			receipts[j].ContractAddress = crypto.CreateAddress(from, transactions[j].Nonce())
// 		}
// 		// The used gas can be calculated based on previous receipts
// 		if j == 0 {
// 			receipts[j].GasUsed = receipts[j].CumulativeGasUsed
// 		} else {
// 			receipts[j].GasUsed = receipts[j].CumulativeGasUsed - receipts[j-1].CumulativeGasUsed
// 		}
// 		// The derived log fields can simply be set from the block and transaction
// 		for k := 0; k < len(receipts[j].Logs); k++ {
// 			receipts[j].Logs[k].BlockNumber = block.NumberU64()
// 			receipts[j].Logs[k].BlockHash = block.Hash()
// 			receipts[j].Logs[k].TxHash = receipts[j].TxHash
// 			receipts[j].Logs[k].TxIndex = uint(j)
// 			receipts[j].Logs[k].Index = logIndex
// 			logIndex++
// 		}
// 	}
// 	return nil
// }

// InsertReceiptChain attempts to complete an already existing header chain with
// transaction and receipt data.
// func (bc *KeyBlockChain) InsertReceiptChain(blockChain types.KeyBlocks, receiptChain []types.Receipts) (int, error) {
// 	bc.wg.Add(1)
// 	defer bc.wg.Done()

// 	// Do a sanity check that the provided chain is actually ordered and linked
// 	for i := 1; i < len(blockChain); i++ {
// 		if blockChain[i].NumberU64() != blockChain[i-1].NumberU64()+1 || blockChain[i].ParentHash() != blockChain[i-1].Hash() {
// 			log.Error("Non contiguous receipt insert", "number", blockChain[i].Number(), "hash", blockChain[i].Hash(), "parent", blockChain[i].ParentHash(),
// 				"prevnumber", blockChain[i-1].Number(), "prevhash", blockChain[i-1].Hash())
// 			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, blockChain[i-1].NumberU64(),
// 				blockChain[i-1].Hash().Bytes()[:4], i, blockChain[i].NumberU64(), blockChain[i].Hash().Bytes()[:4], blockChain[i].ParentHash().Bytes()[:4])
// 		}
// 	}

// 	var (
// 		stats = struct{ processed, ignored int32 }{}
// 		start = time.Now()
// 		bytes = 0
// 		batch = bc.db.NewBatch()
// 	)
// 	for i, block := range blockChain {
// 		receipts := receiptChain[i]
// 		// Short circuit insertion if shutting down or processing failed
// 		if atomic.LoadInt32(&bc.procInterrupt) == 1 {
// 			return 0, nil
// 		}
// 		// Short circuit if the owner header is unknown
// 		if !bc.HasHeader(block.Hash(), block.NumberU64()) {
// 			return i, fmt.Errorf("containing header #%d [%x…] unknown", block.Number(), block.Hash().Bytes()[:4])
// 		}
// 		// Skip if the entire data is already known
// 		if bc.HasBlock(block.Hash(), block.NumberU64()) {
// 			stats.ignored++
// 			continue
// 		}
// 		// Compute all the non-consensus fields of the receipts
// 		if err := SetReceiptsData(bc.chainConfig, block, receipts); err != nil {
// 			return i, fmt.Errorf("failed to set receipts data: %v", err)
// 		}
// 		// Write all the data out into the database
// 		rawdb.WriteBody(batch, block.Hash(), block.NumberU64(), block.Body()) //TODO
// 		rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)
// 		rawdb.WriteTxLookupEntries(batch, block)

// 		stats.processed++

// 		if batch.ValueSize() >= ethdb.IdealBatchSize {
// 			if err := batch.Write(); err != nil {
// 				return 0, err
// 			}
// 			bytes += batch.ValueSize()
// 			batch.Reset()
// 		}
// 	}
// 	if batch.ValueSize() > 0 {
// 		bytes += batch.ValueSize()
// 		if err := batch.Write(); err != nil {
// 			return 0, err
// 		}
// 	}

// 	// Update the head fast sync block if better
// 	bc.mu.Lock()
// 	head := blockChain[len(blockChain)-1]
// 	if td := bc.GetTd(head.Hash(), head.NumberU64()); td != nil { // Rewind may have occurred, skip in that case
// 		currentFastBlock := bc.CurrentFastBlock()
// 		if bc.GetTd(currentFastBlock.Hash(), currentFastBlock.NumberU64()).Cmp(td) < 0 {
// 			rawdb.WriteHeadFastBlockHash(bc.db, head.Hash()) //TODO
// 			bc.currentFastKeyBlock.Store(head)
// 		}
// 	}
// 	bc.mu.Unlock()

// 	log.Info("Imported new block receipts",
// 		"count", stats.processed,
// 		"elapsed", common.PrettyDuration(time.Since(start)),
// 		"number", head.Number(),
// 		"hash", head.Hash(),
// 		"size", common.StorageSize(bytes),
// 		"ignored", stats.ignored)
// 	return 0, nil
// }

var keyBlockLastWrite uint64

// WriteBlockWithoutState writes only the block and its metadata to the database,
// but does not write any state. This is used to construct competing side forks
// up to the point where they exceed the canonical total difficulty.
func (bc *KeyBlockChain) WriteBlockWithoutState(block *types.KeyBlock, td *big.Int) (err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	// if err := bc.hc.WriteTd(block.Hash(), block.NumberU64(), td); err != nil {
	// 	return err
	// }
	rawdb.WriteKeyBlock(bc.db, block)

	return nil
}

// WriteBlockWithState writes the block and all associated state to the database.
func (bc *KeyBlockChain) WriteBlockWithState(block *types.KeyBlock, receipts []*types.Receipt, state *state.StateDB) (status WriteKeyBlockStatus, err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	// Calculate the total difficulty of the block
	// ptd := bc.GetTd(block.ParentHash(), block.NumberU64()-1)
	// if ptd == nil {
	// 	return NonStatTy, consensus.ErrUnknownAncestor
	// }
	// Make sure no inconsistent state is leaked during insertion
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// currentBlock := bc.CurrentBlock()
	// localTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	// externTd := new(big.Int).Add(block.Difficulty(), ptd)

	// // Irrelevant of the canonical status, write the block itself to the database
	// if err := bc.hc.WriteTd(block.Hash(), block.NumberU64(), externTd); err != nil {
	// 	return NonStatTy, err
	// }
	// Write other block data using a batch.
	batch := bc.db.NewBatch()
	rawdb.WriteBlock(batch, block)

	// root, err := state.Commit(bc.chainConfig.IsEIP158(block.Number()))
	// if err != nil {
	// 	return NonStatTy, err
	// }
	// triedb := bc.stateCache.TrieDB()

	// // If we're running an archive node, always flush
	// if bc.cacheConfig.Disabled {
	// 	if err := triedb.Commit(root, false); err != nil {
	// 		return NonStatTy, err
	// 	}
	// } else {
	// 	// Full but not archive node, do proper garbage collection
	// 	triedb.Reference(root, common.Hash{}) // metadata reference to keep trie alive
	// 	bc.triegc.Push(root, -float32(block.NumberU64()))

	// 	if current := block.NumberU64(); current > triesInMemory {
	// 		// Find the next state trie we need to commit
	// 		header := bc.GetHeaderByNumber(current - triesInMemory)
	// 		chosen := header.Number.Uint64()

	// 		// Only write to disk if we exceeded our memory allowance *and* also have at
	// 		// least a given number of tries gapped.
	// 		var (
	// 			size  = triedb.Size()
	// 			limit = common.StorageSize(bc.cacheConfig.TrieNodeLimit) * 1024 * 1024
	// 		)
	// 		if size > limit || bc.gcproc > bc.cacheConfig.TrieTimeLimit {
	// 			// If we're exceeding limits but haven't reached a large enough memory gap,
	// 			// warn the user that the system is becoming unstable.
	// 			if chosen < keyBlockLastWrite+triesInMemory {
	// 				switch {
	// 				case size >= 2*limit:
	// 					log.Warn("State memory usage too high, committing", "size", size, "limit", limit, "optimum", float64(chosen-keyBlockLastWrite)/triesInMemory)
	// 				case bc.gcproc >= 2*bc.cacheConfig.TrieTimeLimit:
	// 					log.Info("State in memory for too long, committing", "time", bc.gcproc, "allowance", bc.cacheConfig.TrieTimeLimit, "optimum", float64(chosen-keyBlockLastWrite)/triesInMemory)
	// 				}
	// 			}
	// 			// If optimum or critical limits reached, write to disk
	// 			if chosen >= keyBlockLastWrite+triesInMemory || size >= 2*limit || bc.gcproc >= 2*bc.cacheConfig.TrieTimeLimit {
	// 				triedb.Commit(header.Root, true)
	// 				keyBlockLastWrite = chosen
	// 				bc.gcproc = 0
	// 			}
	// 		}
	// 		// Garbage collect anything below our required write retention
	// 		for !bc.triegc.Empty() {
	// 			root, number := bc.triegc.Pop()
	// 			if uint64(-number) > chosen {
	// 				bc.triegc.Push(root, number)
	// 				break
	// 			}
	// 			triedb.Dereference(root.(common.Hash), common.Hash{})
	// 		}
	// 	}
	// }
	// rawdb.WriteReceipts(batch, block.Hash(), block.NumberU64(), receipts)

	// If the total difficulty is higher than our known, add it to the canonical chain
	// Second clause in the if statement reduces the vulnerability to selfish mining.
	// Please refer to http://www.cs.cornell.edu/~ie53/publications/btcProcFC.pdf
	// reorg := externTd.Cmp(localTd) > 0
	// currentBlock = bc.CurrentBlock()
	// if !reorg && externTd.Cmp(localTd) == 0 {
	// 	// Split same-difficulty blocks by number, then at random
	// 	reorg = block.NumberU64() < currentBlock.NumberU64() || (block.NumberU64() == currentBlock.NumberU64() && mrand.Float64() < 0.5)
	// }
	// if reorg {
	// 	// Reorganise the chain if the parent is not the head block
	// 	if block.ParentHash() != currentBlock.Hash() {
	// 		if err := bc.reorg(currentBlock, block); err != nil {
	// 			return NonStatTy, err
	// 		}
	// 	}
	// 	// Write the positional metadata for transaction/receipt lookups and preimages
	// 	rawdb.WriteTxLookupEntries(batch, block)
	// 	rawdb.WritePreimages(batch, block.NumberU64(), state.Preimages())

	// 	status = CanonStatTy
	// } else {
	// 	status = SideStatTy
	// }

	status = CanonStatTy

	if err := batch.Write(); err != nil {
		return NonStatTy, err
	}

	// Set new head.
	if status == CanonStatTy {
		bc.insert(block)
	}
	bc.futureKeyKeyBlocks.Remove(block.Hash())
	return status, nil
}

// InsertChain attempts to insert the given batch of blocks in to the canonical
// chain or, otherwise, create a fork. If an error is returned it will return
// the index number of the failing block as well an error describing what went
// wrong.
//
// After insertion is done, all accumulated events will be fired.
func (bc *KeyBlockChain) InsertChain(chain types.KeyBlocks) (int, error) {
	n, events, logs, err := bc.insertChain(chain)
	bc.PostChainEvents(events, logs)
	return n, err
}

// insertChain will execute the actual chain insertion and event aggregation. The
// only reason this method exists as a separate one is to make locking cleaner
// with deferred statements.
func (bc *KeyBlockChain) insertChain(chain types.KeyBlocks) (int, []interface{}, []*types.Log, error) {
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].NumberU64() != chain[i-1].NumberU64()+1 || chain[i].ParentHash() != chain[i-1].Hash() {
			// Chain broke ancestry, log a messge (programming error) and skip insertion
			log.Error("Non contiguous block insert", "number", chain[i].Number(), "hash", chain[i].Hash(),
				"parent", chain[i].ParentHash(), "prevnumber", chain[i-1].Number(), "prevhash", chain[i-1].Hash())

			return 0, nil, nil, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].NumberU64(),
				chain[i-1].Hash().Bytes()[:4], i, chain[i].NumberU64(), chain[i].Hash().Bytes()[:4], chain[i].ParentHash().Bytes()[:4])
		}
	}
	// Pre-checks passed, start the full block imports
	bc.wg.Add(1)
	defer bc.wg.Done()

	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	// A queued approach to delivering events. This is generally
	// faster than direct delivery and requires much less mutex
	// acquiring.
	var (
		stats         = keyBlockInsertStats{startTime: mclock.Now()}
		events        = make([]interface{}, 0, len(chain))
		lastCanon     *types.KeyBlock
		coalescedLogs []*types.Log
	)
	// Start the parallel header verifier
	headers := make([]*types.KeyBlockHdr, len(chain))
	seals := make([]bool, len(chain))

	for i, block := range chain {
		headers[i] = block.Header()
		seals[i] = true
	}
	abort, results := bc.engine.VerifyHeaders(bc, headers, seals) //TODO
	defer close(abort)

	// Iterate over the blocks and insert when the verifier permits
	for i, block := range chain {
		// If the chain is terminating, stop processing blocks
		if atomic.LoadInt32(&bc.procInterrupt) == 1 {
			log.Debug("Premature abort during blocks processing")
			break
		}
		// If the header is a banned one, straight out abort
		if BadHashes[block.Hash()] {
			bc.reportBlock(block, nil, ErrBlacklistedHash)
			return i, events, coalescedLogs, ErrBlacklistedHash
		}
		// Wait for the block's verification to complete
		bstart := time.Now()

		err := <-results
		if err == nil {
			err = bc.Validator().ValidateBody(block) //TODO
		}
		switch {
		case err == ErrKnownBlock:
			// Block and state both already known. However if the current block is below
			// this number we did a rollback and we should reimport it nonetheless.
			if bc.CurrentBlock().NumberU64() >= block.NumberU64() {
				stats.ignored++
				continue
			}

		case err == consensus.ErrFutureBlock:
			// Allow up to MaxFuture second in the future blocks. If this limit is exceeded
			// the chain is discarded and processed at a later time if given.
			max := big.NewInt(time.Now().Unix() + maxTimeFutureKeyBlocks)
			if block.Time().Cmp(max) > 0 {
				return i, events, coalescedLogs, fmt.Errorf("future block: %v > %v", block.Time(), max)
			}
			bc.futureKeyBlocks.Add(block.Hash(), block)
			stats.queued++
			continue

		case err == consensus.ErrUnknownAncestor && bc.futureKeyBlocks.Contains(block.ParentHash()):
			bc.futureKeyBlocks.Add(block.Hash(), block)
			stats.queued++
			continue

		case err == consensus.ErrPrunedAncestor:
			// Block competing with the canonical chain, store in the db, but don't process
			// until the competitor TD goes above the canonical TD
			// currentBlock := bc.CurrentBlock()
			// localTd := bc.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
			// externTd := new(big.Int).Add(bc.GetTd(block.ParentHash(), block.NumberU64()-1), block.Difficulty())
			// if localTd.Cmp(externTd) > 0 {
			// 	if err = bc.WriteBlockWithoutState(block, externTd); err != nil {
			// 		return i, events, coalescedLogs, err
			// 	}
			// 	continue
			// }
			// // Competitor chain beat canonical, gather all blocks from the common ancestor
			// var winner []*types.KeyBlock

			// parent := bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
			// for !bc.HasState(parent.Root()) {
			// 	winner = append(winner, parent)
			// 	parent = bc.GetBlock(parent.ParentHash(), parent.NumberU64()-1)
			// }
			// for j := 0; j < len(winner)/2; j++ {
			// 	winner[j], winner[len(winner)-1-j] = winner[len(winner)-1-j], winner[j]
			// }
			// // Import all the pruned blocks to make the state available
			// bc.chainmu.Unlock()
			// _, evs, logs, err := bc.insertChain(winner)
			// bc.chainmu.Lock()
			// events, coalescedLogs = evs, logs

			// if err != nil {
			// 	return i, events, coalescedLogs, err
			// }
			if err = bc.WriteBlockWithoutState(block, nil); err != nil {
				return i, events, coalescedLogs, err
			}

		case err != nil:
			bc.reportBlock(block, nil, err)
			return i, events, coalescedLogs, err
		}
		// Create a new statedb using the parent block and report an
		// error if it fails.
		var parent *types.KeyBlock
		if i == 0 {
			parent = bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
		} else {
			parent = chain[i-1]
		}
		// state, err := state.New(parent.Root(), bc.stateCache)
		// if err != nil {
		// 	return i, events, coalescedLogs, err
		// }
		// Process block using the parent state as reference point.
		// receipts, logs, usedGas, err := bc.processor.Process(block, state, bc.vmConfig)
		// if err != nil {
		// 	bc.reportBlock(block, receipts, err)
		// 	return i, events, coalescedLogs, err
		// }
		// Validate the state using the default validator
		// err = bc.Validator().ValidateState(block, parent, state, receipts, usedGas)
		// if err != nil {
		// 	bc.reportBlock(block, receipts, err)
		// 	return i, events, coalescedLogs, err
		// }
		proctime := time.Since(bstart)

		// Write the block to the chain and get the status.
		status, err := bc.WriteBlockWithState(block, nil, nil)
		if err != nil {
			return i, events, coalescedLogs, err
		}
		switch status {
		case CanonStatTy:
			log.Debug("Inserted new block", "number", block.Number(), "hash", block.Hash(), "elapsed", common.PrettyDuration(time.Since(bstart)))

			// coalescedLogs = append(coalescedLogs, logs...)
			blockInsertTimer.UpdateSince(bstart)
			events = append(events, KeyBlockChainEvent{block, block.Hash()})
			lastCanon = block

			// Only count canonical blocks for GC processing time
			bc.gcproc += proctime

		case SideStatTy:
			// log.Debug("Inserted forked block", "number", block.Number(), "hash", block.Hash(), "diff", block.Difficulty(), "elapsed",
			// 	common.PrettyDuration(time.Since(bstart)), "txs", len(block.Transactions()), "gas", block.GasUsed(), "uncles", len(block.Uncles()))

			// blockInsertTimer.UpdateSince(bstart)
			// events = append(events, ChainSideEvent{block})
		}
		stats.processed++
		stats.report(chain, i)
	}
	// Append a single chain head event if we've progressed the chain
	if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
		events = append(events, KeyBlockChainHeadEvent{lastCanon})
	}
	return 0, events, nil, nil
}

// keyBlockInsertStats tracks and reports on block insertion.
type keyBlockInsertStats struct {
	queued, processed, ignored int
	lastIndex                  int
	startTime                  mclock.AbsTime
}

// keyBlockStatsReportLimit is the time limit during import after which we always print
// out progress. This avoids the user wondering what's going on.
const keyBlockStatsReportLimit = 8 * time.Second

// report prints statistics if some number of blocks have been processed
// or more than a few seconds have passed since the last message.
func (st *keyBlockInsertStats) report(chain []*types.KeyBlock, index int, cache common.StorageSize) {
	// Fetch the timings for the batch
	var (
		now     = mclock.Now()
		elapsed = time.Duration(now) - time.Duration(st.startTime)
	)
	// If we're at the last block of the batch or report period reached, log
	if index == len(chain)-1 || elapsed >= keyBlockStatsReportLimit {
		end := chain[index]
		context := []interface{}{
			"blocks", st.processed,
			"elapsed", common.PrettyDuration(elapsed),
			"number", end.Number(), "hash", end.Hash(), "cache", cache,
		}
		if st.queued > 0 {
			context = append(context, []interface{}{"queued", st.queued}...)
		}
		if st.ignored > 0 {
			context = append(context, []interface{}{"ignored", st.ignored}...)
		}
		log.Info("Imported new chain segment", context...)

		*st = keyBlockInsertStats{startTime: now, lastIndex: index + 1}
	}
}

// reorgs takes two blocks, an old chain and a new chain and will reconstruct the blocks and inserts them
// to be part of the new canonical chain and accumulates potential missing transactions and post an
// event about them
func (bc *KeyBlockChain) reorg(oldBlock, newBlock *types.KeyBlock) error {
	var (
		newChain    types.KeyBlocks
		oldChain    types.KeyBlocks
		commonBlock *types.KeyBlock
	)

	// first reduce whoever is higher bound
	if oldBlock.NumberU64() > newBlock.NumberU64() {
		// reduce old chain
		for ; oldBlock != nil && oldBlock.NumberU64() != newBlock.NumberU64(); oldBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1) {
			oldChain = append(oldChain, oldBlock)
		}
	} else {
		// reduce new chain and append new chain blocks for inserting later on
		for ; newBlock != nil && newBlock.NumberU64() != oldBlock.NumberU64(); newBlock = bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1) {
			newChain = append(newChain, newBlock)
		}
	}
	if oldBlock == nil {
		return fmt.Errorf("Invalid old chain")
	}
	if newBlock == nil {
		return fmt.Errorf("Invalid new chain")
	}

	for {
		if oldBlock.Hash() == newBlock.Hash() {
			commonBlock = oldBlock
			break
		}

		oldChain = append(oldChain, oldBlock)
		newChain = append(newChain, newBlock)

		oldBlock, newBlock = bc.GetBlock(oldBlock.ParentHash(), oldBlock.NumberU64()-1), bc.GetBlock(newBlock.ParentHash(), newBlock.NumberU64()-1)
		if oldBlock == nil {
			return fmt.Errorf("Invalid old chain")
		}
		if newBlock == nil {
			return fmt.Errorf("Invalid new chain")
		}
	}
	// Ensure the user sees large reorgs
	if len(oldChain) > 0 && len(newChain) > 0 {
		logFn := log.Debug
		if len(oldChain) > 63 {
			logFn = log.Warn
		}
		logFn("Chain split detected", "number", commonBlock.Number(), "hash", commonBlock.Hash(),
			"drop", len(oldChain), "dropfrom", oldChain[0].Hash(), "add", len(newChain), "addfrom", newChain[0].Hash())
	} else {
		log.Error("Impossible reorg, please file an issue", "oldnum", oldBlock.Number(), "oldhash", oldBlock.Hash(), "newnum", newBlock.Number(), "newhash", newBlock.Hash())
	}
	// Insert the new chain, taking care of the proper incremental order
	for i := len(newChain) - 1; i >= 0; i-- {
		// insert the block in the canonical way, re-writing history
		bc.insert(newChain[i])
	}
	if len(oldChain) > 0 {
		go func() {
			for _, block := range oldChain {
				bc.keyBlockChainSideFeed.Send(KeyBlockChainSideEvent{Block: block})
			}
		}()
	}

	return nil
}

// PostChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
// TODO: Should not expose PostChainEvents. The chain events should be posted in WriteBlock.
func (bc *KeyBlockChain) PostChainEvents(events []interface{}, logs []*types.Log) {
	// post event logs for further processing
	if logs != nil {
		bc.logsFeed.Send(logs)
	}
	for _, event := range events {
		switch ev := event.(type) {
		case KeyBlockChainEvent:
			bc.keyBlockChainFeed.Send(ev)

		case KeyBlockChainHeadEvent:
			bc.keyBlockChainHeadFeed.Send(ev)

		case KeyBlockChainSideEvent:
			bc.keyBlockChainSideFeed.Send(ev)
		}
	}
}

func (bc *KeyBlockChain) update() {
	futureTimer := time.NewTicker(5 * time.Second)
	defer futureTimer.Stop()
	for {
		select {
		case <-futureTimer.C:
			bc.procFutureBlocks()
		case <-bc.quit:
			return
		}
	}
}

// BadKeyBlockArgs represents the entries in the list returned when bad blocks are queried.
type BadKeyBlockArgs struct {
	Hash   common.Hash        `json:"hash"`
	Header *types.KeyBlockHdr `json:"header"`
}

// BadBlocks returns a list of the last 'bad blocks' that the client has seen on the network
func (bc *KeyBlockChain) BadBlocks() ([]BadKeyBlockArgs, error) {
	headers := make([]BadKeyBlockArgs, 0, bc.badKeyBlocks.Len())
	for _, hash := range bc.badKeyBlocks.Keys() {
		if hdr, exist := bc.badKeyBlocks.Peek(hash); exist {
			header := hdr.(*types.KeyBlockHdr)
			headers = append(headers, BadKeyBlockArgs{header.Hash(), header})
		}
	}
	return headers, nil
}

// addBadBlock adds a bad block to the bad-block LRU cache
func (bc *KeyBlockChain) addBadBlock(block *types.KeyBlock) {
	bc.badKeyBlocks.Add(block.Header().Hash(), block.Header())
}

// reportBlock logs a bad block error.
func (bc *KeyBlockChain) reportBlock(block *types.KeyBlock, receipts types.Receipts, err error) {
	bc.addBadBlock(block)

	// var receiptString string
	// for _, receipt := range receipts {
	// 	receiptString += fmt.Sprintf("\t%v\n", receipt)
	// }
	log.Error(fmt.Sprintf(`
########## BAD BLOCK #########
Chain config: %v

Number: %v
Hash: 0x%x

Error: %v
##############################
`, bc.chainConfig, block.Number(), block.Hash(), err))
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verify nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
func (bc *KeyBlockChain) InsertHeaderChain(chain []*types.KeyBlockHdr, checkFreq int) (int, error) {
	start := time.Now()
	if i, err := bc.hc.ValidateHeaderChain(chain, checkFreq); err != nil {
		return i, err
	}

	// Make sure only one thread manipulates the chain at once
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	bc.wg.Add(1)
	defer bc.wg.Done()

	whFunc := func(header *types.KeyBlockHdr) error {
		bc.mu.Lock()
		defer bc.mu.Unlock()

		_, err := bc.hc.WriteHeader(header)
		return err
	}

	return bc.hc.InsertHeaderChain(chain, whFunc, start)
}

// writeHeader writes a header into the local chain, given that its parent is
// already known. If the total difficulty of the newly inserted header becomes
// greater than the current known TD, the canonical chain is re-routed.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (bc *KeyBlockChain) writeHeader(header *types.KeyBlockHdr) error {
	bc.wg.Add(1)
	defer bc.wg.Done()

	bc.mu.Lock()
	defer bc.mu.Unlock()

	_, err := bc.hc.WriteHeader(header)
	return err
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (bc *KeyBlockChain) CurrentHeader() *types.KeyBlockHdr {
	return bc.hc.CurrentHeader()
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (bc *KeyBlockChain) GetHeader(hash common.Hash, number uint64) *types.KeyBlockHdr {
	return bc.hc.GetHeader(hash, number)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (bc *KeyBlockChain) GetHeaderByHash(hash common.Hash) *types.KeyBlockHdr {
	return bc.hc.GetHeaderByHash(hash)
}

// HasHeader checks if a block header is present in the database or not, caching
// it if present.
func (bc *KeyBlockChain) HasHeader(hash common.Hash, number uint64) bool {
	return bc.hc.HasHeader(hash, number)
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (bc *KeyBlockChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	return bc.hc.GetBlockHashesFromHash(hash, max)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (bc *KeyBlockChain) GetHeaderByNumber(number uint64) *types.KeyBlockHdr {
	return bc.hc.GetHeaderByNumber(number)
}

// Config retrieves the blockchain's chain configuration.
func (bc *KeyBlockChain) Config() *params.ChainConfig { return bc.chainConfig } //TODO

// Engine retrieves the blockchain's consensus engine.
func (bc *KeyBlockChain) Engine() consensus.Engine { return bc.engine }

// SubscribeChainEvent registers a subscription of ChainEvent.
func (bc *KeyBlockChain) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	return bc.scope.Track(bc.keyBlockChainFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (bc *KeyBlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.keyBlockChainHeadFeed.Subscribe(ch))
}

// SubscribeChainSideEvent registers a subscription of ChainSideEvent.
func (bc *KeyBlockChain) SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription {
	return bc.scope.Track(bc.keyBlockChainSideFeed.Subscribe(ch))
}
