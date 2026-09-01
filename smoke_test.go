package testkit_test

import (
	"context"
	"testing"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/testkit"
	"github.com/nexssp/transport/thttp"
)

type ProductReq struct {
	ID       string  `json:"id" path:"id"`
	Email    string  `json:"contact_email" validate:"required,email"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Active   bool    `json:"is_active"`
}

func TestSmoke_ComplexPayloadGeneration(t *testing.T) {
	t.Parallel()

	getProd := action.New("product.get", func(_ context.Context, req ProductReq) (string, error) {
		return req.ID, nil
	}).Route(thttp.GET("/products/{id}")).Build()

	createProd := action.New("product.create", func(_ context.Context, req ProductReq) (ProductReq, error) {
		return req, nil
	}).Route(thttp.POST("/products")).Build()

	updateProd := action.New("product.update", func(_ context.Context, req ProductReq) (ProductReq, error) {
		return req, nil
	}).Route(thttp.PUT("/products/{id}")).Build()

	deleteProd := action.New("product.delete", func(_ context.Context, _ struct{}) (string, error) {
		return "deleted", nil
	}).Route(thttp.DELETE("/products/{id}")).Build()

	// Probes all 4 distinct REST routes with auto-synthesized payloads
	testkit.RunSmokeTests(t, []action.AnyAction{getProd, createProd, updateProd, deleteProd})
}
