package app

import (
	"encoding/json"
	"fmt"

	abci "github.com/tendermint/abci/types"
	oldwire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	bam "github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/ibc"
	"github.com/cosmos/cosmos-sdk/x/simplestake"

	"github.com/cosmos/cosmos-sdk/examples/ethermint/types"
	"github.com/cosmos/cosmos-sdk/examples/ethermint/x/eth"

	"github.com/ethereum/go-ethereum/rlp"
)

const (
	appName = "Ethermint"
)

var k = rlp.String

// Extended ABCI application
type EthermintApp struct {
	*bam.BaseApp
	cdc *wire.Codec

	// keys to access the substores
	capKeyMainStore    *sdk.KVStoreKey
	capKeyAccountStore *sdk.KVStoreKey
	capKeyIBCStore     *sdk.KVStoreKey
	capKeyStakingStore *sdk.KVStoreKey

	// Manage getting and setting accounts
	accountMapper sdk.AccountMapper
}

func NewEthermintApp(logger log.Logger, dbs map[string]dbm.DB) *EthermintApp {
	// create your application object
	var app = &EthermintApp{
		BaseApp:            bam.NewBaseApp(appName, logger, dbs["main"]),
		cdc:                MakeCodec(),
		capKeyMainStore:    sdk.NewKVStoreKey("main"),
		capKeyAccountStore: sdk.NewKVStoreKey("acc"),
		capKeyIBCStore:     sdk.NewKVStoreKey("ibc"),
		capKeyStakingStore: sdk.NewKVStoreKey("staking"),
	}

	// define the accountMapper
	app.accountMapper = auth.NewAccountMapperSealed(
		app.capKeyMainStore, // target store
		&types.AppAccount{}, // prototype
	)

	// add handlers
	coinKeeper := bank.NewCoinKeeper(app.accountMapper)
	ibcMapper := ibc.NewIBCMapper(app.cdc, app.capKeyIBCStore)
	stakeKeeper := simplestake.NewKeeper(app.capKeyStakingStore, coinKeeper)
	app.Router().
		AddRoute("bank", bank.NewHandler(coinKeeper)).
		AddRoute("ibc", ibc.NewHandler(ibcMapper, coinKeeper)).
		AddRoute("simplestake", simplestake.NewHandler(stakeKeeper))

	// initialize BaseApp
	app.SetTxDecoder(app.txDecoder)
	app.SetInitChainer(app.initChainer)
	app.MountStoreWithDB(app.capKeyMainStore, sdk.StoreTypeIAVL, dbs["main"])
	app.MountStoreWithDB(app.capKeyAccountStore, sdk.StoreTypeIAVL, dbs["acc"])
	app.MountStoreWithDB(app.capKeyIBCStore, sdk.StoreTypeIAVL, dbs["ibc"])
	app.MountStoreWithDB(app.capKeyStakingStore, sdk.StoreTypeIAVL, dbs["staking"])
	app.SetAnteHandler(auth.NewAnteHandler(app.accountMapper))
	err := app.LoadLatestVersion(app.capKeyMainStore)
	if err != nil {
		cmn.Exit(err.Error())
	}

	return app
}

// custom tx codec
// TODO: use new go-wire
func MakeCodec() *wire.Codec {
	const msgTypeSend = 0x1
	const msgTypeIssue = 0x2
	const msgTypeQuiz = 0x3
	const msgTypeSetTrend = 0x4
	const msgTypeIBCTransferMsg = 0x5
	const msgTypeIBCReceiveMsg = 0x6
	const msgTypeBondMsg = 0x7
	const msgTypeUnbondMsg = 0x8
	const msgTypeRawEthTxMsg = 0x9
	var _ = oldwire.RegisterInterface(
		struct{ sdk.Msg }{},
		oldwire.ConcreteType{bank.SendMsg{}, msgTypeSend},
		oldwire.ConcreteType{bank.IssueMsg{}, msgTypeIssue},
		oldwire.ConcreteType{ibc.IBCTransferMsg{}, msgTypeIBCTransferMsg},
		oldwire.ConcreteType{ibc.IBCReceiveMsg{}, msgTypeIBCReceiveMsg},
		oldwire.ConcreteType{simplestake.BondMsg{}, msgTypeBondMsg},
		oldwire.ConcreteType{simplestake.UnbondMsg{}, msgTypeUnbondMsg},
		oldwire.ConcreteType{eth.RawTxMsg{}, msgTypeRawEthTxMsg},
	)

	const accTypeApp = 0x1
	var _ = oldwire.RegisterInterface(
		struct{ sdk.Account }{},
		oldwire.ConcreteType{&types.AppAccount{}, accTypeApp},
	)
	cdc := wire.NewCodec()

	// cdc.RegisterInterface((*sdk.Msg)(nil), nil)
	// bank.RegisterWire(cdc)   // Register bank.[SendMsg,IssueMsg] types.
	// crypto.RegisterWire(cdc) // Register crypto.[PubKey,PrivKey,Signature] types.
	// ibc.RegisterWire(cdc) // Register ibc.[IBCTransferMsg, IBCReceiveMsg] types.
	return cdc
}

// custom logic for transaction decoding
func (app *EthermintApp) txDecoder(txBytes []byte) (sdk.Tx, sdk.Error) {
	var tx = sdk.StdTx{}

	if len(txBytes) == 0 {
		return nil, sdk.ErrTxDecode("txBytes are empty")
	}

	// StdTx.Msg is an interface. The concrete types
	// are registered by MakeTxCodec in bank.RegisterWire.
	err := app.cdc.UnmarshalBinary(txBytes, &tx)
	if err != nil {
		return nil, sdk.ErrTxDecode("").TraceCause(err, "")
	}
	if tx.GetMsg().Type() == eth.EthRawMsgType {
		if ethrawtx, ok := tx.GetMsg().(eth.RawTxMsg); ok {
			ethrawtx.DecodeRaw()
		} else {
			return nil, sdk.ErrTxDecode(fmt.Sprintf("tx is interpreted as EthRaw, but cannot be cast to apppropriate type: %x", ethrawtx))
		}
	}
	return tx, nil
}

// custom logic for basecoin initialization
func (app *EthermintApp) initChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	stateJSON := req.AppStateBytes

	genesisState := new(types.GenesisState)
	err := json.Unmarshal(stateJSON, genesisState)
	if err != nil {
		panic(err) // TODO https://github.com/cosmos/cosmos-sdk/issues/468
		// return sdk.ErrGenesisParse("").TraceCause(err, "")
	}

	for _, gacc := range genesisState.Accounts {
		acc, err := gacc.ToAppAccount()
		if err != nil {
			panic(err) // TODO https://github.com/cosmos/cosmos-sdk/issues/468
			//	return sdk.ErrGenesisParse("").TraceCause(err, "")
		}
		app.accountMapper.SetAccount(ctx, acc)
	}
	return abci.ResponseInitChain{}
}
