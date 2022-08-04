package precompiles

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctransferkeeper "github.com/cosmos/ibc-go/v3/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcchannelkeeper "github.com/cosmos/ibc-go/v3/modules/core/04-channel/keeper"
	ibcchanneltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/evmos/ethermint/x/evm/statedb"
	"github.com/evmos/ethermint/x/evm/types"
)

var (
	TransferMethod     abi.Method
	HasCommitMethod    abi.Method
	QueryNextSeqMethod abi.Method

	_ statedb.StatefulPrecompiledContract = (*IbcContract)(nil)
	_ statedb.JournalEntry                = ibcMessageChange{}
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	stringType, _ := abi.NewType("string", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	boolType, _ := abi.NewType("bool", "", nil)
	TransferMethod = abi.NewMethod(
		"transfer", "transfer", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "portID",
			Type: stringType,
		}, abi.Argument{
			Name: "channelID",
			Type: stringType,
		}, abi.Argument{
			Name: "srcDenom",
			Type: stringType,
		}, abi.Argument{
			Name: "dstDenom",
			Type: stringType,
		}, abi.Argument{
			Name: "ratio",
			Type: uint256Type,
		}, abi.Argument{
			Name: "timeout",
			Type: uint256Type,
		}, abi.Argument{
			Name: "sender",
			Type: addressType,
		}, abi.Argument{
			Name: "receiver",
			Type: stringType,
		}, abi.Argument{
			Name: "amount",
			Type: uint256Type,
		}},
		abi.Arguments{abi.Argument{
			Name: "sequence",
			Type: uint256Type,
		}},
	)
	HasCommitMethod = abi.NewMethod(
		"hasCommit", "hasCommit", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "portID",
			Type: stringType,
		}, abi.Argument{
			Name: "channelID",
			Type: stringType,
		}, abi.Argument{
			Name: "sequence",
			Type: uint256Type,
		}},
		abi.Arguments{abi.Argument{
			Name: "status",
			Type: boolType,
		}},
	)
	QueryNextSeqMethod = abi.NewMethod(
		"queryNextSeq", "queryNextSeq", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "portID",
			Type: stringType,
		}, abi.Argument{
			Name: "channelID",
			Type: stringType,
		}},
		abi.Arguments{abi.Argument{
			Name: "sequence",
			Type: uint256Type,
		}},
	)
}

func (msg ibcMessage) Changed() *big.Int {
	return new(big.Int).Sub(msg.dirtyAmount, msg.originAmount)
}

type IbcContract struct {
	ctx            sdk.Context
	channelKeeper  *ibcchannelkeeper.Keeper
	transferKeeper *ibctransferkeeper.Keeper
	bankKeeper     types.BankKeeper
	msgs           map[common.Address]map[common.Address]*ibcMessage
	module         string
}

func NewIbcContractCreator(channelKeeper *ibcchannelkeeper.Keeper, transferKeeper *ibctransferkeeper.Keeper, bankKeeper types.BankKeeper, module string) statedb.PrecompiledContractCreator {
	return func(ctx sdk.Context) statedb.StatefulPrecompiledContract {
		msgs := make(map[common.Address]map[common.Address]*ibcMessage)
		return &IbcContract{ctx, channelKeeper, transferKeeper, bankKeeper, msgs, module}
	}
}

// RequiredGas calculates the contract gas use
func (ic *IbcContract) RequiredGas(input []byte) uint64 {
	// TODO estimate required gas
	return 0
}

func (ic *IbcContract) Run(evm *vm.EVM, input []byte, caller common.Address, value *big.Int, readonly bool) ([]byte, error) {
	stateDB, ok := evm.StateDB.(ExtStateDB)
	if !ok {
		return nil, errors.New("not run in ethermint")
	}
	methodID := input[:4]
	switch string(methodID) {
	case string(TransferMethod.ID):
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := TransferMethod.Inputs.Unpack(input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		portID := args[0].(string)
		channelID := args[1].(string)
		srcDenom := args[2].(string)
		dstDenom := args[3].(string)
		ratio := args[4].(*big.Int)
		timeout := args[5].(*big.Int)
		sender := args[6].(common.Address)
		receiver := args[7].(string)
		amount := args[8].(*big.Int)
		if amount.Sign() <= 0 {
			return nil, errors.New("invalid amount")
		}
		timeoutTimestamp := uint64(ic.ctx.BlockTime().UnixNano()) + timeout.Uint64()
		timeoutHeight := clienttypes.ZeroHeight()
		fmt.Printf(
			"TransferMethod portID: %s, channelID: %s, sender:%s, receiver: %s, amount: %s, srcDenom: %s, dstDenom: %s, timeoutTimestamp: %d, timeoutHeight: %s\n",
			portID, channelID, sender, receiver, amount.String(), srcDenom, dstDenom, timeoutTimestamp, timeoutHeight,
		)
		token := sdk.NewCoin(srcDenom, sdk.NewInt(amount.Int64()))
		src := sdk.AccAddress(common.HexToAddress(sender.String()).Bytes())
		if err != nil {
			return nil, err
		}
		transfer := &ibctransfertypes.MsgTransfer{
			SourcePort:       portID,
			SourceChannel:    channelID,
			Token:            token,
			Sender:           src.String(),
			Receiver:         receiver,
			TimeoutHeight:    timeoutHeight,
			TimeoutTimestamp: timeoutTimestamp,
		}
		if _, ok := ic.msgs[caller]; !ok {
			ic.msgs[caller] = make(map[common.Address]*ibcMessage)
		}
		msgs := ic.msgs[caller]
		msg, ok := msgs[sender]
		if ok {
			msg.dirtyAmount = new(big.Int).Sub(msg.dirtyAmount, amount)
		} else {
			// query original amount
			addr := sdk.AccAddress(sender.Bytes())
			originAmount := ic.bankKeeper.GetBalance(ic.ctx, addr, srcDenom).Amount.BigInt()
			dirtyAmount := new(big.Int).Sub(originAmount, amount)
			msg = &ibcMessage{transfer, dstDenom, ratio, originAmount, dirtyAmount}
			msgs[sender] = msg
		}
		stateDB.AppendJournalEntry(ibcMessageChange{ic, caller, sender, msg})
		sequence, _ := ic.channelKeeper.GetNextSequenceSend(ic.ctx, portID, channelID)
		status := ic.channelKeeper.HasPacketCommitment(ic.ctx, portID, channelID, sequence)
		fmt.Printf("TransferMethod sequence: %d, %+v\n", sequence, status)
		return TransferMethod.Outputs.Pack(new(big.Int).SetUint64(sequence))
	case string(HasCommitMethod.ID):
		args, err := HasCommitMethod.Inputs.Unpack(input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		portID := args[0].(string)
		channelID := args[1].(string)
		sequence := args[2].(*big.Int)
		seq := sequence.Uint64()
		fmt.Printf("HasCommitMethod portID: %s, channelID: %s, sequence: %d\n", portID, channelID, seq)
		status := ic.channelKeeper.HasPacketCommitment(ic.ctx, portID, channelID, seq)
		fmt.Printf("HasCommitMethod status: %+v\n", status)
		return HasCommitMethod.Outputs.Pack(status)
	case string(QueryNextSeqMethod.ID):
		args, err := QueryNextSeqMethod.Inputs.Unpack(input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		portID := args[0].(string)
		channelID := args[1].(string)
		fmt.Printf("QueryNextSeqMethod portID: %s, channelID: %s\n", portID, channelID)
		sequence, _ := ic.channelKeeper.GetNextSequenceSend(ic.ctx, portID, channelID)
		fmt.Printf("QueryNextSeqMethod sequence: %d\n", sequence)
		return QueryNextSeqMethod.Outputs.Pack(new(big.Int).SetUint64(sequence))
	default:
		return nil, errors.New("unknown method")
	}
}

func (ic *IbcContract) Commit(ctx sdk.Context) error {
	goCtx := sdk.WrapSDKContext(ic.ctx)
	for _, msgs := range ic.msgs {
		for sender, msg := range msgs {
			acc, err := sdk.AccAddressFromBech32(msg.transfer.Sender)
			if err != nil {
				return err
			}
			c := msg.transfer.Token
			ratio := sdk.NewIntFromBigInt(msg.ratio)
			amount8decRem := c.Amount.Mod(ratio)
			amountToBurn := c.Amount.Sub(amount8decRem)
			if amountToBurn.IsZero() {
				// Amount too small
				continue
			}
			changed := msgs[sender].Changed()
			coins := sdk.NewCoins(sdk.NewCoin(msg.transfer.Token.Denom, amountToBurn))
			amount8dec := c.Amount.Quo(ratio)
			hash := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s", ibctransfertypes.ModuleName, msg.transfer.SourceChannel, msg.dstDenom)))
			ibcDenom := fmt.Sprintf("ibc/%s", strings.ToUpper(hex.EncodeToString(hash[:])))
			ibcCoin := sdk.NewCoin(ibcDenom, amount8dec)
			switch changed.Sign() {
			case -1:
				// Send evm tokens to escrow address
				if err = ic.bankKeeper.SendCoinsFromAccountToModule(
					ctx, acc, ic.module, coins); err != nil {
					return err
				}
				// Burns the evm tokens
				if err := ic.bankKeeper.BurnCoins(
					ctx, ic.module, coins); err != nil {
					return err
				}
				// Transfer ibc tokens back to the user
				if err := ic.bankKeeper.SendCoinsFromModuleToAccount(
					ctx, ic.module, acc, sdk.NewCoins(ibcCoin),
				); err != nil {
					return err
				}
				msg.transfer.Token = ibcCoin
				res, err := ic.transferKeeper.Transfer(goCtx, msg.transfer)
				if err != nil {
					if ibcchanneltypes.ErrPacketTimeout.Is(err) {
						if err := ic.bankKeeper.MintCoins(
							ctx, ic.module, coins); err != nil {
							return err
						}
						if err := ic.bankKeeper.SendCoinsFromModuleToAccount(
							ctx, ic.module, acc, coins); err != nil {
							return err
						}
						return nil
					}
					fmt.Printf("Transfer res: %+v, %+v\n", res, err)
					return err
				}
			case 1:
				// msg.transfer.Token = ibcCoin
				// res, err := ic.transferKeeper.Transfer(goCtx, msg.transfer)
				if err := ic.bankKeeper.SendCoinsFromAccountToModule(
					ctx, acc, ic.module, sdk.NewCoins(ibcCoin),
				); err != nil {
					return err
				}
				if err := ic.bankKeeper.MintCoins(
					ctx, ic.module, coins); err != nil {
					return err
				}
				if err := ic.bankKeeper.SendCoinsFromModuleToAccount(
					ctx, ic.module, acc, coins); err != nil {
					return err
				}
				return nil
			}
		}
	}
	return nil
}

type ibcMessage struct {
	transfer     *ibctransfertypes.MsgTransfer
	dstDenom     string
	ratio        *big.Int
	originAmount *big.Int
	dirtyAmount  *big.Int
}

type ibcMessageChange struct {
	ic     *IbcContract
	caller common.Address
	sender common.Address
	msg    *ibcMessage
}

func (ch ibcMessageChange) Revert(*statedb.StateDB) {
	msg := ch.ic.msgs[ch.caller][ch.sender]
	msg.dirtyAmount = new(big.Int).Add(msg.dirtyAmount, ch.msg.transfer.Token.Amount.BigInt())
}

func (ch ibcMessageChange) Dirtied() *common.Address {
	return nil
}
