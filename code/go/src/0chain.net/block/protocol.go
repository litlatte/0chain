package block

import (
	"context"

	"0chain.net/datastore"
	"0chain.net/transaction"
)

var BLOCK_SIZE = 250000

/*GenerateBlock - This works on generating a block
* The context should be a background context which can be used to stop this logic if there is a new
* block published while working on this
 */
func (b *Block) GenerateBlock(ctx context.Context) error {
	b.Txns = make([]*transaction.Transaction, BLOCK_SIZE)
	//TODO: wasting this because we []interface{} != []*transaction.Transaction in Go
	txns := make([]datastore.Entity, BLOCK_SIZE)
	idx := 0
	var txnIterHandler = func(ctx context.Context, qe datastore.CollectionEntity) bool {
		select {
		case <-ctx.Done():
			datastore.GetCon(ctx).Close()
			return false
		default:
		}
		txn, ok := qe.(*transaction.Transaction)
		if !ok {
			return true
		}
		if txn.Status != transaction.TXN_STATUS_FREE {
			return true
		}
		txn.Status = transaction.TXN_STATUS_PENDING
		b.Txns[idx] = txn
		txns[idx] = txn
		b.AddTransaction(txn)
		idx++
		if idx == BLOCK_SIZE {
			b.UpdateTxnsToPending(ctx, txns)
			return false
		}
		return true
	}
	err := datastore.IterateCollection(ctx, txnIterHandler, transaction.Provider)
	return err
}

/*UpdateTxnsToPending - marks all the given transactions to pending */
func (b *Block) UpdateTxnsToPending(ctx context.Context, txns []datastore.Entity) {
	datastore.MultiWrite(ctx, txns)
}

/*VerifyBlock - given a set of transaction ids within a block, validate the block */
func (b *Block) VerifyBlock(ctx context.Context) (bool, error) {
	return true, nil
}

/*Finalize - finalize the transactions in the block */
func (b *Block) Finalize(ctx context.Context) error {
	modifiedTxns := make([]datastore.Entity, 0, BLOCK_SIZE)
	for idx, txn := range b.Txns {
		txn.BlockID = b.ID
		txn.Status = transaction.TXN_STATUS_FINALIZED
		modifiedTxns[idx] = txn
	}
	err := datastore.MultiWrite(ctx, modifiedTxns)
	if err != nil {
		return err
	}
	return nil
}
