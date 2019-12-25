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
See the License for the specific language governing Permissions and
limitations under the License.

*/

package types

import (
	"github.com/xar-network/xar-network/pkg/matcheng"
	"github.com/xar-network/xar-network/pkg/serde"
	"github.com/xar-network/xar-network/types/store"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type MsgPost struct {
	Owner       sdk.AccAddress     `json:"owner" yaml:"owner"`
	MarketID    store.EntityID     `json:"market_id" yaml:"market_id"`
	Direction   matcheng.Direction `json:"direction" yaml:"direction"`
	Price       sdk.Uint           `json:"price" yaml:"price"`
	Quantity    sdk.Uint           `json:"quantity" yaml:"quantity"`
	TimeInForce uint16             `json:"time_in_force" yaml:"time_in_force"`
}

func NewMsgPost(owner sdk.AccAddress, marketID store.EntityID, direction matcheng.Direction, price sdk.Uint, quantity sdk.Uint, tif uint16) MsgPost {
	return MsgPost{
		Owner:       owner,
		MarketID:    marketID,
		Direction:   direction,
		Price:       price,
		Quantity:    quantity,
		TimeInForce: tif,
	}
}

func (msg MsgPost) Route() string {
	return "order"
}

func (msg MsgPost) Type() string {
	return "post"
}

func (msg MsgPost) ValidateBasic() sdk.Error {
	if !msg.MarketID.IsDefined() {
		return sdk.ErrUnauthorized("invalid market ID")
	}
	if msg.Price.IsZero() {
		return sdk.ErrInvalidCoins("price cannot be zero")
	}
	if msg.Quantity.IsZero() {
		return sdk.ErrInvalidCoins("quantity cannot be zero")
	}
	if msg.TimeInForce == 0 {
		return sdk.ErrInternal("time in force cannot be zero")
	}
	if msg.TimeInForce > MaxTimeInForce {
		return sdk.ErrInternal("time in force cannot be larger than 600")
	}
	return nil
}

func (msg MsgPost) GetSignBytes() []byte {
	return serde.MustMarshalSortedJSON(msg)
}

func (msg MsgPost) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Owner}
}

type MsgCancel struct {
	Owner   sdk.AccAddress `json:"owner" yaml:"owner"`
	OrderID store.EntityID `json:"order_id" yaml:"order_id"`
}

func NewMsgCancel(owner sdk.AccAddress, orderID store.EntityID) MsgCancel {
	return MsgCancel{
		Owner:   owner,
		OrderID: orderID,
	}
}

func (msg MsgCancel) Route() string {
	return "order"
}

func (msg MsgCancel) Type() string {
	return "cancel"
}

func (msg MsgCancel) ValidateBasic() sdk.Error {
	if msg.Owner.Empty() {
		return sdk.ErrUnauthorized("owner cannot be empty")
	}
	if !msg.OrderID.IsDefined() {
		return sdk.ErrInternal("invalid order ID")
	}
	return nil
}

func (msg MsgCancel) GetSignBytes() []byte {
	return serde.MustMarshalSortedJSON(msg)
}

func (msg MsgCancel) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Owner}
}
