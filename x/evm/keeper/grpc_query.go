package keeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/eth/tracers"

	"github.com/palantir/stacktrace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	ethparams "github.com/ethereum/go-ethereum/params"

	ethermint "github.com/tharsis/ethermint/types"
	"github.com/tharsis/ethermint/x/evm/statedb"
	"github.com/tharsis/ethermint/x/evm/types"
)

var _ types.QueryServer = Keeper{}

const (
	defaultTraceTimeout = 5 * time.Second
)

// Account implements the Query/Account gRPC method
func (k Keeper) Account(c context.Context, req *types.QueryAccountRequest) (*types.QueryAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := ethermint.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	addr := common.HexToAddress(req.Address)

	ctx := sdk.UnwrapSDKContext(c)
	acct, err := k.GetAccountOrEmpty(ctx, addr)
	if err != nil {
		return nil, err
	}
	return &types.QueryAccountResponse{
		Balance:  acct.Balance.String(),
		CodeHash: hexutil.Encode(acct.CodeHash),
		Nonce:    acct.Nonce,
	}, nil
}

func (k Keeper) CosmosAccount(c context.Context, req *types.QueryCosmosAccountRequest) (*types.QueryCosmosAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := ethermint.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	ethAddr := common.HexToAddress(req.Address)
	cosmosAddr := sdk.AccAddress(ethAddr.Bytes())

	account := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	res := types.QueryCosmosAccountResponse{
		CosmosAddress: cosmosAddr.String(),
	}

	if account != nil {
		res.Sequence = account.GetSequence()
		res.AccountNumber = account.GetAccountNumber()
	}

	return &res, nil
}

func (k Keeper) ValidatorAccount(c context.Context, req *types.QueryValidatorAccountRequest) (*types.QueryValidatorAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	consAddr, err := sdk.ConsAddressFromBech32(req.ConsAddress)
	if err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	validator, found := k.stakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
	if !found {
		return nil, nil
	}

	accAddr := sdk.AccAddress(validator.GetOperator())

	res := types.QueryValidatorAccountResponse{
		AccountAddress: accAddr.String(),
	}

	account := k.accountKeeper.GetAccount(ctx, accAddr)
	if account != nil {
		res.Sequence = account.GetSequence()
		res.AccountNumber = account.GetAccountNumber()
	}

	return &res, nil
}

// Balance implements the Query/Balance gRPC method
func (k Keeper) Balance(c context.Context, req *types.QueryBalanceRequest) (*types.QueryBalanceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := ethermint.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	balanceInt := k.GetBalance(ctx, common.HexToAddress(req.Address))

	return &types.QueryBalanceResponse{
		Balance: balanceInt.String(),
	}, nil
}

// Storage implements the Query/Storage gRPC method
func (k Keeper) Storage(c context.Context, req *types.QueryStorageRequest) (*types.QueryStorageResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := ethermint.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	address := common.HexToAddress(req.Address)
	key := common.HexToHash(req.Key)

	state := k.GetState(ctx, address, key)
	stateHex := state.Hex()

	return &types.QueryStorageResponse{
		Value: stateHex,
	}, nil
}

// Code implements the Query/Code gRPC method
func (k Keeper) Code(c context.Context, req *types.QueryCodeRequest) (*types.QueryCodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := ethermint.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	address := common.HexToAddress(req.Address)
	acct, err := k.GetAccountOrEmpty(ctx, address)
	if err != nil {
		return nil, err
	}
	code := k.GetCode(ctx, common.BytesToHash(acct.CodeHash))

	return &types.QueryCodeResponse{
		Code: code,
	}, nil
}

// Params implements the Query/Params gRPC method
func (k Keeper) Params(c context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	params := k.GetParams(ctx)

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// EthCall implements eth_call rpc api.
func (k Keeper) EthCall(c context.Context, req *types.EthCallRequest) (*types.MsgEthereumTxResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	var args types.CallArgs
	err := json.Unmarshal(req.Args, &args)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	msg := args.ToMessage(req.GasCap)

	// pass false to not commit StateDB
	res, err := k.ApplyMessage(ctx, msg, nil, false)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

// EstimateGas implements eth_estimateGas rpc api.
func (k Keeper) EstimateGas(c context.Context, req *types.EthCallRequest) (*types.EstimateGasResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	if req.GasCap < ethparams.TxGas {
		return nil, status.Error(codes.InvalidArgument, "gas cap cannot be lower than 21,000")
	}

	var args types.CallArgs
	err := json.Unmarshal(req.Args, &args)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  = ethparams.TxGas - 1
		hi  uint64
		cap uint64
	)

	// Determine the highest gas limit can be used during the estimation.
	if args.Gas != nil && uint64(*args.Gas) >= ethparams.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Query block gas limit
		params := ctx.ConsensusParams()
		if params != nil && params.Block != nil && params.Block.MaxGas > 0 {
			hi = uint64(params.Block.MaxGas)
		} else {
			hi = req.GasCap
		}
	}

	// TODO: Recap the highest gas limit with account's available balance.

	// Recap the highest gas allowance with specified gascap.
	if req.GasCap != 0 && hi > req.GasCap {
		hi = req.GasCap
	}
	cap = hi

	cfg, err := k.EVMConfig(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load evm config")
	}

	txConfig := statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash().Bytes()))

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (bool, *types.MsgEthereumTxResponse, error) {
		args.Gas = (*hexutil.Uint64)(&gas)

		msg := args.ToMessage(req.GasCap)

		// pass false to not commit StateDB
		rsp, err := k.ApplyMessageWithConfig(ctx, msg, nil, false, cfg, txConfig)
		if err != nil {
			if errors.Is(stacktrace.RootCause(err), core.ErrIntrinsicGas) {
				return true, nil, nil // Special case, raise gas limit
			}
			return true, nil, err // Bail out
		}
		return len(rsp.VmError) > 0, rsp, nil
	}

	// Execute the binary search and hone in on an executable gas limit
	hi, err = types.BinSearch(lo, hi, executable)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		failed, result, err := executable(hi)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if failed {
			if result != nil && result.VmError != vm.ErrOutOfGas.Error() {
				if result.VmError == vm.ErrExecutionReverted.Error() {
					return nil, types.NewExecErrorWithReason(result.Ret)
				}
				return nil, status.Error(codes.Internal, result.VmError)
			}
			// Otherwise, the specified gas cap is too low
			return nil, status.Error(codes.Internal, fmt.Sprintf("gas required exceeds allowance (%d)", cap))
		}
	}
	return &types.EstimateGasResponse{Gas: hi}, nil
}

// TraceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer dependent.
func (k Keeper) TraceTx(c context.Context, req *types.QueryTraceTxRequest) (*types.QueryTraceTxResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)
	ctx = ctx.WithBlockHeight(req.BlockNumber)
	ctx = ctx.WithBlockTime(req.BlockTime)
	ctx = ctx.WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	cfg, err := k.EVMConfig(ctx)
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to load evm config")
	}
	signer := ethtypes.MakeSigner(cfg.ChainConfig, big.NewInt(ctx.BlockHeight()))

	for _, tx := range req.Predecessors {
		ethTx := tx.AsTransaction()
		msg, err := ethTx.AsMessage(signer)
		if err != nil {
			continue
		}
		txConfig := statedb.NewEmptyTxConfig(
			common.BytesToHash(ctx.HeaderHash()),
		)
		_, err = k.ApplyMessageWithConfig(ctx, msg, types.NewNoOpTracer(), true, cfg, txConfig)
		if err != nil {
			continue
		}
	}

	tx := req.Msg.AsTransaction()
	msg, err := tx.AsMessage(signer)
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to return ethereum transaction as core message")
	}
	txConfig := statedb.NewTxConfig(
		common.BytesToHash(ctx.HeaderHash()), // BlockHash
		tx.Hash(),                            // TxHash
		uint(req.TxIndex),                    // TxIndex
		uint(0),                              // LogIndex
	)
	result, _, err := k.traceTx(ctx, msg, cfg, txConfig, req.TraceConfig)
	if err != nil {
		// error will be returned with detail status from traceTx
		return nil, err
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryTraceTxResponse{
		Data: resultData,
	}, nil
}

// TraceBlock configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment for all the transactions in the queried block.
// The return value will be tracer dependent.
func (k Keeper) TraceBlock(c context.Context, req *types.QueryTraceBlockRequest) (*types.QueryTraceBlockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if req.TraceConfig != nil && req.TraceConfig.Limit < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "output limit cannot be negative, got %d", req.TraceConfig.Limit)
	}

	ctx := sdk.UnwrapSDKContext(c)
	ctx = ctx.WithBlockHeight(req.BlockNumber)
	ctx = ctx.WithBlockTime(req.BlockTime)
	ctx = ctx.WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	cfg, err := k.EVMConfig(ctx)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}
	ethCfg := cfg.Params.ChainConfig.EthereumConfig(k.eip155ChainID)
	signer := ethtypes.MakeSigner(ethCfg, big.NewInt(ctx.BlockHeight()))

	txsLength := len(req.Txs)
	results := make([]*types.TxTraceResult, 0, txsLength)

	var logIndex uint
	for i, tx := range req.Txs {
		result := types.TxTraceResult{}
		ethTx := tx.AsTransaction()
		msg, err := ethTx.AsMessage(signer)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, err.Error())
		}
		txConfig := statedb.NewTxConfig(
			common.BytesToHash(ctx.HeaderHash()), // BlockHash
			ethTx.Hash(),                         // TxHash
			uint(i),                              // TxIndex
			uint(logIndex),                       // LogIndex
		)
		result.Result, logIndex, err = k.traceTx(ctx, msg, cfg, txConfig, req.TraceConfig)
		if err != nil {
			result.Error = err.Error()
			continue
		}
		results = append(results, &result)
	}

	resultData, err := json.Marshal(results)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryTraceBlockResponse{
		Data: resultData,
	}, nil
}

func (k *Keeper) traceTx(
	ctx sdk.Context,
	msg core.Message,
	cfg *types.EVMConfig,
	txConfig statedb.TxConfig,
	traceConfig *types.TraceConfig,
) (*interface{}, uint, error) {
	// Assemble the structured logger or the JavaScript tracer
	var (
		tracer vm.Tracer
		err    error
	)

	switch {
	case traceConfig != nil && traceConfig.Tracer != "":
		timeout := defaultTraceTimeout
		// TODO: change timeout to time.duration
		// Used string to comply with go ethereum
		if traceConfig.Timeout != "" {
			timeout, err = time.ParseDuration(traceConfig.Timeout)
			if err != nil {
				return nil, 0, status.Errorf(codes.InvalidArgument, "timeout value: %s", err.Error())
			}
		}

		txContext := core.NewEVMTxContext(msg)

		// Construct the JavaScript tracer to execute with
		tracer, err = tracers.New(traceConfig.Tracer, txContext)
		if err != nil {
			return nil, 0, status.Error(codes.Internal, err.Error())
		}

		// Handle timeouts and RPC cancellations
		deadlineCtx, cancel := context.WithTimeout(ctx.Context(), timeout)
		defer cancel()

		go func() {
			<-deadlineCtx.Done()
			if errors.Is(deadlineCtx.Err(), context.DeadlineExceeded) {
				tracer.(*tracers.Tracer).Stop(errors.New("execution timeout"))
			}
		}()

	case traceConfig != nil:
		logConfig := vm.LogConfig{
			DisableMemory:  traceConfig.DisableMemory,
			Debug:          traceConfig.Debug,
			DisableStorage: traceConfig.DisableStorage,
			DisableStack:   traceConfig.DisableStack,
		}
		tracer = vm.NewStructLogger(&logConfig)
	default:
		tracer = types.NewTracer(types.TracerStruct, msg, cfg.ChainConfig, ctx.BlockHeight(), true)
	}

	res, err := k.ApplyMessageWithConfig(ctx, msg, tracer, true, cfg, txConfig)
	if err != nil {
		return nil, 0, status.Error(codes.Internal, err.Error())
	}

	var result interface{}

	// Depending on the tracer type, format and return the trace result data.
	switch tracer := tracer.(type) {
	case *vm.StructLogger:
		// TODO: Return proper returnValue
		result = types.ExecutionResult{
			Gas:         res.GasUsed,
			Failed:      res.Failed(),
			ReturnValue: "",
			StructLogs:  types.FormatLogs(tracer.StructLogs()),
		}
	case *tracers.Tracer:
		result, err = tracer.GetResult()
		if err != nil {
			return nil, 0, status.Error(codes.Internal, err.Error())
		}

	default:
		return nil, 0, status.Errorf(codes.InvalidArgument, "invalid tracer type %T", tracer)
	}

	return &result, txConfig.LogIndex + uint(len(res.Logs)), nil
}
