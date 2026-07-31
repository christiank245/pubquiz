package main

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v5"

	routeapi "pubquiz-website2.0/internal/api"
)

//go:embed web/templates web/public
var webFiles embed.FS

var tpl = template.Must(template.New("site").Funcs(template.FuncMap{
	"add1": func(i int) int { return i + 1 },
}).ParseFS(
	webFiles,
	"web/templates/*.html",
	"web/templates/components/*.html",
	"web/templates/pages/*.html",
))

func assetsSubFS() (fs.FS, error) {
	return fs.Sub(webFiles, "web/public")
}

func renderPage(c echo.Context, data routeapi.PageData) error {
	status := data.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout_start", data); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, data.PageTemplate, data); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "layout_end", data); err != nil {
		return err
	}
	return c.HTMLBlob(status, buf.Bytes())
}

func renderNotFoundPage(c echo.Context) error {
	data := routeapi.PageData{
		Title:        "404 - Page not found",
		PageTemplate: "not_found",
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout_start", data); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "not_found", data); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "layout_end", data); err != nil {
		return err
	}
	return c.HTMLBlob(http.StatusNotFound, buf.Bytes())
}

func renderRegisterPanel(c echo.Context, data routeapi.RegisterPanelData, htmx bool) error {
	var buf bytes.Buffer
	if htmx {
		if err := tpl.ExecuteTemplate(&buf, "register_panel", data); err != nil {
			return err
		}
		return c.HTMLBlob(http.StatusOK, buf.Bytes())
	}

	pageData := routeapi.PageData{
		Title:        "Quiz Registration",
		PageTemplate: "quiz",
		Quiz:         data.Quiz,
		Register:     data,
	}
	if err := tpl.ExecuteTemplate(&buf, "layout_start", pageData); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "quiz", pageData); err != nil {
		return err
	}
	if err := tpl.ExecuteTemplate(&buf, "layout_end", pageData); err != nil {
		return err
	}
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

func renderTemplate(c echo.Context, templateName string, data any) error {
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return err
	}
	return c.HTML(http.StatusOK, buf.String())
}
