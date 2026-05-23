package main

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

/*
========================================================
DATA STRUCTURES (НЕ МІНЯЄМО)
========================================================
*/

type MatchDetail struct {
	Address    string `json:"address"`
	Name       string `json:"name"`
	Source     string `json:"source"`
	MatchedVia string `json:"matched_via"` // from | to | log | contract
}
type TokenTransfer struct {
	Token  common.Address
	From   common.Address
	To     common.Address
	Amount *big.Int
}

type TxAnalysisContext struct {
	TxHash      common.Hash
	ChainID     int64
	BlockNumber uint64
	BlockTime   time.Time

	Tx      *types.Transaction
	Receipt *types.Receipt

	Matches        []MatchDetail
	TokenTransfers []TokenTransfer
}

/*
========================================================
CORE FUNCTION
========================================================
*/

func CheckTxHashForKnownAddresses(
	client *ethclient.Client,
	txHash common.Hash,
) (*TxAnalysisContext, error) {

	ctx := context.Background()

	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, err
	}

	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, err
	}

	block, err := client.BlockByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		return nil, err
	}

	found := make(map[string]MatchDetail)

	check := func(addr common.Address, via string) {
		a := strings.ToLower(addr.Hex())

		info, err := DB_GetKnownAddress(a)
		if err != nil || info == nil {
			return
		}

		found[a] = MatchDetail{
			Address:    a,
			Name:       info.Name,
			Source:     info.Source,
			MatchedVia: via,
		}
	}

	// from
	// from (SAFE signer)
	var from common.Address

	chainID := tx.ChainId()
	if chainID == nil || chainID.Sign() == 0 {
		// fallback для legacy / BSC / кривих tx
		signer := types.LatestSignerForChainID(big.NewInt(1))
		from, err = types.Sender(signer, tx)
	} else {
		signer := types.LatestSignerForChainID(chainID)
		from, err = types.Sender(signer, tx)
	}

	if err == nil {
		check(from, "from")
	}

	// to
	if tx.To() != nil {
		check(*tx.To(), "to")
	}

	// contract
	if receipt.ContractAddress != (common.Address{}) {
		check(receipt.ContractAddress, "contract")
	}

	// logs
	for _, lg := range receipt.Logs {
		check(lg.Address, "log")
	}

	// result matches
	matches := make([]MatchDetail, 0, len(found))
	for _, v := range found {
		matches = append(matches, v)
	}

	// parse ERC20 transfers
	tokenTransfers := extractERC20Transfers(receipt)

	return &TxAnalysisContext{
		TxHash:      txHash,
		ChainID:     tx.ChainId().Int64(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		BlockTime:   time.Unix(int64(block.Time()), 0),

		Tx:      tx,
		Receipt: receipt,

		Matches:        matches,
		TokenTransfers: tokenTransfers,
	}, nil
}

var erc20TransferTopic = common.HexToHash(
	"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
)

func extractERC20Transfers(receipt *types.Receipt) []TokenTransfer {
	var out []TokenTransfer

	for _, log := range receipt.Logs {
		if len(log.Topics) != 3 {
			continue
		}
		if log.Topics[0] != erc20TransferTopic {
			continue
		}

		amount := new(big.Int).SetBytes(log.Data)

		out = append(out, TokenTransfer{
			Token:  log.Address,
			From:   common.BytesToAddress(log.Topics[1].Bytes()[12:]),
			To:     common.BytesToAddress(log.Topics[2].Bytes()[12:]),
			Amount: amount,
		})
	}
	return out
}

/*
========================================================
DB HELPERS (READ)
========================================================
*/
