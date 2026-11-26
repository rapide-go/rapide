package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/errs"
)

type createItemReq struct {
	Name string `json:"name" validate:"required,min=3"`
}

type itemResp struct {
	Name string `json:"name"`
}

func TestTypedRouteValidationAndErrs(t *testing.T) {
	r := router.New()
	router.PostTyped(r, "/items", func(c *ctx.Ctx, in createItemReq) (itemResp, error) {
		if in.Name == "taken" {
			return itemResp{}, errs.Conflict("item already exists").WithCode("ITEM_TAKEN")
		}
		c.Status(http.StatusCreated)
		return itemResp{Name: in.Name}, nil
	}).SetName("items.store")

	// 1. Validation failure -> 422
	req1 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString(`{"name":"a"}`))
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// 2. Domain errs.Conflict -> 409 with code ITEM_TAKEN
	req2 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString(`{"name":"taken"}`))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var errBody map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &errBody)
	if errBody["code"] != "ITEM_TAKEN" {
		t.Fatalf("expected code ITEM_TAKEN, got %v", errBody["code"])
	}

	// 3. Valid request -> 201
	req3 := httptest.NewRequest(http.MethodPost, "/items", bytes.NewBufferString(`{"name":"alphyx"}`))
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec3.Code, rec3.Body.String())
	}
}
