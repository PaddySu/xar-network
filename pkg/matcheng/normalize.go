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

package matcheng

import (
	"errors"
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/xar-network/xar-network/pkg/conv"
)

const (
	AssetDecimals = 8
)

var divisor = sdk.NewDec(int64(math.Pow(float64(10), float64(AssetDecimals))))

func NormalizeQuoteQuantity(quotePrice sdk.Uint, baseQuantity sdk.Uint) (sdk.Uint, error) {
	quotePDec := sdk.NewDecFromBigInt(conv.SDKUint2Big(quotePrice))
	baseQDec := sdk.NewDecFromBigInt(conv.SDKUint2Big(baseQuantity))
	baseMult := baseQDec.Quo(divisor)
	res := sdk.NewUintFromBigInt(quotePDec.Mul(baseMult).TruncateInt().BigInt())
	var err error
	if res.IsZero() {
		err = errors.New("quantity too small to represent, setting to 0")
	}
	return res, err
}
