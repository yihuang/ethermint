package keeper

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/tendermint/tendermint/libs/log"

	ethermint "github.com/tharsis/ethermint/types"
	"github.com/tharsis/ethermint/x/evm/statedb"
	"github.com/tharsis/ethermint/x/evm/types"
)

var (
	_             statedb.Keeper = &Keeper{}
	emptyCodeHash                = crypto.Keccak256(nil)
)

// Keeper grants access to the EVM module state and implements the go-ethereum StateDB interface.
type Keeper struct {
	// Protobuf codec
	cdc codec.BinaryCodec
	// Store key required for the EVM Prefix KVStore. It is required by:
	// - storing account's Storage State
	// - storing account's Code
	// - storing transaction Logs
	// - storing Bloom filters by block height. Needed for the Web3 API.
	storeKey sdk.StoreKey

	// key to access the transient store, which is reset on every block during Commit
	transientKey sdk.StoreKey

	// module specific parameter space that can be configured through governance
	paramSpace paramtypes.Subspace
	// access to account state
	accountKeeper types.AccountKeeper
	// update balance and accounting operations with coins
	bankKeeper types.BankKeeper
	// access historical headers for EVM state transition execution
	stakingKeeper types.StakingKeeper

	// chain ID number obtained from the context's chain id
	eip155ChainID *big.Int

	// Tracer used to collect execution traces from the EVM transaction execution
	tracer string
	// trace EVM state transition execution. This value is obtained from the `--trace` flag.
	// For more info check https://geth.ethereum.org/docs/dapp/tracing
	debug bool

	// EVM Hooks for tx post-processing
	hooks types.EvmHooks
}

// NewKeeper generates new evm module keeper
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey, transientKey sdk.StoreKey, paramSpace paramtypes.Subspace,
	ak types.AccountKeeper, bankKeeper types.BankKeeper, sk types.StakingKeeper,
	tracer string, debug bool,
) *Keeper {
	// ensure evm module account is set
	if addr := ak.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the EVM module account has not been set")
	}

	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	// NOTE: we pass in the parameter space to the CommitStateDB in order to use custom denominations for the EVM operations
	return &Keeper{
		cdc:           cdc,
		paramSpace:    paramSpace,
		accountKeeper: ak,
		bankKeeper:    bankKeeper,
		stakingKeeper: sk,
		storeKey:      storeKey,
		transientKey:  transientKey,
		tracer:        tracer,
		debug:         debug,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", types.ModuleName)
}

// WithChainID sets the chain id to the local variable in the keeper
func (k *Keeper) WithChainID(ctx sdk.Context) {
	chainID, err := ethermint.ParseChainID(ctx.ChainID())
	if err != nil {
		panic(err)
	}

	if k.eip155ChainID != nil && k.eip155ChainID.Cmp(chainID) != 0 {
		panic("chain id already set")
	}

	k.eip155ChainID = chainID
}

// ChainID returns the EIP155 chain ID for the EVM context
func (k Keeper) ChainID() *big.Int {
	return k.eip155ChainID
}

// ----------------------------------------------------------------------------
// Block Bloom
// Required by Web3 API.
// ----------------------------------------------------------------------------

// EmitBlockBloomEvent emit block bloom events
func (k Keeper) EmitBlockBloomEvent(ctx sdk.Context, bloom ethtypes.Bloom) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBlockBloom,
			sdk.NewAttribute(types.AttributeKeyEthereumBloom, string(bloom.Bytes())),
		),
	)
}

// GetBlockBloomTransient returns bloom bytes for the current block height
func (k Keeper) GetBlockBloomTransient(ctx sdk.Context) *big.Int {
	store := prefix.NewStore(ctx.TransientStore(k.transientKey), types.KeyPrefixTransientBloom)
	heightBz := sdk.Uint64ToBigEndian(uint64(ctx.BlockHeight()))
	bz := store.Get(heightBz)
	if len(bz) == 0 {
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(bz)
}

// SetBlockBloomTransient sets the given bloom bytes to the transient store. This value is reset on
// every block.
func (k Keeper) SetBlockBloomTransient(ctx sdk.Context, bloom *big.Int) {
	store := prefix.NewStore(ctx.TransientStore(k.transientKey), types.KeyPrefixTransientBloom)
	heightBz := sdk.Uint64ToBigEndian(uint64(ctx.BlockHeight()))
	store.Set(heightBz, bloom.Bytes())
}

// ----------------------------------------------------------------------------
// Tx
// ----------------------------------------------------------------------------

// SetTxIndexTransient set the index of processing transaction
func (k Keeper) SetTxIndexTransient(ctx sdk.Context, index uint64) {
	store := ctx.TransientStore(k.transientKey)
	store.Set(types.KeyPrefixTransientTxIndex, sdk.Uint64ToBigEndian(index))
}

// GetTxIndexTransient returns EVM transaction index on the current block.
func (k Keeper) GetTxIndexTransient(ctx sdk.Context) uint64 {
	store := ctx.TransientStore(k.transientKey)
	bz := store.Get(types.KeyPrefixTransientTxIndex)
	if len(bz) == 0 {
		return 0
	}

	return sdk.BigEndianToUint64(bz)
}

// ----------------------------------------------------------------------------
// Log
// ----------------------------------------------------------------------------

// GetLogSizeTransient returns EVM log index on the current block.
func (k Keeper) GetLogSizeTransient(ctx sdk.Context) uint64 {
	store := ctx.TransientStore(k.transientKey)
	bz := store.Get(types.KeyPrefixTransientLogSize)
	if len(bz) == 0 {
		return 0
	}

	return sdk.BigEndianToUint64(bz)
}

// SetLogSizeTransient fetches the current EVM log index from the transient store, increases its
// value by one and then sets the new index back to the transient store.
func (k Keeper) SetLogSizeTransient(ctx sdk.Context, logSize uint64) {
	store := ctx.TransientStore(k.transientKey)
	store.Set(types.KeyPrefixTransientLogSize, sdk.Uint64ToBigEndian(logSize))
}

// ----------------------------------------------------------------------------
// Storage
// ----------------------------------------------------------------------------

// GetAccountStorage return state storage associated with an account
func (k Keeper) GetAccountStorage(ctx sdk.Context, address common.Address) types.Storage {
	storage := types.Storage{}

	k.ForEachStorage(ctx, address, func(key, value common.Hash) bool {
		storage = append(storage, types.NewState(key, value))
		return true
	})

	return storage
}

// ----------------------------------------------------------------------------
// Account
// ----------------------------------------------------------------------------

// SetHooks sets the hooks for the EVM module
func (k *Keeper) SetHooks(eh types.EvmHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set evm hooks twice")
	}

	k.hooks = eh
	return k
}

// PostTxProcessing delegate the call to the hooks. If no hook has been registered, this function returns with a `nil` error
func (k *Keeper) PostTxProcessing(ctx sdk.Context, txHash common.Hash, logs []*ethtypes.Log) error {
	if k.hooks == nil {
		return nil
	}
	return k.hooks.PostTxProcessing(ctx, txHash, logs)
}

// Tracer return a default vm.Tracer based on current keeper state
func (k Keeper) Tracer(ctx sdk.Context, msg core.Message, ethCfg *params.ChainConfig) vm.Tracer {
	return types.NewTracer(k.tracer, msg, ethCfg, ctx.BlockHeight(), k.debug)
}

// ----------------------------------------------------------------------------
// statedb.Keeper implementation
// ----------------------------------------------------------------------------

func (k *Keeper) GetBalance(ctx sdk.Context, addr common.Address) *big.Int {
	cosmosAddr := sdk.AccAddress(addr.Bytes())
	params := k.GetParams(ctx)
	coin := k.bankKeeper.GetBalance(ctx, cosmosAddr, params.EvmDenom)
	return coin.Amount.BigInt()
}

func (k *Keeper) GetAccount(ctx sdk.Context, addr common.Address) (*statedb.Account, error) {
	cosmosAddr := sdk.AccAddress(addr.Bytes())
	acct := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	if acct == nil {
		return nil, nil
	}

	ethAcct, ok := acct.(*ethermint.EthAccount)
	if !ok {
		return nil, errors.New("not EthAccount")
	}

	balance := k.GetBalance(ctx, addr)
	return &statedb.Account{
		Nonce:    ethAcct.Sequence,
		Balance:  balance,
		CodeHash: common.FromHex(ethAcct.CodeHash),
	}, nil
}

func (k *Keeper) GetAccountOrEmpty(ctx sdk.Context, addr common.Address) (statedb.Account, error) {
	acct, err := k.GetAccount(ctx, addr)
	if err != nil {
		return statedb.Account{}, err
	}
	if acct == nil {
		// empty account
		return statedb.Account{
			Balance:  new(big.Int),
			CodeHash: emptyCodeHash,
		}, nil
	}
	return *acct, nil
}

func (k *Keeper) GetState(ctx sdk.Context, addr common.Address, key common.Hash) common.Hash {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.AddressStoragePrefix(addr))

	value := store.Get(key.Bytes())
	if len(value) == 0 {
		return common.Hash{}
	}

	return common.BytesToHash(value)
}

func (k *Keeper) GetCode(ctx sdk.Context, codeHash common.Hash) []byte {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixCode)
	return store.Get(codeHash.Bytes())
}

func (k *Keeper) GetCodeSize(ctx sdk.Context, codeHash common.Hash) int {
	return len(k.GetCode(ctx, codeHash))
}

// ForEachStorage iterate contract storage, callback return false to break early
func (k *Keeper) ForEachStorage(ctx sdk.Context, addr common.Address, cb func(key, value common.Hash) bool) {
	store := ctx.KVStore(k.storeKey)
	prefix := types.AddressStoragePrefix(addr)

	iterator := sdk.KVStorePrefixIterator(store, prefix)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		// TODO: check if the key prefix needs to be trimmed
		key := common.BytesToHash(iterator.Key())
		value := common.BytesToHash(iterator.Value())

		// check if iteration stops
		if !cb(key, value) {
			return
		}
	}
}

// SetBalance update account's balance, compare with current balance first, then decide to mint or burn.
func (k *Keeper) SetBalance(ctx sdk.Context, addr common.Address, amount *big.Int) error {
	cosmosAddr := sdk.AccAddress(addr.Bytes())

	params := k.GetParams(ctx)
	coin := k.bankKeeper.GetBalance(ctx, cosmosAddr, params.EvmDenom)
	balance := coin.Amount.BigInt()
	delta := new(big.Int).Sub(amount, balance)
	switch delta.Sign() {
	case 1:
		// mint
		coins := sdk.NewCoins(sdk.NewCoin(params.EvmDenom, sdk.NewIntFromBigInt(delta)))
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
			return err
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, cosmosAddr, coins); err != nil {
			return err
		}
	case -1:
		// burn
		coins := sdk.NewCoins(sdk.NewCoin(params.EvmDenom, sdk.NewIntFromBigInt(new(big.Int).Neg(delta))))
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, cosmosAddr, types.ModuleName, coins); err != nil {
			return err
		}
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins); err != nil {
			return err
		}
	default:
		// not changed
	}
	return nil
}

// SetAccount updates nonce/balance/codeHash together.
func (k *Keeper) SetAccount(ctx sdk.Context, addr common.Address, account statedb.Account) error {
	// update account
	cosmosAddr := sdk.AccAddress(addr.Bytes())
	acct := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	if acct == nil {
		acct = k.accountKeeper.NewAccountWithAddress(ctx, cosmosAddr)
	}
	ethAcct, ok := acct.(*ethermint.EthAccount)
	if !ok {
		return errors.New("not EthAccount")
	}
	if err := ethAcct.SetSequence(account.Nonce); err != nil {
		return err
	}
	ethAcct.CodeHash = common.Bytes2Hex(account.CodeHash)
	k.accountKeeper.SetAccount(ctx, ethAcct)

	return k.SetBalance(ctx, addr, account.Balance)
}

func (k *Keeper) SetState(ctx sdk.Context, addr common.Address, key, value common.Hash) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.AddressStoragePrefix(addr))
	if len(value) == 0 {
		store.Delete(key.Bytes())
	} else {
		store.Set(key.Bytes(), value.Bytes())
	}
}

func (k *Keeper) SetCode(ctx sdk.Context, codeHash []byte, code []byte) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixCode)

	// store or delete code
	if len(code) == 0 {
		store.Delete(codeHash)
	} else {
		store.Set(codeHash, code)
	}
}

// PurgeAccount handles contract's suicide call:
// - clear balance
// - remove code
// - remove states
func (k *Keeper) PurgeAccount(ctx sdk.Context, addr common.Address) error {
	acct, err := k.GetAccount(ctx, addr)
	if err != nil {
		return err
	}

	if acct == nil {
		return nil
	}

	// clear balance
	if acct.Balance.Sign() != 0 {
		if err := k.SetBalance(ctx, addr, new(big.Int)); err != nil {
			return err
		}
	}

	// remove code
	if !bytes.Equal(acct.CodeHash, emptyCodeHash) {
		k.SetCode(ctx, acct.CodeHash, []byte{})
	}

	// clear storage
	var keysToDelete []common.Hash
	k.ForEachStorage(ctx, addr, func(key, value common.Hash) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	})
	for _, key := range keysToDelete {
		k.SetState(ctx, addr, key, common.Hash{})
	}

	// FIXME should we remove account?
	return nil
}
