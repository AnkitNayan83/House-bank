package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	Querier
	TransferMoneyTx(ctx context.Context, arg TransferMoneyTxParams) (TransfeMoneyTxResult, error)
}

// store provides all the functions to execute db queries and transactions
type SQLStore struct {
	*Queries
	db *pgxpool.Pool
}

// NewStore create a new store from store sturct
func NewStore(db *pgxpool.Pool) Store {
	return &SQLStore{
		db:      db,
		Queries: New(db),
	}
}

// execTx executes a function within a database transeaction
func (store *SQLStore) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := store.db.BeginTx(ctx, pgx.TxOptions{})

	if err != nil {
		return err
	}

	q := New(tx)
	err = fn(q)

	if err != nil {
		if rbErr := tx.Rollback(context.Background()); rbErr != nil {
			return fmt.Errorf("tx error: %v, rb error: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit(context.Background())
}

type TransferMoneyTxParams struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}

type TransfeMoneyTxResult struct {
	Transfer    *Transfer `json:"transfer"`
	FromAccount *Account  `json:"from_account"`
	ToAccount   *Account  `json:"to_account"`
	FromEntry   *Entry    `json:"from_entry"`
	ToEntry     *Entry    `json:"to_entry"`
}

// txKey is a custom key for transaction context. It will allow us to pass the name of the transaction
// to the context so we can log it later
// type txKetType struct{}

// var txKey = txKetType{}

func (store *SQLStore) TransferMoneyTx(ctx context.Context, arg TransferMoneyTxParams) (TransfeMoneyTxResult, error) {
	var result TransfeMoneyTxResult

	err := store.execTx(ctx, func(q *Queries) error {

		// txName := ctx.Value(txKey)

		// fmt.Println(txName, ">> create transfer")
		// create transfer
		transfer, err := q.CreateTransfer(ctx, CreateTransferParams(arg))
		if err != nil {
			return err
		}
		result.Transfer = &transfer

		// create two entries
		// from entry
		fromEntry, err := q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.FromAccountID,
			Amount:    -arg.Amount,
		})

		if err != nil {
			return err
		}
		result.FromEntry = &fromEntry

		// to entry
		toEntry, err := q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.ToAccountID,
			Amount:    arg.Amount,
		})

		if err != nil {
			return err
		}
		result.ToEntry = &toEntry
		// to avaoid dl
		if arg.FromAccountID < arg.ToAccountID {

			fromAcc, toAcc, err := addMoney(ctx, q, arg.FromAccountID, -arg.Amount, arg.ToAccountID, arg.Amount)

			if err != nil {
				return err
			}

			result.FromAccount = &fromAcc
			result.ToAccount = &toAcc
		} else {
			toAcc, fromAcc, err := addMoney(ctx, q, arg.ToAccountID, arg.Amount, arg.FromAccountID, -arg.Amount)

			if err != nil {
				return err
			}

			result.FromAccount = &fromAcc
			result.ToAccount = &toAcc
		}

		return nil
	})

	return result, err
}

func addMoney(
	ctx context.Context,
	q *Queries,
	account1Id int64,
	amount1 int64,
	account2Id int64,
	amount2 int64,
) (account1, account2 Account, err error) {
	account1, err = q.AddAccountBalance(ctx, AddAccountBalanceParams{
		ID:     account1Id,
		Amount: amount1,
	})

	if err != nil {
		return
	}

	account2, err = q.AddAccountBalance(ctx, AddAccountBalanceParams{
		ID:     account2Id,
		Amount: amount2,
	})

	if err != nil {
		return
	}

	return account1, account2, nil
}
