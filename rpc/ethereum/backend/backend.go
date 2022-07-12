package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	tmrpctypes "github.com/tendermint/tendermint/rpc/core/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/tharsis/ethermint/rpc/ethereum/types"
	"github.com/tharsis/ethermint/server/config"
	ethermint "github.com/tharsis/ethermint/types"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
	feemarkettypes "github.com/tharsis/ethermint/x/feemarket/types"
)

// Backend implements the functionality shared within namespaces.
// Implemented by EVMBackend.
type Backend interface {
	// Fee API
	FeeHistory(blockCount rpc.DecimalOrHex, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (*types.FeeHistoryResult, error)

	// General Ethereum API
	RPCGasCap() uint64            // global gas cap for eth_call over rpc: DoS protection
	RPCEVMTimeout() time.Duration // global timeout for eth_call over rpc: DoS protection
	RPCTxFeeCap() float64         // RPCTxFeeCap is the global transaction fee(price * gaslimit) cap for send-transaction variants. The unit is ether.

	RPCMinGasPrice() int64
	SuggestGasTipCap(baseFee *big.Int) (*big.Int, error)

	// Blockchain API
	BlockNumber() (hexutil.Uint64, error)
	GetTendermintBlockByNumber(blockNum types.BlockNumber) (*tmrpctypes.ResultBlock, error)
	GetTendermintBlockByHash(blockHash common.Hash) (*tmrpctypes.ResultBlock, error)
	GetTendermintBlockResultByNumber(*int64) (*tmrpctypes.ResultBlockResults, error)
	GetBlockByNumber(blockNum types.BlockNumber, fullTx bool) (map[string]interface{}, error)
	GetBlockByHash(hash common.Hash, fullTx bool) (map[string]interface{}, error)
	BlockByNumber(blockNum types.BlockNumber) (*ethtypes.Block, error)
	BlockByHash(blockHash common.Hash) (*ethtypes.Block, error)
	CurrentHeader() *ethtypes.Header
	HeaderByNumber(blockNum types.BlockNumber) (*ethtypes.Header, error)
	HeaderByHash(blockHash common.Hash) (*ethtypes.Header, error)
	PendingTransactions() ([]*sdk.Tx, error)
	GetTransactionCount(address common.Address, blockNum types.BlockNumber) (*hexutil.Uint64, error)
	SendTransaction(args evmtypes.TransactionArgs) (common.Hash, error)
	GetCoinbase() (sdk.AccAddress, error)
	GetTransactionByHash(txHash common.Hash) (*types.RPCTransaction, error)
	GetTxByEthHash(txHash common.Hash) (*tmrpctypes.ResultTx, error)
	GetTxByTxIndex(height int64, txIndex uint) (*tmrpctypes.ResultTx, error)
	EstimateGas(args evmtypes.TransactionArgs, blockNrOptional *types.BlockNumber) (hexutil.Uint64, error)
	BaseFee(blockRes *tmrpctypes.ResultBlockResults) (*big.Int, error)

	// Filter API
	BloomStatus() (uint64, uint64)
	GetLogs(hash common.Hash) ([][]*ethtypes.Log, error)
	GetLogsByHeight(height *int64) ([][]*ethtypes.Log, error)
	ChainConfig() *params.ChainConfig
	SetTxDefaults(args evmtypes.TransactionArgs) (evmtypes.TransactionArgs, error)
	GetEthereumMsgsFromTendermintBlock(block *tmrpctypes.ResultBlock, blockRes *tmrpctypes.ResultBlockResults) []*evmtypes.MsgEthereumTx
}

var _ Backend = (*EVMBackend)(nil)

var bAttributeKeyEthereumBloom = []byte(evmtypes.AttributeKeyEthereumBloom)

// EVMBackend implements the Backend interface
type EVMBackend struct {
	ctx         context.Context
	clientCtx   client.Context
	queryClient *types.QueryClient // gRPC query client
	logger      log.Logger
	chainID     *big.Int
	cfg         config.Config
}

// NewEVMBackend creates a new EVMBackend instance
func NewEVMBackend(ctx *server.Context, logger log.Logger, clientCtx client.Context) *EVMBackend {
	chainID, err := ethermint.ParseChainID(clientCtx.ChainID)
	if err != nil {
		panic(err)
	}

	appConf := config.GetConfig(ctx.Viper)

	return &EVMBackend{
		ctx:         context.Background(),
		clientCtx:   clientCtx,
		queryClient: types.NewQueryClient(clientCtx),
		logger:      logger.With("module", "evm-backend"),
		chainID:     chainID,
		cfg:         appConf,
	}
}

// BlockNumber returns the current block number in abci app state.
// Because abci app state could lag behind from tendermint latest block, it's more stable
// for the client to use the latest block number in abci app state than tendermint rpc.
func (e *EVMBackend) BlockNumber() (hexutil.Uint64, error) {
	// do any grpc query, ignore the response and use the returned block height
	var header metadata.MD
	_, err := e.queryClient.Params(e.ctx, &evmtypes.QueryParamsRequest{}, grpc.Header(&header))
	if err != nil {
		return hexutil.Uint64(0), err
	}

	blockHeightHeader := header.Get(grpctypes.GRPCBlockHeightHeader)
	if headerLen := len(blockHeightHeader); headerLen != 1 {
		return 0, fmt.Errorf("unexpected '%s' gRPC header length; got %d, expected: %d", grpctypes.GRPCBlockHeightHeader, headerLen, 1)
	}

	height, err := strconv.ParseUint(blockHeightHeader[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block height: %w", err)
	}

	return hexutil.Uint64(height), nil
}

// GetBlockByNumber returns the block identified by number.
func (e *EVMBackend) GetBlockByNumber(blockNum types.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	resBlock, err := e.GetTendermintBlockByNumber(blockNum)
	if err != nil {
		return nil, err
	}

	// return if requested block height is greater than the current one
	if resBlock == nil || resBlock.Block == nil {
		return nil, nil
	}

	blockRes, err := e.GetTendermintBlockResultByNumber(&resBlock.Block.Height)
	if err != nil {
		e.logger.Debug("failed to fetch block result from Tendermint", "height", blockNum, "error", err.Error())
		return nil, nil
	}

	res, err := e.EthBlockFromTendermint(resBlock, blockRes, fullTx)
	if err != nil {
		e.logger.Debug("EthBlockFromTendermint failed", "height", blockNum, "error", err.Error())
		return nil, err
	}

	return res, nil
}

// GetBlockByHash returns the block identified by hash.
func (e *EVMBackend) GetBlockByHash(hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	resBlock, err := e.GetTendermintBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	if resBlock == nil {
		// block not found
		return nil, nil
	}

	blockRes, err := e.GetTendermintBlockResultByNumber(&resBlock.Block.Height)
	if err != nil {
		e.logger.Debug("failed to fetch block result from Tendermint", "block-hash", hash.String(), "error", err.Error())
		return nil, nil
	}

	return e.EthBlockFromTendermint(resBlock, blockRes, fullTx)
}

// BlockByNumber returns the block identified by number.
func (e *EVMBackend) BlockByNumber(blockNum types.BlockNumber) (*ethtypes.Block, error) {
	resBlock, err := e.GetTendermintBlockByNumber(blockNum)
	if err != nil {
		return nil, err
	}
	if resBlock == nil {
		// block not found
		return nil, fmt.Errorf("block not found for height %d", blockNum)
	}

	blockRes, err := e.GetTendermintBlockResultByNumber(&resBlock.Block.Height)
	if err != nil {
		return nil, fmt.Errorf("block result not found for height %d", resBlock.Block.Height)
	}

	return e.EthBlockFromTm(resBlock, blockRes)
}

// BlockByHash returns the block identified by hash.
func (e *EVMBackend) BlockByHash(hash common.Hash) (*ethtypes.Block, error) {
	resBlock, err := e.GetTendermintBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	if resBlock == nil || resBlock.Block == nil {
		return nil, fmt.Errorf("block not found for hash %s", hash)
	}

	blockRes, err := e.GetTendermintBlockResultByNumber(&resBlock.Block.Height)
	if err != nil {
		return nil, fmt.Errorf("block result not found for hash %s", hash)
	}

	return e.EthBlockFromTm(resBlock, blockRes)
}

func (e *EVMBackend) EthBlockFromTm(resBlock *tmrpctypes.ResultBlock, blockRes *tmrpctypes.ResultBlockResults) (*ethtypes.Block, error) {
	block := resBlock.Block
	height := block.Height
	bloom, err := e.BlockBloom(blockRes)
	if err != nil {
		e.logger.Debug("EthBlockFromTm BlockBloom failed", "height", height)
	}

	baseFee, err := e.BaseFee(blockRes)
	if err != nil {
		// handle error for pruned node and log
		e.logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", height, "error", err)
	}

	ethHeader := types.EthHeaderFromTendermint(block.Header, bloom, baseFee)

	resBlockResult, err := e.GetTendermintBlockResultByNumber(&block.Height)
	if err != nil {
		return nil, err
	}

	msgs := e.GetEthereumMsgsFromTendermintBlock(resBlock, resBlockResult)

	txs := make([]*ethtypes.Transaction, len(msgs))
	for i, ethMsg := range msgs {
		txs[i] = ethMsg.AsTransaction()
	}

	// TODO: add tx receipts
	ethBlock := ethtypes.NewBlock(ethHeader, txs, nil, nil, nil)
	return ethBlock, nil
}

// GetTendermintBlockByNumber returns a Tendermint format block by block number
func (e *EVMBackend) GetTendermintBlockByNumber(blockNum types.BlockNumber) (*tmrpctypes.ResultBlock, error) {
	height := blockNum.Int64()
	if height <= 0 {
		// fetch the latest block number from the app state, more accurate than the tendermint block store state.
		n, err := e.BlockNumber()
		if err != nil {
			return nil, err
		}
		height = int64(n)
	}
	resBlock, err := e.clientCtx.Client.Block(e.ctx, &height)
	if err != nil {
		e.logger.Debug("tendermint client failed to get block", "height", height, "error", err.Error())
		return nil, err
	}

	if resBlock.Block == nil {
		e.logger.Debug("GetTendermintBlockByNumber block not found", "height", height)
		return nil, nil
	}

	return resBlock, nil
}

// GetTendermintBlockResultByNumber returns a Tendermint-formatted block result by block number
func (e *EVMBackend) GetTendermintBlockResultByNumber(height *int64) (*tmrpctypes.ResultBlockResults, error) {
	return e.clientCtx.Client.BlockResults(e.ctx, height)
}

// GetTendermintBlockByHash returns a Tendermint format block by block number
func (e *EVMBackend) GetTendermintBlockByHash(blockHash common.Hash) (*tmrpctypes.ResultBlock, error) {
	resBlock, err := e.clientCtx.Client.BlockByHash(e.ctx, blockHash.Bytes())
	if err != nil {
		e.logger.Debug("tendermint client failed to get block", "blockHash", blockHash.Hex(), "error", err.Error())
		return nil, err
	}

	if resBlock == nil || resBlock.Block == nil {
		e.logger.Debug("GetTendermintBlockByHash block not found", "blockHash", blockHash.Hex())
		return nil, nil
	}

	return resBlock, nil
}

// BlockBloom query block bloom filter from block results
func (e *EVMBackend) BlockBloom(blockRes *tmrpctypes.ResultBlockResults) (ethtypes.Bloom, error) {
	for _, event := range blockRes.EndBlockEvents {
		if event.Type != evmtypes.EventTypeBlockBloom {
			continue
		}

		for _, attr := range event.Attributes {
			if bytes.Equal(attr.Key, bAttributeKeyEthereumBloom) {
				return ethtypes.BytesToBloom(attr.Value), nil
			}
		}
	}
	return ethtypes.Bloom{}, errors.New("block bloom event is not found")
}

// EthBlockFromTendermint returns a JSON-RPC compatible Ethereum block from a given Tendermint block and its block result.
func (e *EVMBackend) EthBlockFromTendermint(
	resBlock *tmrpctypes.ResultBlock,
	blockRes *tmrpctypes.ResultBlockResults,
	fullTx bool,
) (map[string]interface{}, error) {
	ethRPCTxs := []interface{}{}
	block := resBlock.Block

	baseFee, err := e.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		e.logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", block.Height, "error", err)
	}

	msgs := e.GetEthereumMsgsFromTendermintBlock(resBlock, blockRes)
	for txIndex, ethMsg := range msgs {
		if !fullTx {
			hash := common.HexToHash(ethMsg.Hash)
			ethRPCTxs = append(ethRPCTxs, hash)
			continue
		}

		tx := ethMsg.AsTransaction()
		rpcTx, err := types.NewRPCTransaction(
			tx,
			common.BytesToHash(block.Hash()),
			uint64(block.Height),
			uint64(txIndex),
			baseFee,
		)
		if err != nil {
			e.logger.Debug("NewTransactionFromData for receipt failed", "hash", tx.Hash().Hex(), "error", err.Error())
			continue
		}
		ethRPCTxs = append(ethRPCTxs, rpcTx)
	}

	bloom, err := e.BlockBloom(blockRes)
	if err != nil {
		e.logger.Debug("failed to query BlockBloom", "height", block.Height, "error", err.Error())
	}

	req := &evmtypes.QueryValidatorAccountRequest{
		ConsAddress: sdk.ConsAddress(block.Header.ProposerAddress).String(),
	}

	var validatorAccAddr sdk.AccAddress

	ctx := types.ContextWithHeight(block.Height)
	res, err := e.queryClient.ValidatorAccount(ctx, req)
	if err != nil {
		e.logger.Debug(
			"failed to query validator operator address",
			"height", block.Height,
			"cons-address", req.ConsAddress,
			"error", err.Error(),
		)
		// use zero address as the validator operator address
		validatorAccAddr = sdk.AccAddress(common.Address{}.Bytes())
	} else {
		validatorAccAddr, err = sdk.AccAddressFromBech32(res.AccountAddress)
		if err != nil {
			return nil, err
		}
	}

	validatorAddr := common.BytesToAddress(validatorAccAddr)

	gasLimit, err := types.BlockMaxGasFromConsensusParams(ctx, e.clientCtx, block.Height)
	if err != nil {
		e.logger.Error("failed to query consensus params", "error", err.Error())
	}

	gasUsed := uint64(0)

	for _, txsResult := range blockRes.TxsResults {
		// workaround for cosmos-sdk bug. https://github.com/cosmos/cosmos-sdk/issues/10832
		if ShouldIgnoreGasUsed(txsResult) {
			// block gas limit has exceeded, other txs must have failed with same reason.
			break
		}
		gasUsed += uint64(txsResult.GetGasUsed())
	}

	formattedBlock := types.FormatBlock(
		block.Header, block.Size(),
		gasLimit, new(big.Int).SetUint64(gasUsed),
		ethRPCTxs, bloom, validatorAddr, baseFee,
	)
	return formattedBlock, nil
}

// CurrentHeader returns the latest block header
func (e *EVMBackend) CurrentHeader() *ethtypes.Header {
	header, _ := e.HeaderByNumber(types.EthLatestBlockNumber)
	return header
}

// HeaderByNumber returns the block header identified by height.
func (e *EVMBackend) HeaderByNumber(blockNum types.BlockNumber) (*ethtypes.Header, error) {
	resBlock, err := e.GetTendermintBlockByNumber(blockNum)
	if err != nil {
		return nil, err
	}

	if resBlock == nil {
		return nil, errors.Errorf("block not found for height %d", blockNum)
	}

	blockRes, err := e.GetTendermintBlockResultByNumber(&resBlock.Block.Height)
	if err != nil {
		return nil, fmt.Errorf("block result not found for height %d", resBlock.Block.Height)
	}

	bloom, err := e.BlockBloom(blockRes)
	if err != nil {
		e.logger.Debug("HeaderByNumber BlockBloom failed", "height", resBlock.Block.Height)
	}

	baseFee, err := e.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		e.logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", resBlock.Block.Height, "error", err)
	}

	ethHeader := types.EthHeaderFromTendermint(resBlock.Block.Header, bloom, baseFee)
	return ethHeader, nil
}

// HeaderByHash returns the block header identified by hash.
func (e *EVMBackend) HeaderByHash(blockHash common.Hash) (*ethtypes.Header, error) {
	resBlock, err := e.GetTendermintBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}
	if resBlock == nil {
		return nil, errors.Errorf("block not found for hash %s", blockHash.Hex())
	}

	blockRes, err := e.GetTendermintBlockResultByNumber(&resBlock.Block.Height)
	if err != nil {
		return nil, errors.Errorf("block result not found for height %d", resBlock.Block.Height)
	}

	bloom, err := e.BlockBloom(blockRes)
	if err != nil {
		e.logger.Debug("HeaderByHash BlockBloom failed", "height", resBlock.Block.Height)
	}

	baseFee, err := e.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		e.logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", resBlock.Block.Height, "error", err)
	}

	ethHeader := types.EthHeaderFromTendermint(resBlock.Block.Header, bloom, baseFee)
	return ethHeader, nil
}

// PendingTransactions returns the transactions that are in the transaction pool
// and have a from address that is one of the accounts this node manages.
func (e *EVMBackend) PendingTransactions() ([]*sdk.Tx, error) {
	res, err := e.clientCtx.Client.UnconfirmedTxs(e.ctx, nil)
	if err != nil {
		return nil, err
	}

	result := make([]*sdk.Tx, 0, len(res.Txs))
	for _, txBz := range res.Txs {
		tx, err := e.clientCtx.TxConfig.TxDecoder()(txBz)
		if err != nil {
			return nil, err
		}
		result = append(result, &tx)
	}

	return result, nil
}

// GetLogsByHeight returns all the logs from all the ethereum transactions in a block.
func (e *EVMBackend) GetLogsByHeight(height *int64) ([][]*ethtypes.Log, error) {
	// NOTE: we query the state in case the tx result logs are not persisted after an upgrade.
	blockRes, err := e.GetTendermintBlockResultByNumber(height)
	if err != nil {
		return nil, err
	}

	return GetLogsFromBlockResults(blockRes)
}

// GetLogs returns all the logs from all the ethereum transactions in a block.
func (e *EVMBackend) GetLogs(hash common.Hash) ([][]*ethtypes.Log, error) {
	resBlock, err := e.GetTendermintBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	if resBlock == nil {
		return nil, errors.Errorf("block not found for hash %s", hash)
	}

	return e.GetLogsByHeight(&resBlock.Block.Header.Height)
}

// BloomStatus returns the BloomBitsBlocks and the number of processed sections maintained
// by the chain indexer.
func (e *EVMBackend) BloomStatus() (uint64, uint64) {
	return 4096, 0
}

// GetCoinbase is the address that staking rewards will be send to (alias for Etherbase).
func (e *EVMBackend) GetCoinbase() (sdk.AccAddress, error) {
	node, err := e.clientCtx.GetNode()
	if err != nil {
		return nil, err
	}

	status, err := node.Status(e.ctx)
	if err != nil {
		return nil, err
	}

	req := &evmtypes.QueryValidatorAccountRequest{
		ConsAddress: sdk.ConsAddress(status.ValidatorInfo.Address).String(),
	}

	res, err := e.queryClient.ValidatorAccount(e.ctx, req)
	if err != nil {
		return nil, err
	}

	address, _ := sdk.AccAddressFromBech32(res.AccountAddress)
	return address, nil
}

// GetTransactionByHash returns the Ethereum format transaction identified by Ethereum transaction hash
func (e *EVMBackend) GetTransactionByHash(txHash common.Hash) (*types.RPCTransaction, error) {
	res, err := e.GetTxByEthHash(txHash)
	hexTx := txHash.Hex()

	if err != nil {
		// try to find tx in mempool
		txs, err := e.PendingTransactions()
		if err != nil {
			e.logger.Debug("tx not found", "hash", hexTx, "error", err.Error())
			return nil, nil
		}

		for _, tx := range txs {
			msg, err := evmtypes.UnwrapEthereumMsg(tx, txHash)
			if err != nil {
				// not ethereum tx
				continue
			}

			if msg.Hash == hexTx {
				rpctx, err := types.NewTransactionFromMsg(
					msg,
					common.Hash{},
					uint64(0),
					uint64(0),
					nil,
				)
				if err != nil {
					return nil, err
				}
				return rpctx, nil
			}
		}

		e.logger.Debug("tx not found", "hash", hexTx)
		return nil, nil
	}

	if !TxSuccessOrExceedsBlockGasLimit(&res.TxResult) {
		return nil, errors.New("invalid ethereum tx")
	}

	parsedTxs, err := types.ParseTxResult(&res.TxResult)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tx events: %s", hexTx)
	}

	parsedTx := parsedTxs.GetTxByHash(txHash)
	if parsedTx == nil {
		return nil, fmt.Errorf("ethereum tx not found in msgs: %s", hexTx)
	}

	tx, err := e.clientCtx.TxConfig.TxDecoder()(res.Tx)
	if err != nil {
		return nil, err
	}

	// the `msgIndex` is inferred from tx events, should be within the bound.
	msg, ok := tx.GetMsgs()[parsedTx.MsgIndex].(*evmtypes.MsgEthereumTx)
	if !ok {
		return nil, errors.New("invalid ethereum tx")
	}

	block, err := e.clientCtx.Client.Block(e.ctx, &res.Height)
	if err != nil {
		e.logger.Debug("block not found", "height", res.Height, "error", err.Error())
		return nil, err
	}

	blockRes, err := e.GetTendermintBlockResultByNumber(&block.Block.Height)
	if err != nil {
		e.logger.Debug("block result not found", "height", block.Block.Height, "error", err.Error())
		return nil, nil
	}

	if parsedTx.EthTxIndex == -1 {
		// Fallback to find tx index by iterating all valid eth transactions
		msgs := e.GetEthereumMsgsFromTendermintBlock(block, blockRes)
		for i := range msgs {
			if msgs[i].Hash == hexTx {
				parsedTx.EthTxIndex = int64(i)
				break
			}
		}
	}
	if parsedTx.EthTxIndex == -1 {
		return nil, errors.New("can't find index of ethereum tx")
	}

	baseFee, err := e.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		e.logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", blockRes.Height, "error", err)
	}

	return types.NewTransactionFromMsg(
		msg,
		common.BytesToHash(block.BlockID.Hash.Bytes()),
		uint64(res.Height),
		uint64(parsedTx.EthTxIndex),
		baseFee,
	)
}

// GetTxByEthHash uses `/tx_query` to find transaction by ethereum tx hash
// TODO: Don't need to convert once hashing is fixed on Tendermint
// https://github.com/tendermint/tendermint/issues/6539
func (e *EVMBackend) GetTxByEthHash(hash common.Hash) (*tmrpctypes.ResultTx, error) {
	query := fmt.Sprintf("%s.%s='%s'", evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyEthereumTxHash, hash.Hex())
	resTxs, err := e.clientCtx.Client.TxSearch(e.ctx, query, false, nil, nil, "")
	if err != nil {
		return nil, err
	}
	if len(resTxs.Txs) == 0 {
		return nil, errors.Errorf("ethereum tx not found for hash %s", hash.Hex())
	}
	return resTxs.Txs[0], nil
}

// GetTxByTxIndex uses `/tx_query` to find transaction by tx index of valid ethereum txs
func (e *EVMBackend) GetTxByTxIndex(height int64, index uint) (*tmrpctypes.ResultTx, error) {
	query := fmt.Sprintf("tx.height=%d AND %s.%s=%d",
		height, evmtypes.TypeMsgEthereumTx,
		evmtypes.AttributeKeyTxIndex, index,
	)
	resTxs, err := e.clientCtx.Client.TxSearch(e.ctx, query, false, nil, nil, "")
	if err != nil {
		return nil, err
	}
	if len(resTxs.Txs) == 0 {
		return nil, errors.Errorf("ethereum tx not found for block %d index %d", height, index)
	}
	return resTxs.Txs[0], nil
}

func (e *EVMBackend) SendTransaction(args evmtypes.TransactionArgs) (common.Hash, error) {
	// Look up the wallet containing the requested signer
	_, err := e.clientCtx.Keyring.KeyByAddress(sdk.AccAddress(args.From.Bytes()))
	if err != nil {
		e.logger.Error("failed to find key in keyring", "address", args.From, "error", err.Error())
		return common.Hash{}, fmt.Errorf("%s; %s", keystore.ErrNoMatch, err.Error())
	}

	args, err = e.SetTxDefaults(args)
	if err != nil {
		return common.Hash{}, err
	}

	msg := args.ToTransaction()
	if err := msg.ValidateBasic(); err != nil {
		e.logger.Debug("tx failed basic validation", "error", err.Error())
		return common.Hash{}, err
	}

	bn, err := e.BlockNumber()
	if err != nil {
		e.logger.Debug("failed to fetch latest block number", "error", err.Error())
		return common.Hash{}, err
	}

	signer := ethtypes.MakeSigner(e.ChainConfig(), new(big.Int).SetUint64(uint64(bn)))

	// Sign transaction
	if err := msg.Sign(signer, e.clientCtx.Keyring); err != nil {
		e.logger.Debug("failed to sign tx", "error", err.Error())
		return common.Hash{}, err
	}

	// Query params to use the EVM denomination
	res, err := e.queryClient.QueryClient.Params(e.ctx, &evmtypes.QueryParamsRequest{})
	if err != nil {
		e.logger.Error("failed to query evm params", "error", err.Error())
		return common.Hash{}, err
	}

	// Assemble transaction from fields
	tx, err := msg.BuildTx(e.clientCtx.TxConfig.NewTxBuilder(), res.Params.EvmDenom)
	if err != nil {
		e.logger.Error("build cosmos tx failed", "error", err.Error())
		return common.Hash{}, err
	}

	// Encode transaction by default Tx encoder
	txEncoder := e.clientCtx.TxConfig.TxEncoder()
	txBytes, err := txEncoder(tx)
	if err != nil {
		e.logger.Error("failed to encode eth tx using default encoder", "error", err.Error())
		return common.Hash{}, err
	}

	txHash := msg.AsTransaction().Hash()

	// Broadcast transaction in sync mode (default)
	// NOTE: If error is encountered on the node, the broadcast will not return an error
	syncCtx := e.clientCtx.WithBroadcastMode(flags.BroadcastSync)
	rsp, err := syncCtx.BroadcastTx(txBytes)
	if rsp != nil && rsp.Code != 0 {
		err = sdkerrors.ABCIError(rsp.Codespace, rsp.Code, rsp.RawLog)
	}
	if err != nil {
		e.logger.Error("failed to broadcast tx", "error", err.Error())
		return txHash, err
	}

	// Return transaction hash
	return txHash, nil
}

// EstimateGas returns an estimate of gas usage for the given smart contract call.
func (e *EVMBackend) EstimateGas(args evmtypes.TransactionArgs, blockNrOptional *types.BlockNumber) (hexutil.Uint64, error) {
	blockNr := types.EthPendingBlockNumber
	if blockNrOptional != nil {
		blockNr = *blockNrOptional
	}

	bz, err := json.Marshal(&args)
	if err != nil {
		return 0, err
	}

	req := evmtypes.EthCallRequest{
		Args:   bz,
		GasCap: e.RPCGasCap(),
	}

	// From ContextWithHeight: if the provided height is 0,
	// it will return an empty context and the gRPC query will use
	// the latest block height for querying.
	res, err := e.queryClient.EstimateGas(types.ContextWithHeight(blockNr.Int64()), &req)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(res.Gas), nil
}

// GetTransactionCount returns the number of transactions at the given address up to the given block number.
func (e *EVMBackend) GetTransactionCount(address common.Address, blockNum types.BlockNumber) (*hexutil.Uint64, error) {
	// Get nonce (sequence) from account
	from := sdk.AccAddress(address.Bytes())
	accRet := e.clientCtx.AccountRetriever

	err := accRet.EnsureExists(e.clientCtx, from)
	if err != nil {
		// account doesn't exist yet, return 0
		n := hexutil.Uint64(0)
		return &n, nil
	}

	includePending := blockNum == types.EthPendingBlockNumber
	nonce, err := e.getAccountNonce(address, includePending, blockNum.Int64(), e.logger)
	if err != nil {
		return nil, err
	}

	n := hexutil.Uint64(nonce)
	return &n, nil
}

// RPCGasCap is the global gas cap for eth-call variants.
func (e *EVMBackend) RPCGasCap() uint64 {
	return e.cfg.JSONRPC.GasCap
}

// RPCEVMTimeout is the global evm timeout for eth-call variants.
func (e *EVMBackend) RPCEVMTimeout() time.Duration {
	return e.cfg.JSONRPC.EVMTimeout
}

// RPCGasCap is the global gas cap for eth-call variants.
func (e *EVMBackend) RPCTxFeeCap() float64 {
	return e.cfg.JSONRPC.TxFeeCap
}

// RPCFilterCap is the limit for total number of filters that can be created
func (e *EVMBackend) RPCFilterCap() int32 {
	return e.cfg.JSONRPC.FilterCap
}

// RPCFeeHistoryCap is the limit for total number of blocks that can be fetched
func (e *EVMBackend) RPCFeeHistoryCap() int32 {
	return e.cfg.JSONRPC.FeeHistoryCap
}

// RPCLogsCap defines the max number of results can be returned from single `eth_getLogs` query.
func (e *EVMBackend) RPCLogsCap() int32 {
	return e.cfg.JSONRPC.LogsCap
}

// RPCBlockRangeCap defines the max block range allowed for `eth_getLogs` query.
func (e *EVMBackend) RPCBlockRangeCap() int32 {
	return e.cfg.JSONRPC.BlockRangeCap
}

// RPCMinGasPrice returns the minimum gas price for a transaction obtained from
// the node config. If set value is 0, it will default to 20.

func (e *EVMBackend) RPCMinGasPrice() int64 {
	evmParams, err := e.queryClient.Params(e.ctx, &evmtypes.QueryParamsRequest{})
	if err != nil {
		return ethermint.DefaultGasPrice
	}

	minGasPrice := e.cfg.GetMinGasPrices()
	amt := minGasPrice.AmountOf(evmParams.Params.EvmDenom).TruncateInt64()
	if amt == 0 {
		return ethermint.DefaultGasPrice
	}

	return amt
}

// ChainConfig return the latest ethereum chain configuration
func (e *EVMBackend) ChainConfig() *params.ChainConfig {
	params, err := e.queryClient.Params(e.ctx, &evmtypes.QueryParamsRequest{})
	if err != nil {
		return nil
	}

	return params.Params.ChainConfig.EthereumConfig(e.chainID)
}

// SuggestGasTipCap returns the suggested tip cap
// Although we don't support tx prioritization yet, but we return a positive value to help client to
// mitigate the base fee changes.
func (e *EVMBackend) SuggestGasTipCap(baseFee *big.Int) (*big.Int, error) {
	if baseFee == nil {
		// london hardfork not enabled or feemarket not enabled
		return big.NewInt(0), nil
	}

	params, err := e.queryClient.FeeMarket.Params(e.ctx, &feemarkettypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	// calculate the maximum base fee delta in current block, assuming all block gas limit is consumed
	// ```
	// GasTarget = GasLimit / ElasticityMultiplier
	// Delta = BaseFee * (GasUsed - GasTarget) / GasTarget / Denominator
	// ```
	// The delta is at maximum when `GasUsed` is equal to `GasLimit`, which is:
	// ```
	// MaxDelta = BaseFee * (GasLimit - GasLimit / ElasticityMultiplier) / (GasLimit / ElasticityMultiplier) / Denominator
	//          = BaseFee * (ElasticityMultiplier - 1) / Denominator
	// ```
	maxDelta := baseFee.Int64() * (int64(params.Params.ElasticityMultiplier) - 1) / int64(params.Params.BaseFeeChangeDenominator)
	if maxDelta < 0 {
		// impossible if the parameter validation passed.
		maxDelta = 0
	}
	return big.NewInt(maxDelta), nil
}

// BaseFee returns the base fee tracked by the Fee Market module.
// If the base fee is not enabled globally, the query returns nil.
// If the London hard fork is not activated at the current height, the query will
// return nil.
func (e *EVMBackend) BaseFee(blockRes *tmrpctypes.ResultBlockResults) (*big.Int, error) {
	// return BaseFee if London hard fork is activated and feemarket is enabled
	res, err := e.queryClient.BaseFee(types.ContextWithHeight(blockRes.Height), &evmtypes.QueryBaseFeeRequest{})
	if err != nil {
		// fallback to parsing from begin blocker event, could happen on pruned nodes.
		// faster to iterate reversely
		for i := len(blockRes.BeginBlockEvents) - 1; i >= 0; i-- {
			evt := blockRes.BeginBlockEvents[i]
			if evt.Type == feemarkettypes.EventTypeFeeMarket && len(evt.Attributes) > 0 {
				baseFee, err := strconv.ParseInt(string(evt.Attributes[0].Value), 10, 64)
				if err == nil {
					return big.NewInt(baseFee), nil
				}
				break
			}
		}
		return nil, err
	}

	if res.BaseFee == nil {
		return nil, nil
	}

	return res.BaseFee.BigInt(), nil
}

// GetEthereumMsgsFromTendermintBlock returns all real MsgEthereumTxs from a Tendermint block.
// It also ensures consistency over the correct txs indexes across RPC endpoints
func (e *EVMBackend) GetEthereumMsgsFromTendermintBlock(block *tmrpctypes.ResultBlock, blockRes *tmrpctypes.ResultBlockResults) []*evmtypes.MsgEthereumTx {
	var result []*evmtypes.MsgEthereumTx

	txResults := blockRes.TxsResults

	for i, tx := range block.Block.Txs {
		// check tx exists on EVM by cross checking with blockResults
		// include the tx that exceeds block gas limit
		if !TxSuccessOrExceedsBlockGasLimit(txResults[i]) {
			e.logger.Debug("invalid tx result code", "cosmos-hash", hexutil.Encode(tx.Hash()))
			continue
		}

		tx, err := e.clientCtx.TxConfig.TxDecoder()(tx)
		if err != nil {
			e.logger.Debug("failed to decode transaction in block", "height", block.Block.Height, "error", err.Error())
			continue
		}

		for _, msg := range tx.GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				continue
			}

			ethMsg.Hash = ethMsg.AsTransaction().Hash().Hex()
			result = append(result, ethMsg)
		}
	}

	return result
}
