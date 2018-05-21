package blockchain

const (
	AddrLen   = 20
	PubKeyLen = 32
	PrvKeyLen = 64
	HashLen   = 32
	SigLen    = 64
	AggrLen   = 64
)

type (
	Address   [AddrLen]byte
	PubKey    [PubKeyLen]byte
	PrvKey    [PrvKeyLen]byte
	Hash      [HashLen]byte
	Signature [SigLen]byte
	AggrAddr  [AggrLen]byte
)
