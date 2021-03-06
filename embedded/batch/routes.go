package batch

import (
	"fmt"
	"net/http"

	"github.com/xar-network/xar-network/embedded"

	"github.com/gorilla/mux"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/rest"
)

func RegisterRoutes(ctx context.CLIContext, r *mux.Router, cdc *codec.Codec) {
	r.Handle("/markets/{marketID}/batches", latestBatch(ctx, cdc)).Methods("GET")
}

func latestBatch(ctx context.CLIContext, cdc *codec.Codec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mktId := vars["marketID"]

		ctx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, ctx, r)
		if !ok {
			return
		}

		res, height, err := ctx.QueryWithData(fmt.Sprintf("custom/batch/latest/%s", mktId), nil)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		if res == nil {
			w.WriteHeader(404)
			return
		}
		ctx = ctx.WithHeight(height)
		embedded.PostProcessResponse(w, ctx, res)
	}
}
