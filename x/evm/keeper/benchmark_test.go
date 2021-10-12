package keeper_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/ethereum/go-ethereum/common"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethermint "github.com/tharsis/ethermint/types"
	"github.com/tharsis/ethermint/x/evm/types"
)

func SetupContract(b *testing.B) (*KeeperTestSuite, common.Address) {
	suite := KeeperTestSuite{}
	suite.DoSetupTest(b)

	amt := sdk.Coins{ethermint.NewPhotonCoinInt64(1000000000000000000)}
	err := suite.app.BankKeeper.MintCoins(suite.ctx, types.ModuleName, amt)
	require.NoError(b, err)
	err = suite.app.BankKeeper.SendCoinsFromModuleToAccount(suite.ctx, types.ModuleName, suite.address.Bytes(), amt)
	require.NoError(b, err)

	contractAddr := suite.DeployTestContract(b, suite.address, sdk.NewIntWithDecimal(1000, 18).BigInt())
	suite.Commit()

	return &suite, contractAddr
}

type TxBuilder func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx

func DoBenchmark(b *testing.B, txBuilder TxBuilder) {
	suite, contractAddr := SetupContract(b)

	msg := txBuilder(suite, contractAddr)
	msg.From = suite.address.Hex()
	err := msg.Sign(ethtypes.LatestSignerForChainID(suite.app.EvmKeeper.ChainID()), suite.signer)
	require.NoError(b, err)

	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		ctx, _ := suite.ctx.CacheContext()

		// deduct fee first
		txData, err := types.UnpackTxData(msg.Data)
		require.NoError(b, err)

		fees := sdk.Coins{sdk.NewCoin(suite.EvmDenom(), sdk.NewIntFromBigInt(txData.Fee()))}
		err = authante.DeductFees(suite.app.BankKeeper, suite.ctx, suite.app.AccountKeeper.GetAccount(ctx, msg.GetFrom()), fees)
		require.NoError(b, err)

		rsp, err := suite.app.EvmKeeper.EthereumTx(sdk.WrapSDKContext(ctx), msg)
		require.NoError(b, err)
		require.False(b, rsp.Failed())
	}
}

func BenchmarkTokenTransfer(b *testing.B) {
	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := ContractABI.Pack("transfer", common.HexToAddress("0x378c50D9264C63F3F92B806d4ee56E9D86FfB3Ec"), big.NewInt(1000))
		require.NoError(b, err)
		nonce := suite.vmdb.GetNonce(suite.address)
		return types.NewTx(suite.app.EvmKeeper.ChainID(), nonce, &contract, big.NewInt(0), 410000, big.NewInt(1), input, nil)
	})
}

func BenchmarkEmitLogs(b *testing.B) {
	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := ContractABI.Pack("benchmarkLogs", big.NewInt(1000))
		require.NoError(b, err)
		nonce := suite.vmdb.GetNonce(suite.address)
		return types.NewTx(suite.app.EvmKeeper.ChainID(), nonce, &contract, big.NewInt(0), 4100000, big.NewInt(1), input, nil)
	})
}

func BenchmarkTokenTransferFrom(b *testing.B) {
	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := ContractABI.Pack("transferFrom", suite.address, common.HexToAddress("0x378c50D9264C63F3F92B806d4ee56E9D86FfB3Ec"), big.NewInt(0))
		require.NoError(b, err)
		nonce := suite.vmdb.GetNonce(suite.address)
		return types.NewTx(suite.app.EvmKeeper.ChainID(), nonce, &contract, big.NewInt(0), 410000, big.NewInt(1), input, nil)
	})
}

func BenchmarkTokenMint(b *testing.B) {
	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := ContractABI.Pack("mint", common.HexToAddress("0x378c50D9264C63F3F92B806d4ee56E9D86FfB3Ec"), big.NewInt(1000))
		require.NoError(b, err)
		nonce := suite.vmdb.GetNonce(suite.address)
		return types.NewTx(suite.app.EvmKeeper.ChainID(), nonce, &contract, big.NewInt(0), 410000, big.NewInt(1), input, nil)
	})
}

func DoBenchmarkDeepContextStack(b *testing.B, depth int) {
	begin := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	end := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	suite := KeeperTestSuite{}
	suite.DoSetupTest(b)

	transientKey := suite.app.GetTKey(types.TransientKey)

	stack := types.NewContextStack(suite.ctx)

	for i := 0; i < depth; i++ {
		stack.Snapshot()

		store := stack.CurrentContext().TransientStore(transientKey)
		store.Set(begin, []byte("value"))
	}

	store := stack.CurrentContext().TransientStore(transientKey)
	for i := 0; i < b.N; i++ {
		store.Iterator(begin, end)
	}
}

func BenchmarkDeepContextStack1(b *testing.B) {
	DoBenchmarkDeepContextStack(b, 1)
}

func BenchmarkDeepContextStack10(b *testing.B) {
	DoBenchmarkDeepContextStack(b, 10)
}

func BenchmarkDeepContextStack13(b *testing.B) {
	DoBenchmarkDeepContextStack(b, 13)
}
