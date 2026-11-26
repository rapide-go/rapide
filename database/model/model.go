package model

import "time"

// Model provides standard Eloquent-style primary key and timestamp fields.
// Embed it in your domain structs:
//
//	type Post struct {
//	    model.Model
//	    Title string `db:"title"`
//	}
type Model struct {
	ID        int64     `db:"id" json:"id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// SoftDeletes adds a nullable deleted_at column for logical deletion.
type SoftDeletes struct {
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// Tabler lets a model override the default snake_case plural table name.
type Tabler interface {
	TableName() string
}

// Lifecycle hook interfaces (optional, invoked automatically by orm/query).
type Creating interface {
	BeforeCreate() error
}

type Updating interface {
	BeforeUpdate() error
}

type Deleting interface {
	BeforeDelete() error
}
