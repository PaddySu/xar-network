package cli

import (
	"bufio"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/xar-network/xar-network/pkg/cliutil"
	"github.com/xar-network/xar-network/pkg/matcheng"
	"github.com/xar-network/xar-network/types/store"
	"github.com/xar-network/xar-network/x/order/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func GetCmdPost(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "post [market-id] [direction] [price] [quantity] [time-in-force-blocks]",
		Short: "posts an order",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {

			inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx := client.NewCLIContext().WithCodec(cdc)
			accGetter := authtypes.NewAccountRetriever(cliCtx)
			if err := accGetter.EnsureExists(cliCtx.GetFromAddress()); err != nil {
				return err
			}
			bldr := authtypes.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))

			marketID := store.NewEntityIDFromString(args[0])
			var direction matcheng.Direction
			dirArg := strings.ToLower(args[1])
			if dirArg == "bid" {
				direction = matcheng.Bid
			} else if dirArg == "ask" {
				direction = matcheng.Ask
			} else {
				return errors.New("invalid direction")
			}

			price, err := sdk.ParseUint(args[2])
			if err != nil {
				return err
			}
			quantity, err := sdk.ParseUint(args[3])
			if err != nil {
				return err
			}
			tif, err := strconv.ParseUint(args[4], 10, 64)
			if err != nil {
				return err
			}
			if tif > math.MaxUint16 {
				return errors.New("time in force too large")
			}

			msg := types.NewMsgPost(cliCtx.GetFromAddress(), marketID, direction, price, quantity, uint16(tif))
			return cliutil.ValidateAndBroadcast(cliCtx, bldr, msg)
		},
	}
}

func GetCmdCancel(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel [order-id]",
		Short: "cancels an order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			inBuf := bufio.NewReader(cmd.InOrStdin())
			cliCtx := client.NewCLIContext().WithCodec(cdc)
			accGetter := authtypes.NewAccountRetriever(cliCtx)
			if err := accGetter.EnsureExists(cliCtx.GetFromAddress()); err != nil {
				return err
			}
			bldr := authtypes.NewTxBuilderFromCLI(inBuf).WithTxEncoder(utils.GetTxEncoder(cdc))

			orderID := store.NewEntityIDFromString(args[0])
			msg := types.NewMsgCancel(cliCtx.GetFromAddress(), orderID)
			return cliutil.ValidateAndBroadcast(cliCtx, bldr, msg)
		},
	}
}
