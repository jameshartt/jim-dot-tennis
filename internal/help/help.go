// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package help serves the user guides (docs/guide-*.md) from the app itself,
// so they match the installed version and work without GitHub. The Markdown
// is built into the binary (embed.go at the repository root) and rendered
// with goldmark, which leaves raw HTML out of the page.
//
// Access: the player sheet is public, because clubs send it round to people
// who have no account. The other guides are for people who run the club, so
// they need a signed-in account (any role, so that a Tournaments only account
// can read the tournaments guide). Nothing in them is secret.
package help

import (
	"bytes"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"jim-dot-tennis/internal/render"
)

// BasePath is where the guides are served.
const BasePath = "/help"

// Guide describes one guide.
type Guide struct {
	Slug    string // address: /help/<slug>
	Title   string
	Summary string // one line, for the contents page
	File    string // file name inside the guides file system
	Public  bool   // readable without signing in
}

// Path is the guide's address.
func (g Guide) Path() string { return BasePath + "/" + g.Slug }

// Guides lists every guide, in the order the contents page shows them.
func Guides() []Guide {
	return []Guide{
		{Slug: "admin", Title: "Administrator guide", File: "guide-admin.md",
			Summary: "First season, inviting people, player links, settings, a new season and match cards."},
		{Slug: "captain", Title: "Captain guide", File: "guide-captain.md",
			Summary: "The week: availability, picking the team, telling players, results and rescheduling."},
		{Slug: "player", Title: "Your link", File: "guide-player.md", Public: true,
			Summary: "A one-page sheet for players: availability, notifications and adding the app to a phone."},
		{Slug: "tournaments", Title: "Club championships", File: "guide-tournaments.md",
			Summary: "From switching tournaments on to publishing a draw."},
	}
}

// Find returns the guide with the slug.
func Find(slug string) (Guide, bool) {
	for _, g := range Guides() {
		if g.Slug == slug {
			return g, true
		}
	}
	return Guide{}, false
}

// Heading is one section heading, for the "On this page" list.
type Heading struct {
	ID   string
	Text string
}

// Rendered is a guide ready to show.
type Rendered struct {
	Title    string
	HTML     template.HTML
	Headings []Heading // the second-level headings
}

// Render turns a guide's Markdown into HTML. Links to another guide
// (guide-captain.md#heading) become links inside the app; links to any other
// .md file (the operator guides, which are not served here) become plain text
// with the file name beside them, because the address would lead nowhere.
func Render(src []byte) (Rendered, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(linkRewriter{}, 100)),
		),
	)
	doc := md.Parser().Parse(text.NewReader(src))

	var out Rendered
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		id, _ := h.AttributeString("id")
		idBytes, _ := id.([]byte)
		label := string(h.Text(src)) //nolint:staticcheck // plain text of the heading
		if h.Level == 1 && out.Title == "" {
			out.Title = label
		}
		if h.Level == 2 {
			out.Headings = append(out.Headings, Heading{ID: string(idBytes), Text: label})
		}
		return ast.WalkSkipChildren, nil
	})

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, src, doc); err != nil {
		return Rendered{}, err
	}
	out.HTML = template.HTML(buf.String()) //nolint:gosec // goldmark escapes raw HTML by default
	return out, nil
}

// linkRewriter changes the links that point at other documents.
type linkRewriter struct{}

func (linkRewriter) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	var links []*ast.Link
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if l, ok := n.(*ast.Link); ok && entering {
			links = append(links, l)
		}
		return ast.WalkContinue, nil
	})
	for _, l := range links {
		dest := string(l.Destination)
		base, frag, _ := strings.Cut(dest, "#")
		if !strings.HasSuffix(base, ".md") || strings.Contains(base, "://") {
			continue
		}
		if g, ok := guideForFile(base); ok {
			target := g.Path()
			if frag != "" {
				target += "#" + frag
			}
			l.Destination = []byte(target)
			continue
		}
		// Not served by the app: keep the words, drop the dead link.
		parent := l.Parent()
		for c := l.FirstChild(); c != nil; {
			next := c.NextSibling()
			parent.InsertBefore(parent, l, c)
			c = next
		}
		parent.InsertBefore(parent, l, ast.NewString([]byte(" (docs/"+base+")")))
		parent.RemoveChild(parent, l)
	}
}

func guideForFile(file string) (Guide, bool) {
	for _, g := range Guides() {
		if g.File == file {
			return g, true
		}
	}
	return Guide{}, false
}

// Anchors returns the heading ids of a guide, for tests and for checking the
// help links in the admin pages.
func Anchors(src []byte) ([]string, error) {
	r, err := Render(src)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(r.Headings))
	for _, h := range r.Headings {
		ids = append(ids, h.ID)
	}
	return ids, nil
}

// Handler serves the guides.
type Handler struct {
	files       fs.FS
	templateDir string
	templates   *render.Cache
}

// New returns the handler. files holds guide-*.md at its root (the server
// passes jimdottennis.Guides()); templateDir is the template tree's root.
func New(files fs.FS, templateDir string) *Handler {
	return &Handler{files: files, templateDir: templateDir, templates: render.NewCache()}
}

// RegisterRoutes adds the contents page (signed-in people) and the guides.
// requireAuth is auth.Middleware.RequireAuth in the server.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET "+BasePath, requireAuth(http.HandlerFunc(h.index)))
	guide := http.HandlerFunc(h.guide)
	protected := requireAuth(guide)
	mux.HandleFunc("GET "+BasePath+"/{slug}", func(w http.ResponseWriter, r *http.Request) {
		if g, ok := Find(r.PathValue("slug")); ok && g.Public {
			guide.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})
}

type indexData struct {
	Guides []Guide
}

type guideData struct {
	Guide    Guide
	Rendered Rendered
	Guides   []Guide
}

func (h *Handler) index(w http.ResponseWriter, _ *http.Request) {
	h.page(w, "help_index", "help/index.html", indexData{Guides: Guides()})
}

func (h *Handler) guide(w http.ResponseWriter, r *http.Request) {
	g, ok := Find(r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	src, err := fs.ReadFile(h.files, g.File)
	if err != nil {
		log.Printf("help: reading %s: %v", g.File, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	rendered, err := Render(src)
	if err != nil {
		log.Printf("help: rendering %s: %v", g.File, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.page(w, "help_guide", "help/guide.html", guideData{Guide: g, Rendered: rendered, Guides: Guides()})
}

func (h *Handler) page(w http.ResponseWriter, key, file string, data interface{}) {
	tmpl, err := h.templates.Get(key, func() (*template.Template, error) {
		funcMap := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
		return render.ParseFiles(template.New("").Funcs(funcMap),
			filepath.Join(h.templateDir, "layout.html"),
			filepath.Join(h.templateDir, file),
		)
	})
	if err != nil {
		log.Printf("help: parse templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render.Execute(w, tmpl, file, data); err != nil {
		log.Printf("help: %s: %v", file, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
