package core

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"sync/atomic"
	"time"

	"github.com/cypherium_private/go-cypherium/common"
	"github.com/cypherium_private/go-cypherium/consensus"
	"github.com/cypherium_private/go-cypherium/core/rawdb"
	"github.com/cypherium_private/go-cypherium/core/types"
	"github.com/cypherium_private/go-cypherium/ethdb"
	"github.com/cypherium_private/go-cypherium/log"
	"github.com/cypherium_private/go-cypherium/params"
	"github.com/hashicorp/golang-lru"
)

const (
	keyBlockHeaderCacheLimit = 512
	keyBlockTdCacheLimit     = 1024
	keyBlockNumberCacheLimit = 2048
)

// KeyBlockHeaderChain implements the basic block header chain logic that is shared by
// core.KeyBlockChain and light.KeyLightChain. It is not usable in itself, only as
// a part of either structure.
// It is not thread safe either, the encapsulating chain structures should do
// the necessary mutex locking/unlocking.
type KeyBlockHeaderChain struct {
	config *params.KeyBlockChainConfig //TODO

	chainDb       ethdb.Database
	genesisHeader *types.KeyBlockHdr

	currentHeader     atomic.Value // Current head of the header chain (may be above the block chain!)
	currentHeaderHash common.Hash  // Hash of the current head of the header chain (prevent recomputing all the time)

	headerCache *lru.Cache // Cache for the most recent block headers
	tdCache     *lru.Cache // Cache for the most recent block total difficulties
	numberCache *lru.Cache // Cache for the most recent block numbers

	procInterrupt func() bool

	rand   *mrand.Rand
	engine consensus.Engine
}

// NewKeyBlockHeaderChain creates a new KeyBlockHeaderChain structure.
//  getValidator should return the parent's validator
//  procInterrupt points to the parent's interrupt semaphore
//  wg points to the parent's shutdown wait group
func NewKeyBlockHeaderChain(chainDb ethdb.Database, config *params.KeyBlockChainConfig, engine consensus.Engine, procInterrupt func() bool) (*KeyBlockHeaderChain, error) {
	headerCache, _ := lru.New(keyBlockHeaderCacheLimit)
	tdCache, _ := lru.New(keyBlockTdCacheLimit)
	numberCache, _ := lru.New(keyBlockNumberCacheLimit)

	// Seed a fast but crypto originating random generator
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	hc := &KeyBlockHeaderChain{
		config:        config,
		chainDb:       chainDb,
		headerCache:   headerCache,
		tdCache:       tdCache,
		numberCache:   numberCache,
		procInterrupt: procInterrupt,
		rand:          mrand.New(mrand.NewSource(seed.Int64())),
		engine:        engine,
	}

	hc.genesisHeader = hc.GetHeaderByNumber(0)
	if hc.genesisHeader == nil {
		return nil, ErrNoGenesis
	}

	hc.currentHeader.Store(hc.genesisHeader)
	if head := rawdb.ReadHeadKeyBlockHeaderHash(chainDb); head != (common.Hash{}) {
		if chead := hc.GetHeaderByHash(head); chead != nil {
			hc.currentHeader.Store(chead)
		}
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	return hc, nil
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the cache or database
func (hc *KeyBlockHeaderChain) GetBlockNumber(hash common.Hash) *uint64 {
	if cached, ok := hc.numberCache.Get(hash); ok {
		number := cached.(uint64)
		return &number
	}
	number := rawdb.ReadKeyBlockHeaderNumber(hc.chainDb, hash)
	if number != nil {
		hc.numberCache.Add(hash, *number)
	}
	return number
}

// WriteHeader writes a header into the local chain, given that its parent is
// already known. If the total difficulty of the newly inserted header becomes
// greater than the current known TD, the canonical chain is re-routed.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (hc *KeyBlockHeaderChain) WriteHeader(header *types.KeyBlockHdr) (status WriteStatus, err error) { //TODO WriteStatus
	// Cache some values to prevent constant recalculation
	var (
		hash   = header.Hash()
		number = types.ProtobufFormartToBigInt(header.Number).Uint64()
	)
	// // Calculate the total difficulty of the header
	// ptd := hc.GetTd(header.PKblkHash, number-1)
	// if ptd == nil {
	// 	return NonStatTy, consensus.ErrUnknownAncestor
	// }
	// localTd := hc.GetTd(hc.currentHeaderHash, types.ProtobufFormartToBigInt(hc.CurrentHeader().Number).Uint64())
	// externTd := new(big.Int).Add(header.Difficulty, ptd) //TODO header.Difficulty  委员会由多个Difficulty

	// // Irrelevant of the canonical status, write the td and header to the database
	// if err := hc.WriteTd(hash, number, externTd); err != nil {
	// 	log.Crit("Failed to write header total difficulty", "err", err)
	// }
	rawdb.WriteKeyBlockHeader(hc.chainDb, header)

	// If the total difficulty is higher than our known, add it to the canonical chain
	// Second clause in the if statement reduces the vulnerability to selfish mining.
	// Please refer to http://www.cs.cornell.edu/~ie53/publications/btcProcFC.pdf
	// if externTd.Cmp(localTd) > 0 || (externTd.Cmp(localTd) == 0 && mrand.Float64() < 0.5) {
	// 	// Delete any canonical number assignments above the new head
	// 	for i := number + 1; ; i++ {
	// 		hash := rawdb.ReadCanonicalHash(hc.chainDb, i)
	// 		if hash == (common.Hash{}) {
	// 			break
	// 		}
	// 		rawdb.DeleteCanonicalHash(hc.chainDb, i)
	// 	}
	// 	// Overwrite any stale canonical number assignments
	// 	var (
	// 		headHash   = header.ParentHash
	// 		headNumber = header.Number.Uint64() - 1
	// 		headHeader = hc.GetHeader(headHash, headNumber)
	// 	)
	// 	for rawdb.ReadCanonicalHash(hc.chainDb, headNumber) != headHash {
	// 		rawdb.WriteCanonicalHash(hc.chainDb, headHash, headNumber)

	// 		headHash = headHeader.ParentHash
	// 		headNumber = headHeader.Number.Uint64() - 1
	// 		headHeader = hc.GetHeader(headHash, headNumber)
	// 	}
	// Extend the canonical chain with the new header
	rawdb.WriteKeyBlockHash(hc.chainDb, hash, number)
	rawdb.WriteHeadKeyBlockHeaderHash(hc.chainDb, hash)

	// 	hc.currentHeaderHash = hash
	// 	hc.currentHeader.Store(types.CopyHeader(header))

	// 	status = CanonStatTy
	// } else {
	// 	status = SideStatTy
	// }

	status = CanonStatTy

	hc.headerCache.Add(hash, header)
	hc.numberCache.Add(hash, number)

	return
}

// KeyBlockWhCallback is a callback function for inserting individual headers.
// A callback is used for two reasons: first, in a LightChain, status should be
// processed and light chain events sent, while in a BlockChain this is not
// necessary since chain events are sent after inserting blocks. Second, the
// header writes should be protected by the parent chain mutex individually.
type KeyBlockWhCallback func(*types.KeyBlockHdr) error

func (hc *KeyBlockHeaderChain) ValidateHeaderChain(chain []*types.KeyBlockHdr, checkFreq int) (int, error) {
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if types.ProtobufFormartToBigInt(chain[i].Number).Uint64() != types.ProtobufFormartToBigInt(chain[i-1].Number).Uint64()+1 || chain[i].PKblkHash != chain[i-1].Hash() {
			// Chain broke ancestry, log a messge (programming error) and skip insertion
			log.Error("Non contiguous header insert", "number", chain[i].Number.Abs, "hash", chain[i].Hash(),
				"parent", chain[i].PKblkHash, "prevnumber", chain[i-1].Number.Abs, "prevhash", chain[i-1].Hash())

			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].Number.Abs,
				chain[i-1].Hash().Bytes()[:4], i, chain[i].Number.Abs, chain[i].Hash().Bytes()[:4], chain[i].ParentHash[:4])
		}
	}

	// Generate the list of seal verification requests, and start the parallel verifier
	seals := make([]bool, len(chain))
	for i := 0; i < len(seals)/checkFreq; i++ {
		index := i*checkFreq + hc.rand.Intn(checkFreq)
		if index >= len(seals) {
			index = len(seals) - 1
		}
		seals[index] = true
	}
	seals[len(seals)-1] = true // Last should always be verified to avoid junk

	abort, results := hc.engine.VerifyHeaders(hc, chain, seals) //TODO 验证规则
	defer close(abort)

	// Iterate over the headers and ensure they all check out
	for i, header := range chain {
		// If the chain is terminating, stop processing blocks
		if hc.procInterrupt() {
			log.Debug("Premature abort during headers verification")
			return 0, errors.New("aborted")
		}
		// If the header is a banned one, straight out abort
		if BadHashes[header.Hash()] { //TODO txblock不需要BadHashes吧？
			return i, ErrBlacklistedHash
		}
		// Otherwise wait for headers checks and ensure they pass
		if err := <-results; err != nil {
			return i, err
		}
	}

	return 0, nil
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verfy nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
func (hc *KeyBlockHeaderChain) InsertHeaderChain(chain []*types.KeyBlockHdr, writeHeader KeyBlockWhCallback, start time.Time) (int, error) {
	// Collect some import statistics to report on
	stats := struct{ processed, ignored int }{}
	// All headers passed verification, import them into the database
	for i, header := range chain {
		// Short circuit insertion if shutting down
		if hc.procInterrupt() {
			log.Debug("Premature abort during headers import")
			return i, errors.New("aborted")
		}
		// If the header's already known, skip it, otherwise store
		if hc.HasHeader(header.Hash(), types.ProtobufFormartToBigInt(header.Number).Uint64()) {
			stats.ignored++
			continue
		}
		if err := writeHeader(header); err != nil {
			return i, err
		}
		stats.processed++
	}
	// Report some public statistics so the user has a clue what's going on
	last := chain[len(chain)-1]
	log.Info("Imported new block headers", "count", stats.processed, "elapsed", common.PrettyDuration(time.Since(start)),
		"number", last.Number, "hash", last.Hash(), "ignored", stats.ignored)

	return 0, nil
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (hc *KeyBlockHeaderChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	// Get the origin header from which to fetch
	header := hc.GetHeaderByHash(hash)
	if header == nil {
		return nil
	}
	// Iterate the headers until enough is collected or the genesis reached
	chain := make([]common.Hash, 0, max)
	for i := uint64(0); i < max; i++ {
		next := header.PKblkHash
		if header = hc.GetHeader(next, types.ProtobufFormartToBigInt(header.Number).Uint64()-1); header == nil {
			break
		}
		chain = append(chain, next)
		if types.ProtobufFormartToBigInt(header.Number).Sign() == 0 {
			break
		}
	}
	return chain
}

// GetTd retrieves a block's total difficulty in the canonical chain from the
// database by hash and number, caching it if found.
func (hc *KeyBlockHeaderChain) GetTd(hash common.Hash, number uint64) *big.Int {
	// // Short circuit if the td's already in the cache, retrieve otherwise
	// if cached, ok := hc.tdCache.Get(hash); ok {
	// 	return cached.(*big.Int)
	// }
	// td := rawdb.ReadTd(hc.chainDb, hash, number)
	// if td == nil {
	// 	return nil
	// }
	// // Cache the found body for next time and return
	// hc.tdCache.Add(hash, td)
	// return td
	return new(big.Int).Set(0)
}

// GetTdByHash retrieves a block's total difficulty in the canonical chain from the
// database by hash, caching it if found.
func (hc *KeyBlockHeaderChain) GetTdByHash(hash common.Hash) *big.Int {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetTd(hash, *number)
}

// WriteTd stores a block's total difficulty into the database, also caching it
// along the way.
func (hc *KeyBlockHeaderChain) WriteTd(hash common.Hash, number uint64, td *big.Int) error {
	// rawdb.WriteKeyBlockTd(hc.chainDb, hash, number, td)
	// hc.tdCache.Add(hash, new(big.Int).Set(td))
	return nil
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (hc *KeyBlockHeaderChain) GetHeader(hash common.Hash, number uint64) *types.KeyBlockHdr {
	// Short circuit if the header's already in the cache, retrieve otherwise
	if header, ok := hc.headerCache.Get(hash); ok {
		return header.(*types.KeyBlockHdr)
	}
	header := rawdb.ReadKeyBlockHeader(hc.chainDb, hash, number)
	if header == nil {
		return nil
	}
	// Cache the found header for next time and return
	hc.headerCache.Add(hash, header)
	return header
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (hc *KeyBlockHeaderChain) GetHeaderByHash(hash common.Hash) *types.KeyBlockHdr {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetHeader(hash, *number)
}

// HasHeader checks if a block header is present in the database or not.
func (hc *KeyBlockHeaderChain) HasHeader(hash common.Hash, number uint64) bool {
	if hc.numberCache.Contains(hash) || hc.headerCache.Contains(hash) {
		return true
	}
	return rawdb.KeyBlockHasHeader(hc.chainDb, hash, number)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (hc *KeyBlockHeaderChain) GetHeaderByNumber(number uint64) *types.KeyBlockHdr {
	hash := rawdb.ReadKeyBlcokHash(hc.chainDb, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return hc.GetHeader(hash, number)
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the KeyBlockHeaderChain's internal cache.
func (hc *KeyBlockHeaderChain) CurrentHeader() *types.KeyBlockHdr {
	return hc.currentHeader.Load().(*types.KeyBlockHdr)
}

// SetCurrentHeader sets the current head header of the canonical chain.
func (hc *KeyBlockHeaderChain) SetCurrentHeader(head *types.KeyBlockHdr) {
	rawdb.WriteHeadKeyBlockHeaderHash(hc.chainDb, head.Hash())

	hc.currentHeader.Store(head)
	hc.currentHeaderHash = head.Hash()
}

// DeleteKeyBlockCallback is a callback function that is called by SetHead before
// each header is deleted.
type DeleteKeyBlockCallback func(common.Hash, uint64)

// SetHead rewinds the local chain to a new head. Everything above the new head
// will be deleted and the new one set.
func (hc *KeyBlockHeaderChain) SetHead(head uint64, delFn DeleteKeyBlockCallback) {
	height := uint64(0)

	if hdr := hc.CurrentHeader(); hdr != nil {
		height = types.ProtobufFormartToBigInt(hdr.Number).Uint64()
	}

	for hdr := hc.CurrentHeader(); hdr != nil && types.ProtobufFormartToBigInt(hdr.Number).Uint64() > head; hdr = hc.CurrentHeader() {
		hash := hdr.Hash()
		num := types.ProtobufFormartToBigInt(hdr.Number).Uint64()
		if delFn != nil {
			delFn(hash, num)
		}
		rawdb.DeleteKeyBlockHeader(hc.chainDb, hash, num)
		rawdb.DeleteKeyBlockTd(hc.chainDb, hash, num)

		hc.currentHeader.Store(hc.GetHeader(hdr.PKblkHash, types.ProtobufFormartToBigInt(hdr.Number).Uint64()-1))
	}
	// Roll back the canonical chain numbering
	for i := height; i > head; i-- {
		rawdb.DeleteKeyBlockHash(hc.chainDb, i)
	}
	// Clear out any stale content from the caches
	hc.headerCache.Purge()
	hc.tdCache.Purge()
	hc.numberCache.Purge()

	if hc.CurrentHeader() == nil {
		hc.currentHeader.Store(hc.genesisHeader)
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	rawdb.WriteHeadKeyBlockHeaderHash(hc.chainDb, hc.currentHeaderHash)
}

// SetGenesis sets a new genesis block header for the chain
func (hc *KeyBlockHeaderChain) SetGenesis(head *types.KeyBlockHdr) {
	hc.genesisHeader = head
}

// Config retrieves the header chain's chain configuration.
func (hc *KeyBlockHeaderChain) Config() *params.ChainConfig { return hc.config }

// Engine retrieves the header chain's consensus engine.
func (hc *KeyBlockHeaderChain) Engine() consensus.Engine { return hc.engine }

// GetBlock implements consensus.ChainReader, and returns nil for every input as
// a header chain does not have blocks available for retrieval.
func (hc *KeyBlockHeaderChain) GetBlock(hash common.Hash, number uint64) *types.KeyBlock {
	return nil
}
