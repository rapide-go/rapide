# Rapide

**Expressive DX. Go 1.27. Single Binary.**

Rapide is a full-stack, zero-dependency Go 1.27 web framework inspired by **Laravel**, built from the ground up on Go 1.27 and the Go standard library.

Deploy an entire web application — HTTP server, generic ORM, SQL-backed queues & chains/batches, task scheduler, persistent sessions, cache, Sanctum API tokens, Socialite OAuth2, real-time Live Inertia props, auto-generated TypeScript routes & types, OpenAPI 3.1 docs, and SSE broadcasting — as **one compiled binary** with zero external infrastructure required to start.

---

## Installation

```bash
# Install the Rapide CLI
go install github.com/rapide-go/rapide/cmd/rapide@v0.1.1

# Or add Rapide to an existing Go 1.27 module
go get github.com/rapide-go/rapide@v0.1.1
```

Scaffold and run a new app:

```bash
rapide new my-app
cd my-app
rapide dev
```

---

## Quick Taste

```go
// app/http/requests/post_request.go
type CreatePostRequest struct {
    Title string `json:"title" validate:"required,min=3,max=140"`
    Body  string `json:"body" validate:"required,min=10"`
}

func (r *CreatePostRequest) PrepareForValidation() {
    r.Title = strings.TrimSpace(r.Title)
}

// app/http/controllers/post_controller.go
var Posts = &PostController{}

type PostController struct{}

func (ctrl *PostController) Index(c *ctx.Ctx) ([]models.Post, error) {
    return query.Query[models.Post]().Latest().Get()
}

func (ctrl *PostController) Store(c *ctx.Ctx, in requests.CreatePostRequest) (models.Post, error) {
    post, err := query.Query[models.Post]().Create(models.Post{
        Title: in.Title,
        Slug:  str.Slug(in.Title),
        Body:  in.Body,
    })
    if err != nil {
        return models.Post{}, err
    }

    cache.Forget("posts:all")
    events.Dispatch(events.PostPublished{Post: post})
    return post, nil
}

// routes/api.go
func RegisterAPI(r *router.Router) {
    r.Group("/api", func(api *router.Router) {
        api.Use(middleware.Throttle(60, time.Minute))
        api.GetTyped("/posts", controllers.Posts.Index).SetName("posts.index")
        api.PostTyped("/posts", controllers.Posts.Store).SetName("posts.store")
    })
}
```

---

## Minimal Bootstrap (`bootstrap/app.go`)

```go
func Configure() (*rapide.App, error) {
    return rapide.New(
        rapide.WithMiddleware(middleware.WebDefaults(
            inertia.MiddlewareFromConfig(inertia.ShareAuth(func(id any) (models.User, error) {
                return query.Query[models.User]().Find(id)
            })),
        )...),
        rapide.WithRoutes(routes.RegisterWeb, routes.RegisterAPI),
        rapide.WithHealth(),
        rapide.WithOpenAPI("My API", "1.0.0"),
        rapide.WithLiveInertia(),
        rapide.WithTypeGen(),
        rapide.WithProviders(Providers()...),
    )
}
```

---

## Signature Features

- **Zero External Dependencies** — 100% Go 1.27 standard library in core (`CGO_ENABLED=0` single-binary deploys).
- **Generic ORM & Query Builder** — `query.Query[models.Post]().Where("slug", slug).FirstOrFail()`, eager loading, `iter.Seq2` streaming cursors, reversible migrations, factories, and model observers.
- **Type-Safe Go → TypeScript Codegen (`http/typegen/`)** — Run `go run . gen:ts` (or `rapide.WithTypeGen()`) to generate `types.ts`, typed `routes.ts` (with `same-origin` credentials + `X-CSRF-TOKEN` forwarding), and `live.ts` straight from your Go routes & structs.
- **Real-Time Live Inertia Props (`/_inertia/live`)** — Mark props with `inertia.Live(stats, "dashboard", "post.created")` and bridge domain events via `inertia.LiveOnEvent[PostPublished]("dashboard", "post.created")` for automatic targeted partial reloads over native SSE (no WebSockets, no Redis).
- **Batteries Included** — Persistent SQL/memory sessions, cache (`RememberCoalesced`, `Touch`), SQL-backed queues + chains/batches, task scheduler, local/S3 storage (`os.OpenRoot` confined), SMTP/Resend mail, i18n, Sanctum API tokens, Socialite OAuth2, Gates/Policies, and `testkit` fakes.
- **Hardened by Default** — CSRF verification, session-fixation regeneration on login, open-redirect protection, bounded JSON/upload decoding, rate limiting (`http/ratelimit`), and security headers out of the box.

---

## CLI Highlights

```bash
rapide new <name>             # Scaffold a new application
rapide dev                    # Hot-reload dev server
rapide make:model Post -m -c  # Model + migration + controller
go run . migrate              # Run pending database migrations
go run . gen:ts               # Generate TypeScript types, routes & live client
go run . route:list           # Inspect all registered routes
rapide build                  # Build stripped zero-CGO production binary
```

---

## Documentation & Ecosystem

- **Full Documentation:** **[github.com/rapide-go/docs](https://github.com/rapide-go/docs)**
- **Inertia.js v3 Adapter:** [github.com/rapide-go/inertia-go](https://github.com/rapide-go/inertia-go)
- **Observability Dashboard:** [github.com/rapide-go/watch](https://github.com/rapide-go/watch)
- **Full-Stack Starter App:** [github.com/rapide-go/starter](https://github.com/rapide-go/starter)

---

## License

MIT — see [LICENSE](LICENSE).
