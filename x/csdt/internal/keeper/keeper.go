package keeper

import (
	"bytes"
	"sort"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/xar-network/xar-network/x/csdt/internal/types"
)

// Keeper csdt Keeper
type Keeper struct {
	storeKey       sdk.StoreKey
	cdc            *codec.Codec
	paramsSubspace params.Subspace
	oracle         types.OracleKeeper
	bank           types.BankKeeper
	sk             types.SupplyKeeper
}

// NewKeeper creates a new keeper
func NewKeeper(
	cdc *codec.Codec,
	storeKey sdk.StoreKey,
	subspace params.Subspace,
	oracle types.OracleKeeper,
	bank types.BankKeeper,
	supply types.SupplyKeeper,
) Keeper {
	return Keeper{
		storeKey:       storeKey,
		oracle:         oracle,
		bank:           bank,
		paramsSubspace: subspace.WithKeyTable(types.ParamKeyTable()),
		cdc:            cdc,
		sk:             supply,
	}
}

// ModifyCSDT creates, changes, or deletes a CSDT
// TODO can/should this function be split up?
func (k Keeper) ModifyCSDT(ctx sdk.Context, owner sdk.AccAddress, collateralDenom string, changeInCollateral sdk.Int, changeInDebt sdk.Int) sdk.Error {

	// Phase 1: Get state, make changes in memory and check if they're ok.

	// Check collateral type ok
	p := k.GetParams(ctx)
	if !p.IsCollateralPresent(collateralDenom) { // maybe abstract this logic into GetCSDT
		return sdk.ErrInternal("collateral type not enabled to create CSDTs")
	}

	// Check the owner has enough collateral and stable coins
	if changeInCollateral.IsPositive() { // adding collateral to CSDT
		ok := k.bank.HasCoins(ctx, owner, sdk.NewCoins(sdk.NewCoin(collateralDenom, changeInCollateral)))
		if !ok {
			return sdk.ErrInsufficientCoins("not enough collateral in sender's account")
		}
	}
	if changeInDebt.IsNegative() { // reducing debt, by adding stable coin to CSDT
		ok := k.bank.HasCoins(ctx, owner, sdk.NewCoins(sdk.NewCoin(types.StableDenom, changeInDebt.Neg())))
		if !ok {
			return sdk.ErrInsufficientCoins("not enough stable coin in sender's account")
		}
	}

	// Change collateral and debt recorded in CSDT
	// Get CSDT (or create if not exists)
	csdt, found := k.GetCSDT(ctx, owner, collateralDenom)
	if !found {
		csdt = types.CSDT{
			Owner:            owner,
			CollateralDenom:  collateralDenom,
			CollateralAmount: sdk.NewCoins(sdk.NewCoin(collateralDenom, sdk.ZeroInt())),
			Debt:             sdk.NewCoins(sdk.NewCoin(types.StableDenom, sdk.ZeroInt())),
			AccumulatedFees:  sdk.NewCoins(sdk.NewCoin(types.StableDenom, sdk.ZeroInt())),
		}
	}
	// Add/Subtract collateral and debt
	var collateralCoins sdk.Coins
	var debtCoins sdk.Coins

	if changeInCollateral.IsNegative() {
		collateralCoins = sdk.NewCoins(sdk.NewCoin(collateralDenom, changeInCollateral.Neg()))
		csdt.CollateralAmount = csdt.CollateralAmount.Sub(collateralCoins)
	} else {
		collateralCoins = sdk.NewCoins(sdk.NewCoin(collateralDenom, changeInCollateral))
		csdt.CollateralAmount = csdt.CollateralAmount.Add(collateralCoins)
	}

	if csdt.CollateralAmount.IsAnyNegative() {
		return sdk.ErrInternal(" can't withdraw more collateral than exists in CSDT")
	}

	if changeInDebt.IsNegative() {
		debtCoins = sdk.NewCoins(sdk.NewCoin(types.StableDenom, changeInDebt.Neg()))
		csdt.Debt = csdt.Debt.Sub(debtCoins)
	} else {
		debtCoins = sdk.NewCoins(sdk.NewCoin(types.StableDenom, changeInDebt))
		csdt.Debt = csdt.Debt.Add(debtCoins)
	}

	if csdt.Debt.IsAnyNegative() {
		return sdk.ErrInternal("can't pay back more debt than exists in CSDT")
	}

	// If we have prices denominated in non csdt pairs, this changes the model
	isUnderCollateralized := csdt.IsUnderCollateralized(
		k.oracle.GetCurrentPrice(ctx, csdt.CollateralDenom).Price,
		p.GetCollateralParam(csdt.CollateralDenom).LiquidationRatio,
	)
	if isUnderCollateralized {
		return sdk.ErrInternal("Change to CSDT would put it below liquidation ratio")
	}
	// TODO check for dust

	// Add/Subtract from global debt limit
	gDebt := k.GetGlobalDebt(ctx)
	gDebt = gDebt.Add(changeInDebt)
	if gDebt.IsNegative() {
		return sdk.ErrInternal("global debt can't be negative") // This should never happen if debt per CSDT can't be negative
	}
	if gDebt.GT(p.GlobalDebtLimit.AmountOf(types.StableDenom)) {
		return sdk.ErrInternal("change to CSDT would put the system over the global debt limit")
	}

	// Add/Subtract from collateral debt limit
	collateralState, found := k.GetCollateralState(ctx, csdt.CollateralDenom)
	if !found {
		collateralState = types.CollateralState{Denom: csdt.CollateralDenom, TotalDebt: sdk.ZeroInt()} // Already checked that this denom is authorized, so ok to create new CollateralState
	}
	collateralState.TotalDebt = collateralState.TotalDebt.Add(changeInDebt)
	if collateralState.TotalDebt.IsNegative() {
		return sdk.ErrInternal("total debt for this collateral type can't be negative") // This should never happen if debt per CSDT can't be negative
	}
	if collateralState.TotalDebt.GT(p.GetCollateralParam(csdt.CollateralDenom).DebtLimit.AmountOf(types.StableDenom)) {
		return sdk.ErrInternal("change to CSDT would put the system over the debt limit for this collateral type")
	}

	// Phase 2: Update all the state

	// change owner's coins (increase or decrease)
	var err sdk.Error
	if changeInCollateral.IsNegative() {
		err = k.sk.SendCoinsFromModuleToAccount(ctx, types.ModuleName, owner, sdk.NewCoins(sdk.NewCoin(collateralDenom, changeInCollateral.Neg())))
		if err != nil {
			panic(err) // this shouldn't happen because coin balance was checked earlier
		}
	} else {
		err = k.sk.SendCoinsFromAccountToModule(ctx, owner, types.ModuleName, sdk.NewCoins(sdk.NewCoin(collateralDenom, changeInCollateral)))
		if err != nil {
			panic(err) // this shouldn't happen because coin balance was checked earlier
		}
	}

	if changeInDebt.IsNegative() { //Depositing stable coin from owner to CSDT (decrease supply)
		depositCoins := sdk.NewCoins(sdk.NewCoin(types.StableDenom, changeInDebt.Neg()))

		er := k.sk.SendCoinsFromAccountToModule(ctx, owner, types.ModuleName, depositCoins)
		if er != nil {
			return er
		}

		er = k.sk.BurnCoins(ctx, types.ModuleName, depositCoins)
		if er != nil {
			return er
		}
	} else { //Withdrawing stable coins to owner (minting)
		withdrawCoins := sdk.NewCoins(sdk.NewCoin(types.StableDenom, changeInDebt))

		er := k.sk.MintCoins(ctx, types.ModuleName, withdrawCoins)
		if er != nil {
			return er
		}

		er = k.sk.SendCoinsFromModuleToAccount(ctx, types.ModuleName, owner, withdrawCoins)
		if er != nil {
			return er
		}
	}
	if err != nil {
		panic(err) // this shouldn't happen because coin balance was checked earlier
	}
	// Set CSDT
	if csdt.CollateralAmount.IsZero() && csdt.Debt.IsZero() { // TODO maybe abstract this logic into SetCSDT
		k.DeleteCSDT(ctx, csdt)
	} else {
		k.SetCSDT(ctx, csdt)
	}
	// set total debts
	k.SetGlobalDebt(ctx, gDebt)
	k.SetCollateralState(ctx, collateralState)

	return nil
}

// TODO
// // TransferCSDT allows people to transfer ownership of their CSDTs to others
// func (k Keeper) TransferCSDT(ctx sdk.Context, from sdk.AccAddress, to sdk.AccAddress, collateralDenom string) sdk.Error {
// 	return nil
// }

// PartialSeizeCSDT removes collateral and debt from a CSDT and decrements global debt counters. It does not move collateral to another account so is unsafe.
// TODO should this be made safer by moving collateral to liquidatorModuleAccount ? If so how should debt be moved?
func (k Keeper) PartialSeizeCSDT(ctx sdk.Context, owner sdk.AccAddress, collateralDenom string, collateralToSeize sdk.Int, debtToSeize sdk.Int) sdk.Error {
	// get CSDT
	csdt, found := k.GetCSDT(ctx, owner, collateralDenom)
	if !found {
		return sdk.ErrInternal("could not find CSDT")
	}

	// Check if CSDT is undercollateralized
	p := k.GetParams(ctx)
	isUnderCollateralized := csdt.IsUnderCollateralized(
		k.oracle.GetCurrentPrice(ctx, csdt.CollateralDenom).Price,
		p.GetCollateralParam(csdt.CollateralDenom).LiquidationRatio,
	)
	if !isUnderCollateralized {
		return sdk.ErrInternal("CSDT is not currently under the liquidation ratio")
	}

	// Remove Collateral
	if collateralToSeize.IsNegative() {
		return sdk.ErrInternal("cannot seize negative collateral")
	}
	collateralCoins := sdk.NewCoins(sdk.NewCoin(csdt.CollateralDenom, collateralToSeize))
	csdt.CollateralAmount = csdt.CollateralAmount.Sub(collateralCoins)
	if csdt.CollateralAmount.IsAnyNegative() {
		return sdk.ErrInternal("can't seize more collateral than exists in CSDT")
	}

	// Remove Debt
	if debtToSeize.IsNegative() {
		return sdk.ErrInternal("cannot seize negative debt")
	}
	debtCoins := sdk.NewCoins(sdk.NewCoin(types.StableDenom, debtToSeize))
	csdt.Debt = csdt.Debt.Sub(debtCoins)
	if csdt.Debt.IsAnyNegative() {
		return sdk.ErrInternal("can't seize more debt than exists in CSDT")
	}

	// Update debt per collateral type
	collateralState, found := k.GetCollateralState(ctx, csdt.CollateralDenom)
	if !found {
		return sdk.ErrInternal("could not find collateral state")
	}
	collateralState.TotalDebt = collateralState.TotalDebt.Sub(debtToSeize)
	if collateralState.TotalDebt.IsNegative() {
		return sdk.ErrInternal("Total debt per collateral type is negative.") // This should not happen given the checks on the CSDT.
	}

	// Note: Global debt is not decremented here. It's only decremented when debt and stable coin are annihilated (aka heal)
	// TODO update global seized debt? this is what maker does (named vice in Vat.grab) but it's not used anywhere

	// Store updated state
	if csdt.CollateralAmount.IsZero() && csdt.Debt.IsZero() { // TODO maybe abstract this logic into SetCSDT
		k.DeleteCSDT(ctx, csdt)
	} else {
		k.SetCSDT(ctx, csdt)
	}
	k.SetCollateralState(ctx, collateralState)
	return nil
}

// ReduceGlobalDebt decreases the stored global debt counter. It is used by the liquidator when it annihilates debt and stable coin.
// TODO Can the interface between csdt and liquidator modules be improved so that this function doesn't exist?
func (k Keeper) ReduceGlobalDebt(ctx sdk.Context, amount sdk.Int) sdk.Error {
	if amount.IsNegative() {
		return sdk.ErrInternal("reduction in global debt must be a positive amount")
	}
	newGDebt := k.GetGlobalDebt(ctx).Sub(amount)
	if newGDebt.IsNegative() {
		return sdk.ErrInternal("cannot reduce global debt by amount specified")
	}
	k.SetGlobalDebt(ctx, newGDebt)
	return nil
}

func (k Keeper) GetStableDenom() string {
	return types.StableDenom
}
func (k Keeper) GetGovDenom() string {
	return types.GovDenom
}

// ---------- Store Wrappers ----------

func (k Keeper) getCSDTKeyPrefix(collateralDenom string) []byte {
	return bytes.Join(
		[][]byte{
			[]byte("csdt"),
			[]byte(collateralDenom),
		},
		nil, // no separator
	)
}
func (k Keeper) getCSDTKey(owner sdk.AccAddress, collateralDenom string) []byte {
	return bytes.Join(
		[][]byte{
			k.getCSDTKeyPrefix(collateralDenom),
			[]byte(owner.String()),
		},
		nil, // no separator
	)
}
func (k Keeper) GetCSDT(ctx sdk.Context, owner sdk.AccAddress, collateralDenom string) (types.CSDT, bool) {
	// get store
	store := ctx.KVStore(k.storeKey)
	// get CSDT
	bz := store.Get(k.getCSDTKey(owner, collateralDenom))
	// unmarshal
	if bz == nil {
		return types.CSDT{}, false
	}
	var csdt types.CSDT
	k.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &csdt)
	return csdt, true
}

//Potentially change this logic to use the account interface?
func (k Keeper) SetCSDT(ctx sdk.Context, csdt types.CSDT) {
	// get store
	store := ctx.KVStore(k.storeKey)
	// marshal and set
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(csdt)
	store.Set(k.getCSDTKey(csdt.Owner, csdt.CollateralDenom), bz)
}
func (k Keeper) DeleteCSDT(ctx sdk.Context, csdt types.CSDT) { // TODO should this id the csdt by passing in owner,collateralDenom pair?
	// get store
	store := ctx.KVStore(k.storeKey)
	// delete key
	store.Delete(k.getCSDTKey(csdt.Owner, csdt.CollateralDenom))
}

// GetCSDTs returns all CSDTs, optionally filtered by collateral type and liquidation price.
// `price` filters for CSDTs that will be below the liquidation ratio when the collateral is at that specified price.
func (k Keeper) GetCSDTs(ctx sdk.Context, collateralDenom string, price sdk.Dec) (types.CSDTs, sdk.Error) {
	// Validate inputs
	p := k.GetParams(ctx)
	if len(collateralDenom) != 0 && !p.IsCollateralPresent(collateralDenom) {
		return nil, sdk.ErrInternal("collateral denom not authorized")
	}
	if len(collateralDenom) == 0 && !(price.IsNil() || price.IsNegative()) {
		return nil, sdk.ErrInternal("cannot specify price without collateral denom")
	}

	// Get an iterator over CSDTs
	store := ctx.KVStore(k.storeKey)
	iter := sdk.KVStorePrefixIterator(store, k.getCSDTKeyPrefix(collateralDenom)) // could be all CSDTs is collateralDenom is ""

	// Decode CSDTs into slice
	var csdts types.CSDTs
	for ; iter.Valid(); iter.Next() {
		var csdt types.CSDT
		k.cdc.MustUnmarshalBinaryLengthPrefixed(iter.Value(), &csdt)
		csdts = append(csdts, csdt)
	}

	// Sort by collateral ratio (collateral/debt)
	sort.Sort(types.ByCollateralRatio(csdts)) // TODO this doesn't make much sense across different collateral types

	// Filter for CSDTs that would be under-collateralized at the specified price
	// If price is nil or -ve, skip the filtering as it would return all CSDTs anyway
	if !price.IsNil() && !price.IsNegative() {
		var filteredCSDTs types.CSDTs
		for _, csdt := range csdts {
			if csdt.IsUnderCollateralized(price, p.GetCollateralParam(collateralDenom).LiquidationRatio) {
				filteredCSDTs = append(filteredCSDTs, csdt)
			} else {
				break // break early because list is sorted
			}
		}
		csdts = filteredCSDTs
	}

	return csdts, nil
}

var globalDebtKey = []byte("globalDebt")

func (k Keeper) GetGlobalDebt(ctx sdk.Context) sdk.Int {
	// get store
	store := ctx.KVStore(k.storeKey)
	// get bytes
	bz := store.Get(globalDebtKey)
	// unmarshal
	if bz == nil {
		panic("global debt not found")
	}
	var globalDebt sdk.Int
	k.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &globalDebt)
	return globalDebt
}
func (k Keeper) SetGlobalDebt(ctx sdk.Context, globalDebt sdk.Int) {
	// get store
	store := ctx.KVStore(k.storeKey)
	// marshal and set
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(globalDebt)
	store.Set(globalDebtKey, bz)
}

func (k Keeper) getCollateralStateKey(collateralDenom string) []byte {
	return []byte(collateralDenom)
}
func (k Keeper) GetCollateralState(ctx sdk.Context, collateralDenom string) (types.CollateralState, bool) {
	// get store
	store := ctx.KVStore(k.storeKey)
	// get bytes
	bz := store.Get(k.getCollateralStateKey(collateralDenom))
	// unmarshal
	if bz == nil {
		return types.CollateralState{}, false
	}
	var collateralState types.CollateralState
	k.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &collateralState)
	return collateralState, true
}
func (k Keeper) SetCollateralState(ctx sdk.Context, collateralstate types.CollateralState) {
	// get store
	store := ctx.KVStore(k.storeKey)
	// marshal and set
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(collateralstate)
	store.Set(k.getCollateralStateKey(collateralstate.Denom), bz)
}

// GetOracle allows testing
func (k Keeper) GetOracle() types.OracleKeeper {
	return k.oracle
}

// GetOracle allows testing
func (k Keeper) GetSupply() types.SupplyKeeper {
	return k.sk
}

func (k Keeper) IsNominee(ctx sdk.Context, nominee string) bool {
	params := k.GetParams(ctx)
	nominees := params.Nominees
	for _, v := range nominees {
		if v == nominee {
			return true
		}
	}
	return false
}
