package resource_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/http/resource"
)

type userModel struct {
	ID       int
	Name     string
	Password string
	IsAdmin  bool
}

type userDTO struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Badge *string `json:"badge,omitempty"`
}

func toUserDTO(u userModel) userDTO {
	return userDTO{
		ID:    u.ID,
		Name:  u.Name,
		Badge: resource.When(u.IsAdmin, "admin"),
	}
}

func TestResourceTransformations(t *testing.T) {
	item := resource.Make(userModel{ID: 1, Name: "Ada", Password: "secret", IsAdmin: true}, toUserDTO)
	raw, _ := json.Marshal(item)
	s := string(raw)
	if strings.Contains(s, "secret") {
		t.Fatalf("password leaked: %s", s)
	}
	if !strings.Contains(s, `"badge":"admin"`) {
		t.Fatalf("expected admin badge: %s", s)
	}

	page := query.Page[userModel]{
		Data:        []userModel{{ID: 2, Name: "Grace", IsAdmin: false}},
		Total:       1,
		PerPage:     15,
		CurrentPage: 1,
		LastPage:    1,
	}
	paginated := resource.FromPage(page, toUserDTO)
	if len(paginated.Data) != 1 || paginated.Data[0].Badge != nil || paginated.Meta.Total != 1 {
		t.Fatalf("unexpected paginated resource: %+v", paginated)
	}
}
