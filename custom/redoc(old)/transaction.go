package main

//Transaction entity that encapsulates the transaction related data and meta data
type Transaction struct {
	// Hash of the transaction
	//
	// required: true
	Hash string `json:"hash,omitempty"`
	// Version of the API. Set it to 1.0
	//
	// required: true
	Version string `json:"version,omitempty"`
	// ClientID of the originating wallet
	//
	// required: true
	ClientID string `json:"client_id,omitempty"`
	// PublicKey of the originating wallet
	//
	PublicKey string `json:"public_key,omitempty"`
	// ToClientID of the destination wallet
	//
	// required: true
	ToClientID string `json:"to_client_id,omitempty"`
	// ChainID of the blockchain where wallets are active
	//
	// required: true`
	ChainID string `json:"chain_id,omitempty"`
	// Serialized Object of one of Send or StoreData or SmartContract object
	//
	// required: true
	TransactionData string `json:"transaction_data"`
	// Value of Send Transaction. For all other transaction type set it to 0
	//
	// required: true
	Value int64 `json:"transaction_value"`
	// Signature of the orginating wallet
	//
	// required: true
	Signature string `json:"signature,omitempty"`
	// Creation Date of the transaction. This will determine the age of the transaction.
	//
	// required: true
	CreationDate int64 `json:"creation_date,omitempty"`
	// Type of transaction. Should be one of:
	// * 0 for Send,
	// * 10 for StoreData,
	// * 1000 for Smartcontract
	//
	// required: true
	TransactionType int `json:"transaction_type"`
	// Transaction output present only in the response
	//
	TransactionOutput string `json:"transaction_output,omitempty"`
	// Gas money the originating wallet is willing to pay.
	//
	// required: true
	TransactionFee int64 `json:"transaction_fee"`
	// Hash of the TransactionOutput. Present only in the response
	//
	OutputHash string `json:"txn_output_hash"`
}

// swagger:parameters GetBlock
type blockParams struct {
	// The round number of the block
	Round int64 `json:"round"`
	// content expected in the response. Use "header"
	Content string `json:"content"`
}

// swagger:parameters confirmTransaction
type confirmationParams struct {
	// Transaction hash you are interested in
	Hash string `json:"hash"`
}

// Transaction serializabled blockheader in which the requested transaction is proecessed
// swagger:response BlockHeader
// in: body
type MyBlockResponse struct {

	// The block header information.
	//
	// in: body
	// required: true
	Blockheader *BlockHeader `json:"block"`
}

type BlockHeader struct {
	// Version of the block object. Currently 1.0
	Version string `json:"version,omitempty"`
	// Creation date of the block object.
	CreationDate int64 `json:"creation_date,omitempty"`
	// Hash of the Block
	Hash string `json:"hash,omitempty"`
	// ID of the Miner who generated this block
	MinerID string `json:"miner_id,omitempty"`
	// Round in which the block is processed
	Round int64 `json:"round,omitempty"`
	// RoundRandomSeed that is used for this block
	RoundRandomSeed int64 `json:"round_random_seed,omitempy"`
	// MerkleTreeRoot
	MerkleTreeRoot string `json:"merkle_tree_root,omitempty"`
	// Hash of the state when the block is processed
	StateHash string `json:"state_hash,omitempty"`

	ReceiptMerkleTreeRoot string `json:"receipt_merkle_tree_root,omitempty"`
	// Number of transactions in this block
	NumTxns int64 `json:"num_txns,omitempty"`
}

// Transaction request parameter used in post
// swagger:parameters putTransaction
type MyTransactionParam struct {

	// The transaction to submit.
	//
	// in: body
	// required: true
	Transaction *Transaction `json:"transaction"`
}

// Transaction response serialized
// swagger:response Transaction
type MyTransactionResponse struct {
	// The transaction with transaction hash
	// in: body
	Transaction *Transaction `json:"transaction"`
}

// PutTransaction - Given a transaction data, it stores it
// swagger:route PUT /v1/transaction/put transactions putTransaction
//
// Handler to put transactions
//
// Responses:
// 		    200: Transaction
//
/*PutTransaction - Given a transaction data, it stores it */
func PutTransaction() (interface{}, error) {

	return nil, nil
}

// ConfirmTransaction - Given a transaction hash, confirms with the block that has the transaction if processed; otherwise nil
// swagger:route GET /v1/transaction/get/confirmation transactions confirmTransaction
//
// Handler to check transaction status
//
// Responses:
// 		    200: BlockHeader
//
/*ConfirmTransaction - Given a transaction hash, get the block that includes it */
func ConfirmTransaction() *BlockHeader {
	return nil
}

// GetBlock - Given a transaction hash, confirms with the block that has the transaction if processed; otherwise nil
// swagger:route GET /v1/block/get blocks GetBlock
//
// Handler to get a block associated to a round
//
// Responses:
// 		    200: BlockHeader
//
/*ConfirmTransaction - Given a transaction hash, get the block that includes it */
func GetBlock() *BlockHeader {
	return nil
}
