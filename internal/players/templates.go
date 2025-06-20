package players

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"
)

// parseTemplate loads and parses a template file
func parseTemplate(templateDir, templateName string) (*template.Template, error) {
	templatePath := filepath.Join(templateDir, templateName)

	// Create template with common functions
	tmpl := template.New(filepath.Base(templateName)).Funcs(template.FuncMap{
		"currentYear": func() int {
			return time.Now().Year()
		},
		"formatDate": func(t time.Time) string {
			return t.Format("January 2, 2006")
		},
		"formatDateTime": func(t time.Time) string {
			return t.Format("January 2, 2006 at 3:04 PM")
		},
	})

	return tmpl.ParseFiles(templatePath)
}

// renderTemplate executes a template with the given data
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

// renderFallbackHTML renders a simple HTML page when templates are not available
func renderFallbackHTML(w http.ResponseWriter, title, heading, message, backLink string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background-color: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #2c5530; border-bottom: 2px solid #4a7c59; padding-bottom: 10px; }
        .back-link { display: inline-block; margin-top: 20px; color: #4a7c59; text-decoration: none; }
        .back-link:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>%s</h1>
        <p>%s</p>
        <a href="%s" class="back-link">← Back</a>
    </div>
</body>
</html>`, title, heading, message, backLink)

	w.Write([]byte(html))
}
