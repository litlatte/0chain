package state

import (
	"sync"

	"0chain.net/chaincore/block"
	"0chain.net/chaincore/config"
	"0chain.net/chaincore/state"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	"0chain.net/core/util"
	"0chain.net/smartcontract/dbs/event"
)

var (
	approvedMinters = []string{
		"6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d9", // miner SC
		"cf8d0df9bd8cc637a4ff4e792ffe3686da6220c45f0e1103baa609f3f1751ef4", // interest SC
		"6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7", // storage SC
		"6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712e0"} //zcn SC
)

/*
* The state context is available to the smart contract logic.
* The smart contract logic can use
*    GetClientBalance - to get the balance of a client at the beginning of executing the transaction.
*    AddTransfer - to add transfer of tokens from one client to another.
*  Restrictions:
*    1) The total transfer out from the txn.ClientID should be <= txn.Value
*    2) The only from clients valid are txn.ClientID and txn.ToClientID (which will be the smart contract's client id)
 */

//StateContextI - a state context interface. These interface are available for the smart contract
// todo this needs to be split up into different interfaces
type StateContextI interface {
	GetLastestFinalizedMagicBlock() *block.Block
	GetChainCurrentMagicBlock() *block.MagicBlock
	GetBlock() *block.Block                   // Can use in REST endpoints
	SetMagicBlock(block *block.MagicBlock)    // cannot use in smart contracts or REST endpoints
	GetState() util.MerklePatriciaTrieI       // cannot use in smart contracts or REST endpoints
	GetTransaction() *transaction.Transaction // cannot use in smart contracts or REST endpoints
	GetClientBalance(clientID datastore.Key) (state.Balance, error)
	SetStateContext(st *state.State) error                    // cannot use in smart contracts or REST endpoints
	GetTrieNode(key datastore.Key) (util.Serializable, error) // Can use in REST endpoints
	InsertTrieNode(key datastore.Key, node util.Serializable) (datastore.Key, error)
	DeleteTrieNode(key datastore.Key) (datastore.Key, error)
	AddTransfer(t *state.Transfer) error
	AddSignedTransfer(st *state.SignedTransfer)
	AddMint(m *state.Mint) error
	GetTransfers() []*state.Transfer // cannot use in smart contracts or REST endpoints
	GetSignedTransfers() []*state.SignedTransfer
	GetMints() []*state.Mint // cannot use in smart contracts or REST endpoints
	Validate() error
	GetBlockSharders(b *block.Block) []string
	GetSignatureScheme() encryption.SignatureScheme
	EmitEvent(string, string, string)
	EmitError(error)
	GetEvents() []event.Event   // cannot use in smart contracts or REST endpoints
	GetEventDB() *event.EventDb // do not use in smart contracts can use in REST endpoints
}

//StateContext - a context object used to manipulate global state
type StateContext struct {
	block                         *block.Block
	state                         util.MerklePatriciaTrieI
	txn                           *transaction.Transaction
	transfers                     []*state.Transfer
	signedTransfers               []*state.SignedTransfer
	mints                         []*state.Mint
	events                        []event.Event
	clientStateDeserializer       state.DeserializerI
	getSharders                   func(*block.Block) []string
	getLastestFinalizedMagicBlock func() *block.Block
	getChainCurrentMagicBlock     func() *block.MagicBlock
	getSignature                  func() encryption.SignatureScheme
	eventDb                       *event.EventDb
	mutex                         *sync.Mutex
}

// NewStateContext - create a new state context
func NewStateContext(
	b *block.Block,
	s util.MerklePatriciaTrieI,
	csd state.DeserializerI,
	t *transaction.Transaction,
	getSharderFunc func(*block.Block) []string,
	getLastestFinalizedMagicBlock func() *block.Block,
	getChainCurrentMagicBlock func() *block.MagicBlock,
	getChainSignature func() encryption.SignatureScheme,
	eventDb *event.EventDb,
) (
	balances *StateContext,
) {
	return &StateContext{
		block:                         b,
		state:                         s,
		clientStateDeserializer:       csd,
		txn:                           t,
		getSharders:                   getSharderFunc,
		getLastestFinalizedMagicBlock: getLastestFinalizedMagicBlock,
		getChainCurrentMagicBlock:     getChainCurrentMagicBlock,
		getSignature:                  getChainSignature,
		eventDb:                       eventDb,
		mutex:                         new(sync.Mutex),
	}
}

//GetBlock - get the block associated with this state context
func (sc *StateContext) GetBlock() *block.Block {
	return sc.block
}

func (sc *StateContext) SetMagicBlock(block *block.MagicBlock) {
	sc.block.MagicBlock = block
}

//GetState - get the state MPT associated with this state context
func (sc *StateContext) GetState() util.MerklePatriciaTrieI {
	return sc.state
}

//GetTransaction - get the transaction associated with this context
func (sc *StateContext) GetTransaction() *transaction.Transaction {
	return sc.txn
}

//AddTransfer - add the transfer
func (sc *StateContext) AddTransfer(t *state.Transfer) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	if t.ClientID != sc.txn.ClientID && t.ClientID != sc.txn.ToClientID {
		return state.ErrInvalidTransfer
	}
	sc.transfers = append(sc.transfers, t)
	return nil
}

//AddSignedTransfer - add the signed transfer
func (sc *StateContext) AddSignedTransfer(st *state.SignedTransfer) {
	// Signature on the signed transfer will be checked on call to sc.Validate()
	sc.signedTransfers = append(sc.signedTransfers, st)
}

//AddMint - add the mint
func (sc *StateContext) AddMint(m *state.Mint) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	if !sc.isApprovedMinter(m) {
		return state.ErrInvalidMint
	}
	sc.mints = append(sc.mints, m)
	return nil
}

func (sc *StateContext) isApprovedMinter(m *state.Mint) bool {
	for _, minter := range approvedMinters {
		if m.Minter == minter && sc.txn.ToClientID == minter {
			return true
		}
	}
	return false
}

//GetTransfers - get all the transfers
func (sc *StateContext) GetTransfers() []*state.Transfer {
	return sc.transfers
}

//GetTransfers - get all the transfers
func (sc *StateContext) GetSignedTransfers() []*state.SignedTransfer {
	return sc.signedTransfers
}

//GetMints - get all the mints and fight bad breath
func (sc *StateContext) GetMints() []*state.Mint {
	return sc.mints
}

func (sc *StateContext) EmitEvent(eventType, tag string, data string) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.events = append(sc.events, event.Event{
		BlockNumber: sc.block.Round,
		TxHash:      sc.txn.Hash,
		Type:        eventType,
		Tag:         tag,
		Data:        data,
	})
}

func (sc *StateContext) EmitError(err error) {
	sc.events = []event.Event{
		{
			BlockNumber: sc.block.Round,
			TxHash:      sc.txn.Hash,
			Type:        "Error",
			Data:        err.Error(),
		},
	}
}

func (sc *StateContext) GetEvents() []event.Event {
	return sc.events
}

func (sc *StateContext) GetEventDB() *event.EventDb {
	return sc.eventDb
}

//Validate - implement interface
func (sc *StateContext) Validate() error {
	var amount state.Balance
	for _, transfer := range sc.transfers {
		if transfer.ClientID == sc.txn.ClientID {
			amount += transfer.Amount
		} else {
			if transfer.ClientID != sc.txn.ToClientID {
				return state.ErrInvalidTransfer
			}
		}
		if transfer.Amount < 0 {
			return state.ErrInvalidTransfer
		}
	}
	totalValue := state.Balance(sc.txn.Value)
	if config.DevConfiguration.IsFeeEnabled {
		totalValue += state.Balance(sc.txn.Fee)
	}
	if amount > totalValue {
		return state.ErrInvalidTransfer
	}

	for _, signedTransfer := range sc.signedTransfers {
		err := signedTransfer.VerifySignature(true)
		if err != nil {
			return err
		}
		if signedTransfer.Amount <= 0 {
			return state.ErrInvalidTransfer
		}
	}

	return nil
}

func (sc *StateContext) getClientState(clientID string) (*state.State, error) {
	s := &state.State{}
	s.Balance = state.Balance(0)
	ss, err := sc.state.GetNodeValue(util.Path(clientID))
	if err != nil {
		if err != util.ErrValueNotPresent {
			return nil, err
		}
		return s, err
	}
	s = sc.clientStateDeserializer.Deserialize(ss).(*state.State)
	//TODO: should we apply the pending transfers?
	return s, nil
}

//GetClientBalance - get the balance of the client
func (sc *StateContext) GetClientBalance(clientID string) (state.Balance, error) {
	s, err := sc.getClientState(clientID)
	if err != nil {
		return 0, err
	}
	return s.Balance, nil
}

func (sc *StateContext) GetBlockSharders(b *block.Block) []string {
	return sc.getSharders(b)
}

func (sc *StateContext) GetLastestFinalizedMagicBlock() *block.Block {
	return sc.getLastestFinalizedMagicBlock()
}

func (sc *StateContext) GetChainCurrentMagicBlock() *block.MagicBlock {
	return sc.getChainCurrentMagicBlock()
}

func (sc *StateContext) GetSignatureScheme() encryption.SignatureScheme {
	return sc.getSignature()
}

func (sc *StateContext) GetTrieNode(key datastore.Key) (util.Serializable, error) {
	key_hash := encryption.Hash(key)
	return sc.state.GetNodeValue(util.Path(key_hash))
}

func (sc *StateContext) InsertTrieNode(key datastore.Key, node util.Serializable) (datastore.Key, error) {
	key_hash := encryption.Hash(key)
	byteKey, err := sc.state.Insert(util.Path(key_hash), node)
	return datastore.Key(byteKey), err
}

func (sc *StateContext) DeleteTrieNode(key datastore.Key) (datastore.Key, error) {
	key_hash := encryption.Hash(key)
	byteKey, err := sc.state.Delete(util.Path(key_hash))
	return datastore.Key(byteKey), err
}

//SetStateContext - set the state context
func (sc *StateContext) SetStateContext(s *state.State) error {
	s.SetRound(sc.block.Round)
	return s.SetTxnHash(sc.txn.Hash)
}
