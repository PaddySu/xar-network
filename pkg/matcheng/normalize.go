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
		err = errors.New("quantity too small to represent")
	}
	return res, err
}
