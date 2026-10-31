package main

import (
	"net/http"
)

// postRoutes is what the routes need from the posts handlers (*posts.Handler).
type postRoutes interface {
	List(http.ResponseWriter, *http.Request)
	Create(http.ResponseWriter, *http.Request)
	Get(http.ResponseWriter, *http.Request)
	Update(http.ResponseWriter, *http.Request)
	Delete(http.ResponseWriter, *http.Request)
	Publish(http.ResponseWriter, *http.Request)
	Unpublish(http.ResponseWriter, *http.Request)
	Restore(http.ResponseWriter, *http.Request)
	Published(http.ResponseWriter, *http.Request)
}

// adminRoutes is what the routes need from the admins handlers (*admins.Handler).
type adminRoutes interface {
	List(http.ResponseWriter, *http.Request)
	Invite(http.ResponseWriter, *http.Request)
	Resend(http.ResponseWriter, *http.Request)
	Remove(http.ResponseWriter, *http.Request)
	Restore(http.ResponseWriter, *http.Request)
}

// routes holds every handler the API serves and the checks guarding them.
type routes struct {
	events       http.Handler
	posts        postRoutes
	uploads      http.Handler
	latestDeploy http.Handler
	rebuild      http.Handler
	admins       adminRoutes

	admin        func(http.Handler) http.Handler // Auth0 token with manage:posts
	manageAdmins func(http.Handler) http.Handler // Auth0 token with manage:admins
	build        func(http.Handler) http.Handler // the site build's token
}

func newHandler(rt routes) http.Handler {
	// Everything under /api/admin/ sits behind one check, so a new admin
	// route can't be added unprotected and unknown paths reveal nothing.
	admin := http.NewServeMux()
	admin.HandleFunc("GET /api/admin/posts", rt.posts.List)
	admin.HandleFunc("POST /api/admin/posts", rt.posts.Create)
	admin.HandleFunc("GET /api/admin/posts/{id}", rt.posts.Get)
	admin.HandleFunc("PUT /api/admin/posts/{id}", rt.posts.Update)
	admin.HandleFunc("DELETE /api/admin/posts/{id}", rt.posts.Delete)
	admin.HandleFunc("POST /api/admin/posts/{id}/publish", rt.posts.Publish)
	admin.HandleFunc("POST /api/admin/posts/{id}/unpublish", rt.posts.Unpublish)
	admin.HandleFunc("POST /api/admin/posts/{id}/restore", rt.posts.Restore)
	admin.Handle("POST /api/admin/uploads", rt.uploads)
	admin.Handle("GET /api/admin/deploys/latest", rt.latestDeploy)
	admin.Handle("POST /api/admin/rebuild", rt.rebuild)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {})
	mux.Handle("GET /api/events", rt.events)
	mux.Handle("GET /api/posts", rt.build(http.HandlerFunc(rt.posts.Published)))
	mux.Handle("/api/admin/", rt.admin(admin))

	// Managing admins needs its own permission. These patterns are more
	// specific than /api/admin/, so they win over it; anything else under
	// /api/admin/admins still falls to the check above.
	adminsRoute := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, rt.manageAdmins(h)) }
	adminsRoute("GET /api/admin/admins", rt.admins.List)
	adminsRoute("POST /api/admin/admins", rt.admins.Invite)
	adminsRoute("POST /api/admin/admins/{id}/invite", rt.admins.Resend)
	adminsRoute("DELETE /api/admin/admins/{id}", rt.admins.Remove)
	adminsRoute("PUT /api/admin/admins/{id}", rt.admins.Restore)
	return mux
}
