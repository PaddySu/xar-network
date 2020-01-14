package keeper_test

import (
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/exported"
	"github.com/cosmos/cosmos-sdk/x/mock"
	"github.com/cosmos/cosmos-sdk/x/supply"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/xar-network/xar-network/x/csdt/internal/types"
	"github.com/xar-network/xar-network/x/oracle"
)

// How could one reduce the number of params in the test cases. Create a table driven test for each of the 4 add/withdraw collateral/debt?

func TestKeeper_ModifyCSDT(t *testing.T) {
	_, addrs := mock.GeneratePrivKeyAddressPairs(2)
	ownerAddr := addrs[0]

	type state struct { // TODO this allows invalid state to be set up, should it?
		CSDT            CSDT
		OwnerCoins      sdk.Coins
		GlobalDebt      sdk.Int
		CollateralState CollateralState
		ModuleCoins     sdk.Coins
	}
	type args struct {
		owner              sdk.AccAddress
		collateralDenom    string
		changeInCollateral sdk.Int
		changeInDebt       sdk.Int
	}

	tests := []struct {
		name       string
		priorState state
		price      string
		// also missing CSDTModuleParams
		args          args
		expectPass    bool
		expectedState state
	}{
		{
			"addCollateralAndDecreaseDebt",
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 100)),
				Debt:             cs(c(StableDenom, 2)),
			}, cs(c("uftm", 10), c(StableDenom, 2)), i(2), CollateralState{Denom: "uftm", TotalDebt: i(2)}, cs(c("uftm", 100))},
			"10.345",
			args{ownerAddr, "uftm", i(10), i(-1)},
			true,
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 110)),
				Debt:             cs(c(StableDenom, 1)),
			}, cs( /*  0uftm  */ c(StableDenom, 1)), i(1), CollateralState{Denom: "uftm", TotalDebt: i(1)}, cs(c("uftm", 110))},
		},
		{
			"removeTooMuchCollateral",
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 1000)),
				Debt:             cs(c(StableDenom, 200)),
			}, cs(c("uftm", 10), c(StableDenom, 10)), i(200), CollateralState{Denom: "uftm", TotalDebt: i(200)}, cs(c("uftm", 1000))},
			"1.00",
			args{ownerAddr, "uftm", i(-801), i(0)},
			false,
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 1000)),
				Debt:             cs(c(StableDenom, 200)),
			}, cs(c("uftm", 10), c(StableDenom, 10)), i(200), CollateralState{Denom: "uftm", TotalDebt: i(200)}, cs(c("uftm", 1000))},
		},
		{
			"withdrawTooMuchStableCoin",
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 1000)),
				Debt:             cs(c(StableDenom, 200)),
			}, cs(c("uftm", 10), c(StableDenom, 10)), i(200), CollateralState{Denom: "uftm", TotalDebt: i(200)}, cs(c("uftm", 1000))},
			"1.00",
			args{ownerAddr, "uftm", i(0), i(500)},
			false,
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 1000)),
				Debt:             cs(c(StableDenom, 200)),
			}, cs(c("uftm", 10), c(StableDenom, 10)), i(200), CollateralState{Denom: "uftm", TotalDebt: i(200)}, cs(c("uftm", 1000))},
		},
		{
			"createCSDTAndWithdrawStable",
			state{CSDT{}, cs(c("uftm", 10), c(StableDenom, 10)), i(0), CollateralState{Denom: "uftm", TotalDebt: i(0)}, cs(c("uftm", 0))},
			"1.00",
			args{ownerAddr, "uftm", i(5), i(2)},
			true,
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 5)),
				Debt:             cs(c(StableDenom, 2)),
			}, cs(c("uftm", 5), c(StableDenom, 12)), i(2), CollateralState{Denom: "uftm", TotalDebt: i(2)}, cs(c("uftm", 5))},
		},
		{
			"emptyCSDT",
			state{CSDT{
				Owner:            ownerAddr,
				CollateralDenom:  "uftm",
				CollateralAmount: cs(c("uftm", 1000)),
				Debt:             cs(c(StableDenom, 200)),
			}, cs(c("uftm", 10), c(StableDenom, 201)), i(200), CollateralState{Denom: "uftm", TotalDebt: i(200)}, cs(c("uftm", 1000))},
			"1.00",
			args{ownerAddr, "uftm", i(-1000), i(-200)},
			true,
			state{CSDT{}, cs(c("uftm", 1010), c(StableDenom, 1)), i(0), CollateralState{Denom: "uftm", TotalDebt: i(0)}, cs(c("uftm", 0))},
		},
		{
			"invalidCollateralType",
			state{CSDT{}, cs(c("shitcoin", 5000000)), i(0), CollateralState{}, cs(c("uftm", 0))},
			"0.000001",
			args{ownerAddr, "shitcoin", i(5000000), i(1)}, // ratio of 5:1
			false,
			state{CSDT{}, cs(c("shitcoin", 5000000)), i(0), CollateralState{}, cs(c("uftm", 0))},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// setup keeper
			mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
			// initialize csdt owner account with coins
			genAcc := auth.BaseAccount{
				Address: ownerAddr,
				Coins:   tc.priorState.OwnerCoins,
			}

			mock.SetGenesis(mapp, []exported.Account{&genAcc})
			// create a new context
			header := abci.Header{Height: mapp.LastBlockHeight() + 1}
			mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
			ctx := mapp.BaseApp.NewContext(false, header)
			// setup store state
			oracleParams := oracle.DefaultParams()
			oracleParams.Assets = oracle.Assets{
				oracle.Asset{
					AssetCode:  "uftm",
					BaseAsset:  "uftm",
					QuoteAsset: StableDenom,
					Oracles: oracle.Oracles{
						oracle.Oracle{
							Address: addrs[1],
						},
					},
				},
			}
			oracleParams.Nominees = []string{addrs[1].String()}

			keeper.GetOracle().SetParams(ctx, oracleParams)
			_, _ = keeper.GetOracle().SetPrice(
				ctx, addrs[1], "uftm",
				sdk.MustNewDecFromStr(tc.price),
				time.Now().Add(time.Hour*1))
			_ = keeper.GetOracle().SetCurrentPrices(ctx)

			if tc.priorState.CSDT.CollateralDenom != "" { // check if the prior CSDT should be created or not (see if an empty one was specified)
				keeper.SetCSDT(ctx, tc.priorState.CSDT)
			}
			keeper.SetGlobalDebt(ctx, tc.priorState.GlobalDebt)
			if tc.priorState.CollateralState.Denom != "" {
				keeper.SetCollateralState(ctx, tc.priorState.CollateralState)
			}

			keeper.GetSupply().SetSupply(ctx, supply.NewSupply(sdk.NewCoins(sdk.NewCoin(StableDenom, tc.priorState.GlobalDebt))))

			keeper.GetSupply().MintCoins(ctx, types.ModuleName, tc.priorState.ModuleCoins)

			// call func under test
			keeper.SetParams(ctx, types.DefaultParams())
			err := keeper.ModifyCSDT(ctx, tc.args.owner, tc.args.collateralDenom, tc.args.changeInCollateral, tc.args.changeInDebt)
			mapp.EndBlock(abci.RequestEndBlock{})
			mapp.Commit()

			// check for err
			if tc.expectPass {
				require.NoError(t, err, fmt.Sprint(err))
			} else {
				require.Error(t, err)
			}
			// get new state for verification
			actualCSDT, found := keeper.GetCSDT(ctx, tc.args.owner, tc.args.collateralDenom)
			actualGDebt := keeper.GetGlobalDebt(ctx)
			actualCstate, _ := keeper.GetCollateralState(ctx, tc.args.collateralDenom)
			// check state
			require.Equal(t, tc.expectedState.CSDT, actualCSDT)
			if tc.expectedState.CSDT.CollateralDenom == "" { // if the expected CSDT is blank, then expect the CSDT to have been deleted (hence not found)
				require.False(t, found)
			} else {
				require.True(t, found)
			}
			require.Equal(t, tc.expectedState.GlobalDebt, actualGDebt)
			require.Equal(t, tc.expectedState.CollateralState, actualCstate)
			// check owner balance
			mock.CheckBalance(t, mapp, ownerAddr, tc.expectedState.OwnerCoins)
		})
	}
}

// TODO change to table driven test to test more test cases
func TestKeeper_PartialSeizeCSDT(t *testing.T) {
	// Setup
	const collateral = "uftm"
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	genAccs, addrs, _, _ := mock.CreateGenAccounts(2, cs(c(collateral, 100)))

	testAddr := addrs[0]
	mock.SetGenesis(mapp, genAccs)
	// setup oracle
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)

	// setup store state
	oracleParams := oracle.DefaultParams()
	oracleParams.Assets = oracle.Assets{
		oracle.Asset{
			AssetCode:  collateral,
			BaseAsset:  collateral,
			QuoteAsset: StableDenom,
			Oracles: oracle.Oracles{
				oracle.Oracle{
					Address: addrs[1],
				},
			},
		},
	}
	oracleParams.Nominees = []string{addrs[1].String()}

	keeper.GetOracle().SetParams(ctx, oracleParams)
	_, _ = keeper.GetOracle().SetPrice(
		ctx, addrs[1], collateral,
		sdk.MustNewDecFromStr("1.00"),
		time.Now().Add(time.Hour*1))
	_ = keeper.GetOracle().SetCurrentPrices(ctx)

	// Create CSDT
	keeper.SetParams(ctx, types.DefaultParams())
	keeper.SetGlobalDebt(ctx, sdk.NewInt(0))
	keeper.GetSupply().SetSupply(ctx, supply.NewSupply(sdk.NewCoins(sdk.NewCoin(collateral, sdk.NewInt(200)))))

	err := keeper.ModifyCSDT(ctx, testAddr, collateral, i(10), i(5))
	require.NoError(t, err)
	// Reduce price
	_, _ = keeper.GetOracle().SetPrice(
		ctx, addrs[1], collateral,
		sdk.MustNewDecFromStr("0.50"),
		time.Now().Add(time.Hour*1))
	_ = keeper.GetOracle().SetCurrentPrices(ctx)
	// Seize entire CSDT
	err = keeper.PartialSeizeCSDT(ctx, testAddr, collateral, i(10), i(5))

	// Check
	require.NoError(t, err)
	_, found := keeper.GetCSDT(ctx, testAddr, collateral)
	require.False(t, found)
	collateralState, found := keeper.GetCollateralState(ctx, collateral)
	require.True(t, found)
	require.Equal(t, sdk.ZeroInt(), collateralState.TotalDebt)
}

// TODO change to table driven test to test more test cases
func TestKeeper_CollateralParams(t *testing.T) {
	// Setup
	const collateral = "uftm"
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	genAccs, addrs, _, _ := mock.CreateGenAccounts(2, cs(c(collateral, 100)))

	//testAddr := addrs[0]
	mock.SetGenesis(mapp, genAccs)
	// setup oracle
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)

	params := types.DefaultParams()
	params.Nominees = []string{addrs[1].String()}

	// Create CSDT
	keeper.SetParams(ctx, params)
	keeper.SetGlobalDebt(ctx, sdk.NewInt(0))
	keeper.GetSupply().SetSupply(ctx, supply.NewSupply(sdk.NewCoins(sdk.NewCoin(collateral, sdk.NewInt(200)))))

	// Try to add a denom that already exists and fail
	collateralParam := types.CollateralParam{
		Denom:            "uftm",
		LiquidationRatio: sdk.MustNewDecFromStr("1.5"),
		DebtLimit:        sdk.NewCoins(sdk.NewCoin(StableDenom, sdk.NewInt(500000000000))),
	}
	err := keeper.AddCollateralParam(ctx, addrs[1].String(), collateralParam)
	require.Error(t, err)
	// Try to set an existing denom

	err = keeper.SetCollateralParam(ctx, addrs[1].String(), collateralParam)
	require.NoError(t, err)

	// Try to set with non authority
	err = keeper.SetCollateralParam(ctx, addrs[0].String(), collateralParam)
	require.Error(t, err)
	// Try to add when not a nominee
	err = keeper.AddCollateralParam(ctx, addrs[0].String(), collateralParam)
	require.Error(t, err)
	collateralParam = types.CollateralParam{
		Denom:            "uftm2",
		LiquidationRatio: sdk.MustNewDecFromStr("1.5"),
		DebtLimit:        sdk.NewCoins(sdk.NewCoin(StableDenom, sdk.NewInt(500000000000))),
	}
	err = keeper.SetCollateralParam(ctx, addrs[1].String(), collateralParam)
	require.Error(t, err)

	// Add successfully
	err = keeper.AddCollateralParam(ctx, addrs[1].String(), collateralParam)
	require.NoError(t, err)
}

func TestKeeper_GetCSDTs(t *testing.T) {
	// setup keeper
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	mock.SetGenesis(mapp, []exported.Account(nil))
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	// setup CSDTs
	_, addrs := mock.GeneratePrivKeyAddressPairs(2)
	csdts := CSDTs{
		{Owner: addrs[0], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 5))},
		{Owner: addrs[1], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 2000))},
		{Owner: addrs[0], CollateralDenom: "ubtc", CollateralAmount: cs(c("uftm", 10)), Debt: cs(c(StableDenom, 20))},
	}
	for _, csdt := range csdts {
		keeper.SetCSDT(ctx, csdt)
	}

	tests := []struct {
		name            string
		collateralDenom string
		price           sdk.Dec
		expectError     bool
		expected        CSDTs
	}{
		{
			"nilParamsReturnNilCsdts",
			"",
			sdk.Dec{},
			false,
			CSDTs{
				{Owner: addrs[0], CollateralDenom: "ubtc", CollateralAmount: cs(c("uftm", 10)), Debt: cs(c(StableDenom, 20))},
				{Owner: addrs[1], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 2000))},
				{Owner: addrs[0], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 5))},
			},
		},
		{
			"csdtsFilteredByCollateralAndSortedNoPrice",
			"uftm",
			sdk.Dec{},
			false,
			CSDTs{
				{Owner: addrs[1], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 2000))},
				{Owner: addrs[0], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 5))},
			},
		},
		{
			"csdtsFilteredByCollateralAndSortedMinimalPrice",
			"uftm",
			d("0.00000001"),
			false,
			CSDTs{
				{Owner: addrs[1], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 2000))},
				{Owner: addrs[0], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 5))},
			},
		},
		{
			"csdtsFilteredByCollateralAndSorted",
			"uftm",
			d("0.74"),
			false,
			CSDTs{
				{Owner: addrs[1], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 2000))},
			},
		},
		{
			"highPriceReturnsNoCsdts",
			"uftm",
			d("999999999.99"),
			false,
			CSDTs(nil),
		},
		{
			"unauthorisedCollateralDenomReturnsError",
			"a non existent coin",
			d("0.34023"),
			true,
			CSDTs(nil),
		},
		{
			"priceWithoutCollateralReturnsError",
			"",
			d("0.34023"),
			true,
			CSDTs(nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keeper.SetParams(ctx, types.DefaultParams())
			returnedCsdts, err := keeper.GetCSDTs(ctx, tc.collateralDenom, tc.price)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, returnedCsdts)
			}
		})
	}

	// Check deleting a CSDT removes it
	keeper.DeleteCSDT(ctx, csdts[0])
	returnedCsdts, err := keeper.GetCSDTs(ctx, "", sdk.Dec{})
	require.NoError(t, err)
	require.Equal(t,
		CSDTs{
			{Owner: addrs[0], CollateralDenom: "ubtc", CollateralAmount: cs(c("uftm", 10)), Debt: cs(c(StableDenom, 20))},
			{Owner: addrs[1], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 4000)), Debt: cs(c(StableDenom, 2000))},
		},
		returnedCsdts,
	)
}
func TestKeeper_GetSetDeleteCSDT(t *testing.T) {
	// setup keeper, create CSDT
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	_, addrs := mock.GeneratePrivKeyAddressPairs(1)
	csdt := CSDT{Owner: addrs[0], CollateralDenom: "uftm", CollateralAmount: cs(c("uftm", 412)), Debt: cs(c(StableDenom, 56))}

	// write and read from store
	keeper.SetCSDT(ctx, csdt)
	readCSDT, found := keeper.GetCSDT(ctx, csdt.Owner, csdt.CollateralDenom)

	// check before and after match
	require.True(t, found)
	require.Equal(t, csdt, readCSDT)

	// delete auction
	keeper.DeleteCSDT(ctx, csdt)

	// check auction does not exist
	_, found = keeper.GetCSDT(ctx, csdt.Owner, csdt.CollateralDenom)
	require.False(t, found)
}

func TestKeeper_GetSetGlobalBorrows(t *testing.T) {
	// setup keeper, create global cash
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	gBorrows := sdk.NewUint(888890000)

	// write and read from store
	keeper.SetTotalBorrows(ctx, gBorrows, "btc")
	readGBorrows, ok := keeper.GetTotalBorrows(ctx, "btc")

	// check before and after match
	require.True(t, ok, "must exist")
	require.Equal(t, gBorrows, readGBorrows)
}

func TestKeeper_GetSetGlobalCash(t *testing.T) {
	// setup keeper, create global cash
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	gCash := sdk.NewUint(99990000)

	// write and read from store
	keeper.SetTotalCash(ctx, gCash, "btc")
	readGCash, ok := keeper.GetTotalCash(ctx, "btc")

	// check before and after match
	require.True(t, ok, "must exist")
	require.Equal(t, gCash, readGCash)
}

func TestKeeper_GetSetGlobalReserve(t *testing.T) {
	// setup keeper, create global reserve
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	gCash := sdk.NewUint(77770000)

	// write and read from store
	keeper.SetTotalReserve(ctx, gCash, "btc")
	readGReserve, ok := keeper.GetTotalReserve(ctx, "btc")

	// check before and after match
	require.True(t, ok, "must exist")
	require.Equal(t, gCash, readGReserve)
}

func TestKeeper_GetSetGDebt(t *testing.T) {
	// setup keeper, create GDebt
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	gDebt := i(4120000)

	// write and read from store
	keeper.SetGlobalDebt(ctx, gDebt)
	readGDebt := keeper.GetGlobalDebt(ctx)

	// check before and after match
	require.Equal(t, gDebt, readGDebt)
}

func TestKeeper_GetSetCollateralState(t *testing.T) {
	// setup keeper, create CState
	mapp, keeper, _, _ := setUpMockAppWithoutGenesis()
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	collateralState := CollateralState{Denom: "uftm", TotalDebt: i(15400)}

	// write and read from store
	keeper.SetCollateralState(ctx, collateralState)
	readCState, found := keeper.GetCollateralState(ctx, collateralState.Denom)

	// check before and after match
	require.Equal(t, collateralState, readCState)
	require.True(t, found)
}

// shorten for easier reading
type (
	CSDT            = types.CSDT
	CSDTs           = types.CSDTs
	CollateralState = types.CollateralState
)

const (
	StableDenom = types.StableDenom
)
