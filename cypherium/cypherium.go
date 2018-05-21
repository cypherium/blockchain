package cypherium

/*
The `NewProtocol` method is used to define the protocol and to register
the handlers that will be called if a certain type of message is received.
The handlers will be treated according to their signature.

The protocol-file defines the actions that the protocol needs to do in each
step. The root-node will call the `Start`-method of the protocol. Each
node will only use the `Handle`-methods, and not call `Start` again.
*/

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/cypherium_private/cvm"
	"github.com/cypherium_private/mvp/bftcosi"
	"github.com/cypherium_private/mvp/blockchain"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/golang/protobuf/proto"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"golang.org/x/crypto/ed25519"
)

func init() {
	network.RegisterMessage(Announce{})
	network.RegisterMessage(Reply{})
	onet.GlobalProtocolRegister(CypheriumProtocolName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return NewProtocol(n)
	})
	// Register test protocol using BFTCoSi
	onet.GlobalProtocolRegister(BFTCoSiProtocolName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, Verify)
	})
}

// FinalSignature holds the message Msg and its signature
type FinalSignature struct {
	Msg []byte
	Sig []byte
}

// TemplateProtocol holds the state of a given protocol.
//
// For this example, it defines a channel that will receive the number
// of children. Only the root-node will write to the channel.
type TemplateProtocol struct {
	// the node we are represented-in
	*onet.TreeNodeInstance
	// the suite we use
	suite network.Suite //suites.Suite
	// aggregated public key of the peers
	aggregatedPublic kyber.Point

	//Refer to byzcoinx
	// Msg is the message that will be signed by cosigners
	Msg []byte
	// Data is used for verification only, not signed
	Data []byte
	// FinalSignature is output of the protocol, for the caller to read
	FinalSignatureChan chan FinalSignature
	// CreateProtocol stores a function pointer used to create the bftcosi
	// protocol
	CreateProtocol bftcosi.CreateProtocolFunction
	// prepCosiProtoName is the ftcosi protocol name for the prepare phase
	prepCosiProtoName string
	// commitCosiProtoName is the ftcosi protocol name for the commit phase
	commitCosiProtoName string
	// prepSigChan is the channel for reading the prepare phase signature
	prepSigChan chan []byte
	//Refer to byzcoinx end

	// channel to notify when we are done
	done chan bool
	// channel used to wait for the verification of the block
	verifyBlockChan chan bool

	//  block to pass up between the two rounds (prepare + commits)
	tempBlock *blockchain.TxBlock
	// transactions is the slice of transactions that contains transactions
	// coming from clients
	transactions []blockchain.STransaction
	// last block computed
	lastBlock string
	// last key block computed
	lastKeyBlock string

	// onDoneCallback is the callback that will be called at the end of the
	// protocol when all nodes have finished. Either at the end of the response
	// phase of the commit round or at the end of a view change.
	onDoneCallback func()

	//Refer to byzcoinx
	// Timeout is passed down to the ftcosi protocol and used for waiting
	// for some of its messages.
	Timeout time.Duration
	//Refer to byzcoinx end

	// rootTimeout is the timeout given to the root. It will be passed down the
	// tree so every nodes knows how much time to wait. This root is a very nice
	// malicious node.
	rootTimeout uint64
	timeoutChan chan uint64
	// onTimeoutCallback is the function that will be called if a timeout
	// occurs.
	onTimeoutCallback func()
	// function to let callers of the protocol (or the server) add functionality
	// to certain parts of the protocol; mainly used in simulation to do
	// measurements. Hence functions will not be called in go routines

	// root fails:
	rootFailMode uint
	// view change setup and measurement
	viewchangeChan chan struct {
		*onet.TreeNode
		viewChange
	}
	// bool set to true when the final signature is produced
	doneSigning chan bool
	// // lock associated
	// doneLock sync.Mutex

	// threshold for how much view change acceptance we need
	// basically n - threshold
	viewChangeThreshold int
	// how many view change request have we received
	vcCounter int
	// done processing is used to stop the processing of the channels
	doneProcessing chan bool

	// // finale signature that this ByzCoin round has produced
	// finalSignature *BlockSignature
	pbftcosi *bftcosi.ProtocolBFTCoSi
	//can use for test log
	ChildCount chan int

	Res chan int
}

// Check that *TemplateProtocol implements onet.ProtocolInstance
var _ onet.ProtocolInstance = (*TemplateProtocol)(nil)

// NewProtocol creates and initialises a cypherium protocol.
func NewProtocol(n *onet.TreeNodeInstance) (*TemplateProtocol, error) {
	log.Lvl5(n.Name(), "NewProtocol creates and initialises a cypherium protocol")
	t := &TemplateProtocol{
		TreeNodeInstance: n,
		// we do not have Msg to make the protocol fail if it's not set
		FinalSignatureChan: make(chan FinalSignature, 1),
		Data:               make([]byte, 0),
		// prepCosiProtoName:   prepCosiProtoName,
		// commitCosiProtoName: commitCosiProtoName,
		prepSigChan: make(chan []byte, 0),
		// aggregatedPublic:    n.ServerIdentity.Public,
		aggregatedPublic:    n.Roster().Aggregate,
		done:                make(chan bool),
		suite:               n.Suite(),
		verifyBlockChan:     make(chan bool),
		doneProcessing:      make(chan bool, 2),
		doneSigning:         make(chan bool, 1),
		timeoutChan:         make(chan uint64, 1),
		viewChangeThreshold: int(math.Ceil(float64(len(n.Tree().List())) * 2.0 / 3.0)),

		ChildCount: make(chan int),
		Res:        make(chan int),
	}

	for _, handler := range []interface{}{t.HandleAnnounce, t.HandleReply} {
		if err := t.RegisterHandler(handler); err != nil {
			return nil, errors.New("couldn't register handler: " + err.Error())
		}
	}
	if err := n.RegisterChannel(&t.viewchangeChan); err != nil {
		return t, err
	}
	//n.OnDoneCallback(t.nodeDone)
	return t, nil
}

// NewCypheriumRootProtocol returns a new Cypherium struct with the block to sign
// that will be sent to all others nodes
func NewCypheriumRootProtocol(p *TemplateProtocol, transactions []blockchain.STransaction, timeOutMs uint64, failMode uint) (*TemplateProtocol, error) {

	// p, err := NewProtocol(n)
	// if err != nil {
	// 	return nil, err
	// }

	tempBlock, err := GetBlock(transactions, p.lastBlock, p.lastKeyBlock)
	p.tempBlock = tempBlock
	p.rootFailMode = failMode
	p.rootTimeout = timeOutMs
	return p, err
}

// GlobalInitBFTCoSiProtocol creates and registers the protocols required to run
// BFTCoSi globally.
func GlobalInitBFTCoSiProtocol(protoName string, verify bftcosi.VerificationFunction) {
	log.Lvl1("GlobalInitBFTCoSiProtocol")
	// Register test protocol using BFTCoSi
	onet.GlobalProtocolRegister(protoName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, verify)
	})
}

//counters Maybe not used
var counters = &Counters{}

// Verify function that returns true if the length of the data is 1.
func Verify(m []byte, d []byte) bool {

	txb := new(blockchain.TxBlock)
	if err := proto.Unmarshal(m, txb); err != nil {
		log.Error("Verify: %s\n", err.Error())
	}

	op := opt.Options{ErrorIfMissing: true, ReadOnly: true}
	StateDB, err := leveldb.OpenFile("../app/mock_state_db", &op)
	defer StateDB.Close()
	if err != nil {
		log.Errorf("Verify: %s\n", err.Error())
	}

	//fmt.Printf("Verify block: %+v\n", *txb)
	stxs := txb.GetTbd().GetStx()
	for i := 0; i < len(stxs); i++ {
		sig := stxs[i].GetSenderSig()
		tx := stxs[i].GetTx()
		msg := []byte(tx.String())
		pk := tx.GetSenderKey()
		/* Verify the signature of the STransaction */
		ok := ed25519.Verify(pk, msg, sig)
		if ok == false {
			//fmt.Printf("STx %d not valid\n", i)
			log.Errorf("STx %d not valid\n", i)
		} else {
			log.Errorf("Verify: Signature verified\n", i)
		}
		/* Verify the Transaction */
		recipient := tx.GetRecipient()
		accMsh, err := StateDB.Get(recipient, nil)
		if err != nil {
			//TODO Create account
			fmt.Printf("Verify: %s\n", err.Error())
		}
		account := new(blockchain.Account)
		err = proto.Unmarshal(accMsh, account)
		if err != nil {
			log.Errorf("Verify: %d\n", err.Error())
		}
		if account.Type == 1 {
			log.Errorf("Verify: balance manipulation not implemented\n")
		} else {
			contract := cvm.Contract{
				Caller:   nil,
				Self:     account.GetAddress(),
				Code:     account.GetCodeHash(),
				Input:    tx.GetData(),
				Storage:  account.GetStgRoot(),
				GasLimit: tx.GetAvailGas(),
				GasPrice: new(big.Int).SetBytes(tx.GetGasPrice()),
				Balance:  new(big.Int).SetBytes(account.GetBalance()),
			}
			fmt.Printf("Verify contract: %+v\n", contract)
			cf := cvm.NewContractFrame(&contract, StateDB)
			if err := cf.Execute(); err != nil {
				log.Errorf("VM: %s\n", err.Error())
			}
		}
		//fmt.Printf("Verify: Account type: %d, balance: %d\n", account.Type, account.Balance)
	}
	return true
}

// VerifyBlock is a simulation of a real verification block algorithm
func VerifyBlock(block *blockchain.TxBlock, lastBlock, lastKeyBlock string, done chan bool) {
	//We measure the average block verification delays is 174ms for an average
	//block of 500kB.
	//To simulate the verification cost of bigger blocks we multiply 174ms
	//times the size/500*1024
	b, _ := json.Marshal(block)
	s := len(b)
	var n time.Duration
	n = time.Duration(s / (500 * 1024))
	time.Sleep(150 * time.Millisecond * n) //verification of 174ms per 500KB simulated
	verified := true
	// verification of the header
	//verified := block.Header.Parent == lastBlock && block.Header.ParentKey == lastKeyBlock
	//verified = verified && block.Header.MerkleRoot == blockchain.HashRootTransactions(block.TransactionList)
	//verified = verified && block.HeaderHash == blockchain.HashHeader(block.Header)
	// notify it
	log.Lvl3("Verification of the block done =", verified)
	done <- verified
}

// Start sends the Announce-message to all children
func (p *TemplateProtocol) Start() error {

	log.Lvl3("Starting TemplateProtocol")

	if !p.IsRoot() {
		return fmt.Errorf("non-root should not start this protocol")
	}

	// Register the function generating the protocol instance
	log.Lvl3(p.Tree().Dump())
	pbftcosi, _ := p.CreateProtocol(BFTCoSiProtocolName, p.Tree())

	p.pbftcosi = pbftcosi.(*bftcosi.ProtocolBFTCoSi)
	//marshalled, err := json.Marshal(p.tempBlock)
	marshalled, err := proto.Marshal(p.tempBlock)
	if err != nil {
		return err
	}
	p.pbftcosi.Msg = marshalled
	// p.pbftcosi.Msg = p.Msg //uncomment for unit test

	p.pbftcosi.RegisterOnDone(func() {
		log.Lvl1("RegisterOnDone protocol done")
		p.nodeDone()
		p.doneSigning <- true
	})

	go p.pbftcosi.Start()
	log.Lvl1("Launched protocol")
	// are we done yet?
	wait := time.Second * 60
	select {
	case <-p.doneSigning:
		log.Lvl2("Protocol done")
		sig := p.pbftcosi.Signature()
		err := sig.Verify(p.pbftcosi.Suite(), p.pbftcosi.Roster().Publics())
		if err != nil {
			log.Errorf("%s Verification of the signature refused: %s - %+v", p.pbftcosi.Name(), err.Error(), sig.Sig)
			p.Res <- PBFT_VERIFICATION_REFUSED
		}
		if err == nil {
			log.Lvl1("%s: Verification succeed", p.pbftcosi.Name(), sig)
			p.Res <- PBFT_OK
		}
	case <-time.After(wait):
		log.Lvl1("Going to break because of timeout", "Waited ", wait.String(), " for BFTCoSi to finish ...")
		p.Res <- PBFT_TIMEOUT
	}

	return p.HandleAnnounce(StructAnnounce{p.TreeNode(),
		Announce{""}})

}

// Dispatch is the main logic of the cypherium protocol.
func (p *TemplateProtocol) Dispatch() error {

	log.Lvl1(p.Name(), "Dispatch start")
	if !p.IsRoot() {
		return nil
	}

	// FIXME handle different failure modes
	// fail := (p.rootFailMode != 0) && p.IsRoot()
	// var timeoutStarted bool
	// for {
	// 	var err error
	// 	select {
	// 	//Handle Announcement PREPARE phase or somewhere need to start timer. handle different failure modes or view change
	// 	case timeout := <-p.timeoutChan:
	// 		// start the timer
	// 		if timeoutStarted {
	// 			continue
	// 		}
	// 		timeoutStarted = true
	// 		go p.startTimer(timeout)
	// 	case msg := <-p.viewchangeChan:
	// 		// receive view change
	// 		err = p.handleViewChange(msg.TreeNode, &msg.viewChange)
	// 		if err != nil {
	// 			log.Error("Error handling messages1:", err)
	// 		}
	// 	case <-p.doneProcessing:
	// 		// we are done
	// 		log.Lvl2(p.Name(), "ByzCoin Dispatches stop.")
	// 		p.tempBlock = nil
	// 		return nil
	// 	}
	// 	if err != nil {
	// 		log.Error("Error handling messages:", err)
	// 	}
	// }

	return nil
}

// HandleAnnounce is the first message and is used to send an ID that
// is stored in all nodes.
func (p *TemplateProtocol) HandleAnnounce(msg StructAnnounce) error {
	log.Lvl3("Parent announces:", msg.Message)
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		p.SendToChildren(&msg.Announce)
	} else {
		// If we're the leaf, start to reply
		p.HandleReply(nil)
	}
	return nil
}

// HandleReply is the message going up the tree and holding a counter
// to verify the number of nodes.
func (p *TemplateProtocol) HandleReply(reply []StructReply) error {
	defer p.Done()

	children := 1
	for _, c := range reply {
		children += c.ChildrenCount
	}
	log.Lvl3(p.ServerIdentity().Address, "is done with total of", children)
	if !p.IsRoot() {
		log.Lvl3("Sending to parent")
		return p.SendTo(p.Parent(), &Reply{children})
	}
	log.Lvl3("Root-node is done - nbr of children found:", children)
	p.ChildCount <- children
	p.Res <- PBFT_OK
	return nil
}

// GetBlock returns the next block available from the transaction pool.
func GetBlock(transactions []blockchain.STransaction, lastBlock, lastKeyBlock string) (*blockchain.TxBlock, error) {
	if len(transactions) < 1 {
		return nil, errors.New("no transaction available")
	}

	//fmt.Printf("Cypherium GetBlock: %+v\n", transactions)
	trlist := blockchain.NewTxBlockData(transactions, len(transactions))
	header := blockchain.NewHeader(lastBlock, lastKeyBlock)
	txblock := blockchain.NewTxBlock(trlist, header)
	return txblock, nil
}

// RegisterOnDone registers a callback to call when the byzcoin protocols has
// really finished (after a view change maybe)
func (p *TemplateProtocol) RegisterOnDone(fn func()) {
	p.onDoneCallback = fn
}

//RegisterOnSignatureDone register a callback to call when the byzcoin
//protocol reached a signature on the block
// func (p *TemplateProtocol) RegisterOnSignatureDone(fn func(*BlockSignature)) {
// 	p.onSignatureDone = fn
// }

// startTimer starts the timer to decide whether we should request a view change
// after a certain timeout or not. If the signature is done, we don't. otherwise
// we start the view change protocol.
func (p *TemplateProtocol) startTimer(millis uint64) {
	if p.rootFailMode != 0 {
		log.Lvl3(p.Name(), "Started timer (", millis, ")...")
		select {
		case <-p.doneSigning:
			return
		case <-time.After(time.Millisecond * time.Duration(millis)):
			p.sendAndMeasureViewchange()
		}
	}
}

// sendAndMeasureViewChange is a method that creates the viewchange request,
// broadcast it and measures the time it takes to accept it.
func (p *TemplateProtocol) sendAndMeasureViewchange() {
	log.Lvl3(p.Name(), "Created viewchange measure")
	vc := newViewChange()
	var err error
	for _, n := range p.Tree().List() {
		// don't send to ourself
		if n.ID.Equal(p.TreeNode().ID) {
			continue
		}
		err = p.SendTo(n, vc)
		if err != nil {
			log.Error(p.Name(), "Error sending view change", err)
		}
	}
}

// viewChange is simply the last hash / id of the previous leader.
type viewChange struct {
	LastBlock [sha256.Size]byte
}

// newViewChange creates a new view change.
func newViewChange() *viewChange {
	res := &viewChange{}
	for i := 0; i < sha256.Size; i++ {
		res.LastBlock[i] = 0
	}
	return res
}

// handleViewChange receives a view change request and if received more than
// 2/3, accept the view change.
func (p *TemplateProtocol) handleViewChange(tn *onet.TreeNode, vc *viewChange) error {
	p.vcCounter++
	// only do it once
	if p.vcCounter == p.viewChangeThreshold {
		if p.IsRoot() {
			log.Lvl3(p.Name(), "Viewchange threshold reached (2/3) of all nodes")
			go p.Done()
			//	bz.endProto.Start()
		}
		return nil
	}
	return nil
}

// nodeDone is either called by the end of EndProtocol or by the end of the
// response phase of the commit round.
func (p *TemplateProtocol) nodeDone() bool {
	log.Lvl3(p.Name(), "nodeDone()      ----- ")
	p.doneProcessing <- true
	log.Lvl3(p.Name(), "nodeDone()      +++++  ", p.onDoneCallback)
	if p.onDoneCallback != nil {
		p.onDoneCallback()
	}
	return true
}
