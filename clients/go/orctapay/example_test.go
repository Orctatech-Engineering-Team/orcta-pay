package orctapay_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/orctatech/orcta-pay/clients/go/orctapay"
	"github.com/orctatech/orcta-pay/internal/money"
)

func ExampleClient_CreateCharge() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ref":"optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV","gateway":"hubtel","status":"pending"}`))
	}))
	defer srv.Close()

	_ = os.Setenv("ORCTA_PAY_URL", srv.URL)
	_ = os.Setenv("ORCTA_PAY_API_KEY", "test-key")

	client := orctapay.NewClient(os.Getenv("ORCTA_PAY_URL"), os.Getenv("ORCTA_PAY_API_KEY"))

	res, err := client.CreateCharge(context.Background(), orctapay.CreateChargeRequest{
		Product: "orctago",
		Amount:  money.New(1800, money.GHS),
		Wallet:  "0241234567",
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	switch v := res.(type) {
	case orctapay.ChargePending:
		fmt.Println("pending", v.Ref)
	case orctapay.ChargeSucceeded:
		fmt.Println("succeeded", v.Ref)
	case orctapay.ChargeFailed:
		fmt.Println("failed", v.Reason)
	}
	// Output: pending optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV
}
