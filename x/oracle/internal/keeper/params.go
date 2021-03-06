package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/xar-network/xar-network/x/oracle/internal/types"
)

// GetParams gets params from the store
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	return types.NewParams(k.GetAssetParams(ctx), k.GetNomineeParams(ctx))
}

// SetParams updates params in the store
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramstore.SetParamSet(ctx, &params)
}

// GetAssetParams get asset params from store
func (k Keeper) GetAssetParams(ctx sdk.Context) types.Assets {
	var assets types.Assets
	k.paramstore.Get(ctx, types.KeyAssets, &assets)
	return assets
}

// GetNomineeParams get nominee params from store
func (k Keeper) GetNomineeParams(ctx sdk.Context) []string {
	var nominees []string
	k.paramstore.Get(ctx, types.KeyNominees, &nominees)
	return nominees
}

// GetOracles returns the oracles in the oracle store
func (k Keeper) GetOracles(ctx sdk.Context, assetCode string) (types.Oracles, error) {

	for _, a := range k.GetAssetParams(ctx) {
		if assetCode == a.AssetCode {
			return a.Oracles, nil
		}
	}
	return types.Oracles{}, fmt.Errorf("asset %s not found", assetCode)
}

// AddOracle returns the oracle in the oracle store
func (k Keeper) AddOracle(ctx sdk.Context, nominee string, assetCode string, address sdk.AccAddress) error {
	if !k.IsNominee(ctx, nominee) {
		return fmt.Errorf("not a valid nominee %s", nominee)
	}
	_, err := k.GetOracle(ctx, assetCode, address)
	if err == nil {
		return fmt.Errorf("oracle %s already exists for asset %s", address, assetCode)
	}

	assets := k.GetAssetParams(ctx)
	updateAssets := assets[:0]
	found := false
	for _, a := range assets {
		if assetCode == a.AssetCode {
			oracle := types.NewOracle(address)
			a.Oracles = append(a.Oracles, oracle)
			found = true
		}
		updateAssets = append(updateAssets, a)
	}
	if found {
		params := k.GetParams(ctx)
		params.Assets = updateAssets
		k.SetParams(ctx, params)
		return nil
	}
	return fmt.Errorf("asset %s not found", assetCode)
}

// AddOracle returns the oracle in the oracle store
func (k Keeper) SetOracles(ctx sdk.Context, nominee string, assetCode string, addresses types.Oracles) error {
	if !k.IsNominee(ctx, nominee) {
		return fmt.Errorf("not a valid nominee %s", nominee)
	}
	assets := k.GetAssetParams(ctx)
	updateAssets := assets[:0]
	found := false
	for _, a := range assets {
		if assetCode == a.AssetCode {
			a.Oracles = addresses
			found = true
		}
		updateAssets = append(updateAssets, a)
	}
	if found {
		params := k.GetParams(ctx)
		params.Assets = updateAssets
		k.SetParams(ctx, params)
		return nil
	}
	return fmt.Errorf("asset %s not found", assetCode)
}

// AddOracle returns the oracle in the oracle store
func (k Keeper) SetAsset(ctx sdk.Context, nominee string, assetCode string, asset types.Asset) error {
	if !k.IsNominee(ctx, nominee) {
		return fmt.Errorf("not a valid nominee %s", nominee)
	}
	assets := k.GetAssetParams(ctx)
	updateAssets := assets[:0]
	found := false
	for _, a := range assets {
		if assetCode == a.AssetCode {
			a = asset
			found = true
		}
		updateAssets = append(updateAssets, a)
	}
	if found {
		params := k.GetParams(ctx)
		params.Assets = updateAssets
		k.SetParams(ctx, params)
		return nil
	}
	return fmt.Errorf("asset %s not found", assetCode)
}

// AddOracle returns the oracle in the oracle store
func (k Keeper) AddAsset(ctx sdk.Context, nominee string, assetCode string, asset types.Asset) error {
	if !k.IsNominee(ctx, nominee) {
		return fmt.Errorf("not a valid nominee %s", nominee)
	}
	_, exists := k.GetAsset(ctx, assetCode)
	if exists {
		return fmt.Errorf("asset %s already exists", assetCode)
	}
	assets := k.GetAssetParams(ctx)
	assets = append(assets, asset)

	params := k.GetParams(ctx)
	params.Assets = assets
	k.SetParams(ctx, params)
	return nil
}

// GetOracle returns the oracle from the store or an error if not found
func (k Keeper) GetOracle(ctx sdk.Context, assetCode string, address sdk.AccAddress) (types.Oracle, error) {
	oracles, err := k.GetOracles(ctx, assetCode)
	if err != nil {
		return types.Oracle{}, fmt.Errorf("asset %s not found", assetCode)
	}
	for _, o := range oracles {
		if address.Equals(o.Address) {
			return o, nil
		}
	}
	return types.Oracle{}, fmt.Errorf("oracle %s not found for asset %s", address, assetCode)
}

// GetAsset returns the asset if it is in the oracle system
func (k Keeper) GetAsset(ctx sdk.Context, assetCode string) (types.Asset, bool) {
	assets := k.GetAssetParams(ctx)

	for i := range assets {
		if assets[i].AssetCode == assetCode {
			return assets[i], true
		}
	}
	return types.Asset{}, false

}
