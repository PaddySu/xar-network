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

package order

import (
	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/xar-network/xar-network/types/errs"
	"github.com/xar-network/xar-network/types/store"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	QueryList = "list"
)

func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) ([]byte, sdk.Error) {
		switch path[0] {
		case QueryList:
			return queryList(keeper, req.Data)
		default:
			return nil, sdk.ErrUnknownRequest("unknown embedded order request")
		}
	}
}

func getIterFunction(req ListQueryRequest) (ordersList *[]Order, lastIDptr *store.EntityID, iterCB func(order Order) bool) {
	orders := make([]Order, 0)
	var lastID store.EntityID

	iterCB = func(order Order) bool {
		// MarketID filter
		if len(req.MarketID) > 0 {
			present := false
			for _, marketID := range req.MarketID {
				if order.MarketID.Equals(marketID) {
					present = true
					break
				}
			}
			if !present {
				return true
			}
		}

		// Status filter
		if len(req.Status) > 0 {
			present := false
			for _, status := range req.Status {
				if order.Status == status {
					present = true
					break
				}
			}
			if !present {
				return true
			}
		}

		// Time filter
		if req.UnixTimeAfter != 0 {
			if order.CreatedTime.UnixNano() < req.UnixTimeAfter {
				return true
			}
		}
		if req.UnixTimeBefore != 0 {
			if order.CreatedTime.UnixNano() > req.UnixTimeBefore {
				return true
			}
		}

		orders = append(orders, order)
		lastID = order.ID
		return (req.Limit == 0) || (len(orders) < req.Limit)
	}

	return &orders, &lastID, iterCB
}

func queryList(keeper Keeper, reqB []byte) ([]byte, sdk.Error) {
	var req ListQueryRequest
	err := keeper.cdc.UnmarshalBinaryBare(reqB, &req)
	if err != nil {
		return nil, errs.ErrUnmarshalFailure("failed to unmarshal list query request")
	}

	ordersList, lastIDPtr, iterCB := getIterFunction(req)

	if req.Owner.Empty() {
		if req.Start.IsDefined() {
			keeper.ReverseIteratorFrom(req.Start, iterCB)
		} else {
			keeper.ReverseIterator(iterCB)
		}
	} else {
		// TEMPORARY: can add support for richer querying with sqlite
		keeper.OrdersByOwner(req.Owner, iterCB)
	}

	orders := *ordersList
	lastID := *lastIDPtr

	if (req.Limit == 0) || (len(orders) < req.Limit) {
		lastID = store.NewEntityID(0)
	}
	res := ListQueryResult{
		NextID: lastID.Dec(),
		Orders: orders,
	}
	b, err := codec.MarshalJSONIndent(keeper.cdc, res)
	if err != nil {
		return nil, sdk.ErrInternal("could not marshal result")
	}
	return b, nil
}
