/*

Copyright 2019 All in Bits, Inc
Copyright 2019 Xar Network

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package order_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xar-network/xar-network/pkg/matcheng"
	"github.com/xar-network/xar-network/testutil"
	"github.com/xar-network/xar-network/testutil/mockapp"
	"github.com/xar-network/xar-network/testutil/testflags"
	"github.com/xar-network/xar-network/types/errs"
	"github.com/xar-network/xar-network/types/store"
	"github.com/xar-network/xar-network/x/denominations"
	types2 "github.com/xar-network/xar-network/x/market/types"
	types4 "github.com/xar-network/xar-network/x/order/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/supply"
)

type testCtx struct {
	ctx      sdk.Context
	marketID store.EntityID
	owner    sdk.AccAddress
	buyer    sdk.AccAddress
	seller   sdk.AccAddress
	app      *mockapp.MockApp
	asset1   string
	asset2   string
	market   types2.Market
}

func TestKeeper_Post(t *testing.T) {
	testflags.UnitTest(t)
	t.Run("returns an error for a nonexistent market", func(t *testing.T) {
		ctx := setupTest(t)
		_, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID.Inc(), matcheng.Bid, testutil.ToBaseUnits(1), testutil.ToBaseUnits(10), 599)
		assert.Error(t, err)
		assert.Equal(t, err.Code(), errs.CodeNotFound)
	})
	t.Run("returns an error if buying more than owned coins", func(t *testing.T) {
		ctx := setupTest(t)
		_, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, testutil.ToBaseUnits(2), testutil.ToBaseUnits(5001), 599)
		assert.Error(t, err)
		assert.Equal(t, err.Code(), sdk.CodeInsufficientCoins)
	})
	t.Run("returns an error if selling more than owned coins", func(t *testing.T) {
		ctx := setupTest(t)
		_, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.seller, ctx.marketID, matcheng.Ask, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10001), 599)
		assert.Error(t, err)
		assert.Equal(t, err.Code(), sdk.CodeInsufficientCoins)
	})
	t.Run("returns an error if trying to post a non-representable order", func(t *testing.T) {
		ctx := setupTest(t)
		_, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.seller, ctx.marketID, matcheng.Bid, sdk.NewUint(2), sdk.NewUint(2), 599)
		assert.Error(t, err)
		assert.Equal(t, err.Code(), sdk.CodeInvalidCoins)
	})
	t.Run("creates the order", func(t *testing.T) {
		ctx := setupTest(t)
		created, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, testutil.ToBaseUnits(1), testutil.ToBaseUnits(10), 599)
		require.NoError(t, err)
		retrieved, err := ctx.app.OrderKeeper.Get(ctx.ctx, created.ID)
		require.NoError(t, err)
		assert.EqualValues(t, created, retrieved)
	})
	t.Run("debits the correct coins", func(t *testing.T) {
		ctx := setupTest(t)
		_, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10), 599)
		require.NoError(t, err)
		_, err = ctx.app.OrderKeeper.Post(ctx.ctx, ctx.seller, ctx.marketID, matcheng.Ask, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10), 599)
		require.NoError(t, err)
	})
	t.Run("test correct frosen", testFrozen)
}

func testFrozen(t *testing.T) {
	ctx := setupTest(t)
	price := testutil.ToBaseUnits(2)
	quantity := testutil.ToBaseUnits(10)
	fe, _ := matcheng.NormalizeQuoteQuantity(price, quantity)
	frozenExpected, _ := sdk.NewIntFromString(fe.String())

	_, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, price, quantity, 599)
	require.NoError(t, err)

	cns := ctx.app.OrderKeeper.GetFrozenCoins(ctx.ctx)
	require.True(t, len(cns) > 0)
	require.True(t, cns[0].Amount.Equal(frozenExpected))
	log.Println()
}

func TestKeeper_Cancel(t *testing.T) {
	testflags.UnitTest(t)
	t.Run("returns an error for a nonexistent order", func(t *testing.T) {
		ctx := setupTest(t)
		_, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, store.NewEntityID(0), matcheng.Bid, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10), 599)
		assert.Error(t, err)
		assert.Equal(t, err.Code(), errs.CodeNotFound)
	})
	t.Run("deletes the order and returns coins after cancellation", func(t *testing.T) {
		ctx := setupTest(t)
		bid, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10), 599)
		require.NoError(t, err)
		err = ctx.app.OrderKeeper.Cancel(ctx.ctx, bid.ID)
		require.NoError(t, err)
		assert.False(t, ctx.app.OrderKeeper.Has(ctx.ctx, bid.ID))
	})
}

func TestKeeper_Iteration(t *testing.T) {
	testflags.UnitTest(t)
	ctx := setupTest(t)
	first, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10), 599)
	require.NoError(t, err)
	_, err = ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10), 599)
	require.NoError(t, err)
	last, err := ctx.app.OrderKeeper.Post(ctx.ctx, ctx.buyer, ctx.marketID, matcheng.Bid, testutil.ToBaseUnits(2), testutil.ToBaseUnits(10), 599)
	require.NoError(t, err)

	var coll []store.EntityID
	ctx.app.OrderKeeper.Iterator(ctx.ctx, func(order types4.Order) bool {
		if order.ID.Equals(last.ID) {
			return false
		}
		coll = append(coll, order.ID)
		return true
	})
	assert.EqualValues(t, []store.EntityID{store.NewEntityID(1), store.NewEntityID(2)}, coll)

	coll = make([]store.EntityID, 0)
	ctx.app.OrderKeeper.ReverseIterator(ctx.ctx, func(order types4.Order) bool {
		if order.ID.Equals(first.ID) {
			return false
		}
		coll = append(coll, order.ID)
		return true
	})
	assert.EqualValues(t, []store.EntityID{store.NewEntityID(3), store.NewEntityID(2)}, coll)
}

func setupTest(t *testing.T) *testCtx {
	app := mockapp.New(t)
	nominee := testutil.RandAddr()
	buyer := testutil.RandAddr()
	seller := testutil.RandAddr()

	app.SupplyKeeper.SetSupply(app.Ctx, supply.NewSupply(sdk.Coins{}))
	marketParams := app.MarketKeeper.GetParams(app.Ctx)
	marketParams.Nominees = []string{nominee.String()}
	app.MarketKeeper.SetParams(app.Ctx, marketParams)

	err := app.SupplyKeeper.MintCoins(app.Ctx, denominations.ModuleName, sdk.NewCoins(sdk.NewCoin("tst1", sdk.NewInt(1000000000000)), sdk.NewCoin("tst2", sdk.NewInt(1000000000000))))
	require.NoError(t, err)
	require.NoError(t, app.SupplyKeeper.SendCoinsFromModuleToAccount(app.Ctx, denominations.ModuleName, buyer, sdk.NewCoins(sdk.NewCoin("tst1", sdk.NewInt(10000000000)))))
	require.NoError(t, app.SupplyKeeper.SendCoinsFromModuleToAccount(app.Ctx, denominations.ModuleName, buyer, sdk.NewCoins(sdk.NewCoin("tst2", sdk.NewInt(10000000000)))))
	require.NoError(t, app.SupplyKeeper.SendCoinsFromModuleToAccount(app.Ctx, denominations.ModuleName, seller, sdk.NewCoins(sdk.NewCoin("tst1", sdk.NewInt(10000000000)))))
	require.NoError(t, app.SupplyKeeper.SendCoinsFromModuleToAccount(app.Ctx, denominations.ModuleName, seller, sdk.NewCoins(sdk.NewCoin("tst2", sdk.NewInt(10000000000)))))
	market := types2.NewMsgCreateMarket(nominee, "tst1", "tst2")
	mkt, err := app.MarketKeeper.CreateMarket(app.Ctx, market.Nominee.String(), market.BaseAsset, market.QuoteAsset)
	require.NoError(t, err)

	marketParams = app.MarketKeeper.GetParams(app.Ctx)

	return &testCtx{
		ctx:      app.Ctx,
		marketID: mkt.ID,
		buyer:    buyer,
		seller:   seller,
		app:      app,
		asset1:   "tst1",
		asset2:   "tst2",
		market:   mkt,
	}
}
