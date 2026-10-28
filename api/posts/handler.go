package posts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// maxRequestBody caps a post save; the body text limit is far below it.
const maxRequestBody = 1 << 20

// Handler serves the posts endpoints.
type Handler struct {
	posts    *Service
	site     Publisher
	images   ImageSigner
	callerID func(*http.Request) string
	logger   *slog.Logger
}

// NewHandler returns the posts HTTP handlers. callerID returns the signed-in
// admin's user ID for a request.
func NewHandler(posts *Service, site Publisher, images ImageSigner, callerID func(*http.Request) string, logger *slog.Logger) *Handler {
	return &Handler{posts: posts, site: site, images: images, callerID: callerID, logger: logger}
}

type rebuild struct {
	Status string `json:"status"` // started, failed or skipped
}

type changeResponse struct {
	Post    *Post   `json:"post,omitempty"`
	Rebuild rebuild `json:"rebuild"`
}

// List serves GET /api/admin/posts.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ps, err := h.posts.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string][]Post{"posts": ps})
}

// Create serves POST /api/admin/posts.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var d Draft
	if !h.decode(w, r, &d) {
		return
	}
	d.PublishedAt = nil
	p, err := h.posts.Create(r.Context(), d, h.callerID(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/admin/posts/"+p.ID)
	writePost(w, http.StatusCreated, p)
}

// Get serves GET /api/admin/posts/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.posts.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writePost(w, http.StatusOK, p)
}

// Update serves PUT /api/admin/posts/{id}. The request must send If-Match
// with the ETag from when the post was loaded.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	version := parseETag(r.Header.Get("If-Match"))
	if version == "" {
		writeError(w, http.StatusPreconditionRequired, "send If-Match with the post's ETag")
		return
	}
	var d Draft
	if !h.decode(w, r, &d) {
		return
	}
	p, err := h.posts.Update(r.Context(), r.PathValue("id"), version, d, h.callerID(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	writePost(w, http.StatusOK, p)
}

// Publish serves POST /api/admin/posts/{id}/publish. It always asks for a
// rebuild, so publishing again pushes later edits of a live post to the site.
func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	p, err := h.posts.Publish(r.Context(), r.PathValue("id"), h.callerID(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.respondChanged(w, r, &p, true)
}

// Unpublish serves POST /api/admin/posts/{id}/unpublish.
func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	p, changed, err := h.posts.Unpublish(r.Context(), r.PathValue("id"), h.callerID(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.respondChanged(w, r, &p, changed)
}

// Delete serves DELETE /api/admin/posts/{id}. The site only needs rebuilding
// when the post was live.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p, err := h.posts.Delete(r.Context(), r.PathValue("id"), h.callerID(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.respondChanged(w, r, nil, p.Status == "published")
}

// Restore serves POST /api/admin/posts/{id}/restore.
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	p, err := h.posts.Restore(r.Context(), r.PathValue("id"), h.callerID(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.respondChanged(w, r, &p, p.Status == "published")
}

// respondChanged asks for a site rebuild when needed. A failed rebuild still
// answers 200: the change is saved, and the editor offers to try again.
func (h *Handler) respondChanged(w http.ResponseWriter, r *http.Request, p *Post, needsRebuild bool) {
	res := changeResponse{Post: p, Rebuild: rebuild{Status: "skipped"}}
	if needsRebuild {
		res.Rebuild.Status = "started"
		// The change is already saved, so a cancelled request must not stop the rebuild call.
		if err := h.site.Rebuild(context.WithoutCancel(r.Context())); err != nil {
			h.logger.Error("starting site rebuild", "err", err)
			res.Rebuild.Status = "failed"
		}
	}
	if p != nil {
		w.Header().Set("ETag", etag(*p))
	}
	writeJSON(w, http.StatusOK, res)
}

// PublicPost is a published post as the site build reads it.
type PublicPost struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	BodyMd      string `json:"bodyMd"`
	Cover       *Cover `json:"cover"`
	PublishedAt string `json:"publishedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// Cover is a post's cover image: a link valid for an hour, and its alt text.
type Cover struct {
	Key string `json:"key"`
	URL string `json:"url"`
	Alt string `json:"alt"`
}

// Published serves GET /api/posts for the site build.
func (h *Handler) Published(w http.ResponseWriter, r *http.Request) {
	ps, err := h.posts.Published(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]PublicPost, 0, len(ps))
	for _, p := range ps {
		pp := PublicPost{
			ID: p.ID, Title: p.Title, Description: p.Description, BodyMd: p.BodyMd, UpdatedAt: p.UpdatedAt,
		}
		if p.Slug != nil {
			pp.Slug = *p.Slug
		}
		if p.PublishedAt != nil {
			pp.PublishedAt = *p.PublishedAt
		}
		if p.CoverImageKey != nil {
			// Failing the whole list is safer than a build quietly dropping
			// images: a failed build leaves the current site up.
			url, err := h.images.SignedReadURL(r.Context(), *p.CoverImageKey)
			if err != nil {
				h.fail(w, err)
				return
			}
			pp.Cover = &Cover{Key: *p.CoverImageKey, URL: url}
			if p.CoverAlt != nil {
				pp.Cover.Alt = *p.CoverAlt
			}
		}
		out = append(out, pp)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string][]PublicPost{"posts": out})
}

// decode reads a JSON body into v, answering 400 itself if it can't.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "the request body isn't valid JSON for a post")
		return false
	}
	return true
}

// fail maps an error to a response, logging only unexpected ones.
func (h *Handler) fail(w http.ResponseWriter, err error) {
	var invalid *ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "that post doesn't exist")
	case errors.Is(err, ErrConflict):
		writeError(w, http.StatusConflict, "someone else saved this post since you opened it")
	case errors.As(err, &invalid):
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "some fields need fixing",
			"fields": invalid.Fields,
		})
	default:
		h.logger.Error("posts request failed", "err", err)
		writeError(w, http.StatusInternalServerError, "something went wrong")
	}
}

func writePost(w http.ResponseWriter, status int, p Post) {
	w.Header().Set("ETag", etag(p))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, p)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": msg})
}
