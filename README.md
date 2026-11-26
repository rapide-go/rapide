# Rapide

**Expressive DX. Go 1.27 Generics. Single Binary.**

Rapide is a full-stack, zero-dependency Go 1.27 web framework inspired by **Laravel**, built from the ground up on Go 1.27 generic methods, `net/http.ServeMux`, `reflect.TypeFor[T]()`, `new(expr)`, and `iter.Seq` iterators.

Deploy an entire web application — HTTP server, SQL-backed queues & chains/batches, task scheduler, persistent sessions, cache, Sanctum API tokens, Socialite OAuth2, i18n, embedded views, auto-generated OpenAPI 3.1 docs, and real-time SSE broadcasting — as **one compiled binary** with zero external infrastructure required to start.

---

## Quick Taste

```go
// Typed request validation + automatic 422 JSON errors + OpenAPI 3.1 schema
type CreatePostRequest struct {
    Title string `json:"title" validate:"required,min=3,max=140"`
    Body  string `json:"body" validate:"required,min=10"`
}

api.PostTyped("/posts", func(c *ctx.Ctx, in CreatePostRequest) (models.Post, error) {
    post, err := query.Query[models.Post]().Create(models.Post{
        Title: in.Title,
        Body:  in.Body,
    })
    if err != nil {
        return models.Post{}, err
    }

    cache.Forget("posts:all")
    events.Dispatch(PostPublished{Post: post})
    return post, nil
}).SetName("posts.store")
```

---

## Documentation Manual

Browse the full 39-page feature-by-feature manual starting at **[../docs/index.md](../docs/index.md)** (with website-ready **[../docs/sidebar.json](../docs/sidebar.json)** for VitePress / Astro Starlight / Docusaurus / MkDocs):

- **Getting Started:** [Installation](../docs/getting-started/installation.md) · [Configuration](../docs/getting-started/configuration.md) · [Directory Structure](../docs/getting-started/structure.md) · [Artisan CLI](../docs/getting-started/cli.md)
- **Architecture:** [Service Container](../docs/architecture/container.md) · [Service Providers](../docs/architecture/providers.md) · [Lifecycle](../docs/architecture/lifecycle.md)
- **The Basics:** [Routing](../docs/basics/routing.md) · [Controllers](../docs/basics/controllers.md) · [Middleware](../docs/basics/middleware.md) · [Requests & Validation](../docs/basics/requests-and-validation.md) · [Responses & Resources](../docs/basics/responses-and-resources.md) · [Views & Vite](../docs/basics/views-and-vite.md) · [Inertia.js v3](../docs/basics/inertia.md)
- **Database & ORM:** [Getting Started](../docs/database/getting-started.md) · [Models & Observers](../docs/database/models-and-observers.md) · [Query Builder](../docs/database/query-builder.md) · [Relationships & Eager Loading](../docs/database/relationships.md) · [Migrations](../docs/database/migrations.md) · [Factories & Seeding](../docs/database/factories-and-seeding.md)
- **Security & Auth:** [Authentication](../docs/security/authentication.md) · [Sanctum](../docs/security/sanctum.md) · [Socialite](../docs/security/socialite.md) · [Authorization](../docs/security/authorization.md) · [Encryption & Hashing](../docs/security/encryption-and-hashing.md) · [Hardening](../docs/security/hardening.md)
- **Digging Deeper:** [Sessions](../docs/deeper/sessions.md) · [Cache](../docs/deeper/cache.md) · [Storage](../docs/deeper/storage.md) · [Queues](../docs/deeper/queues.md) · [Scheduling](../docs/deeper/scheduling.md) · [Mail](../docs/deeper/mail.md) · [Events](../docs/deeper/events.md) · [Broadcasting](../docs/deeper/broadcasting.md) · [Localization](../docs/deeper/localization.md) · [Logging](../docs/deeper/logging.md) · [Testing & Fakes](../docs/deeper/testing.md) · [Deployment](../docs/deeper/deployment.md)
