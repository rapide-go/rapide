package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/rapide-go/rapide/cache"
	"github.com/rapide-go/rapide/collections"
	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/database/model"
	"github.com/rapide-go/rapide/events"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/http/ctx"
	applog "github.com/rapide-go/rapide/log"
	"github.com/rapide-go/rapide/queue"
	"github.com/rapide-go/rapide/routing"
)

// --- Domain Model ---

type Post struct {
	model.Model
	Title string `db:"title" json:"title"`
	Body  string `db:"body" json:"body"`
}

// --- Typed Request DTO ---

type CreatePostRequest struct {
	Title string `json:"title" validate:"required,min=3,max=140"`
	Body  string `json:"body" validate:"required,min=10"`
}

// --- Typed Event & Background Job ---

type PostPublished struct {
	Post Post
}

type NotifySubscribersJob struct {
	PostID int64
	Title  string
}

func (j NotifySubscribersJob) Handle(ctx context.Context) error {
	applog.Info("sending notifications for published post", "post_id", j.PostID, "title", j.Title)
	return nil
}

// --- Typed Config ---

type AppConfig struct {
	Name string `env:"APP_NAME" default:"Rapide Blog"`
	Port int    `env:"APP_PORT" default:"8080"`
}

// --- Simple In-Memory Store for Demo ---

var (
	postsMu sync.RWMutex
	posts         = []Post{}
	nextID  int64 = 1
)

func main() {
	_ = config.LoadEnv(".env")
	cfg, err := config.Load[AppConfig]()
	if err != nil {
		log.Fatal(err)
	}

	// Register typed event listener
	events.Listen(func(e PostPublished) {
		queue.Dispatch(NotifySubscribersJob{
			PostID: e.Post.ID,
			Title:  e.Post.Title,
		})
	})

	r := router.New()

	// Request logging middleware
	r.Use(func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			start := time.Now()
			err := next(c)
			applog.Info("http", "method", c.Request.Method, "path", c.Request.URL.Path, "ms", time.Since(start).Milliseconds())
			return err
		}
	})

	r.Group("/api", func(api *router.Router) {
		// Typed output handler + generic cache.Remember facade
		api.Get("/posts", router.HandleOut(func(c *ctx.Ctx) ([]Post, error) {
			return cache.Remember("posts:all", 30*time.Second, func() ([]Post, error) {
				postsMu.RLock()
				defer postsMu.RUnlock()
				return collect.From(posts).Items(), nil
			})
		}))

		// Flagship generic handler: auto-decode, validate (422 JSON), execute, serialize
		api.Post("/posts", router.Handle(func(c *ctx.Ctx, in CreatePostRequest) (Post, error) {
			postsMu.Lock()
			post := Post{
				ID:        nextID,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
				Title:     in.Title,
				Body:      in.Body,
			}
			nextID++
			posts = append(posts, post)
			postsMu.Unlock()

			cache.Forget("posts:all")
			events.Dispatch(PostPublished{Post: post})

			c.Status(http.StatusCreated)
			return post, nil
		}))
	})

	app := foundation.New(
		cache.Provider{},
		queue.Provider{Workers: 4},
	)

	addr := fmt.Sprintf(":%d", cfg.Port)
	applog.Info("starting server", "app", cfg.Name, "addr", addr)
	if err := app.Serve(addr, r); err != nil {
		log.Fatal(err)
	}
}
