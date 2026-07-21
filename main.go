package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Name        string        `yaml:"name"`
	Bio         template.HTML `yaml:"bio"`
	Focus       string        `yaml:"focus"`
	Tagline     string        `yaml:"tagline"`
	Section     string        `yaml:"section"`
	Description string        `yaml:"description"`
	SiteURL     string        `yaml:"site_url"`
	SameAs      []string      `yaml:"same_as"`
	Photo       string        `yaml:"photo"`
	CtaText     string        `yaml:"cta_text"`
	CtaURL      string        `yaml:"cta_url"`
	Email       string        `yaml:"email"`
	Domain      string        `yaml:"domain"`
}

type FrontMatter struct {
	Title string    `yaml:"title"`
	Date  time.Time `yaml:"date"`
}

type Essay struct {
	Title       string
	Date        time.Time
	Slug        string
	Content     template.HTML
	URL         string
	AbsoluteURL string
	Description string
}

type EssayPage struct {
	Config     Config
	Essay      Essay
	PageTitle  string
	Canonical  string
	ImageURL   string
	SiteName   string
	JSONLD     template.JS
}

type IndexPage struct {
	Config     Config
	Essays     []Essay
	PageTitle  string
	Canonical  string
	ImageURL   string
	SiteName   string
	JSONLD     template.JS
}

func main() {
	cmd := "build"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "build":
		if err := build(); err != nil {
			log.Fatalf("build failed: %v", err)
		}
	case "serve":
		if err := build(); err != nil {
			log.Fatalf("build failed: %v", err)
		}
		fmt.Println("Serving blog at http://localhost:8000")
		log.Fatal(http.ListenAndServe(":8000", http.FileServer(http.Dir("site"))))
	default:
		log.Fatalf("unknown command %q (use build or serve)", cmd)
	}
}

func build() error {
	cfg, err := loadConfig("config.yaml")
	if err != nil {
		return err
	}

	tmpl, err := template.ParseFiles(
		"templates/essay.html",
		"templates/index.html",
	)
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	outputDir := "site"
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	essays, err := loadEssays("posts", cfg.SiteURL)
	if err != nil {
		return err
	}

	sort.Slice(essays, func(i, j int) bool {
		return essays[i].Date.After(essays[j].Date)
	})

	for i := range essays {
		if essays[i].Description == "" {
			essays[i].Description = fmt.Sprintf("%s — an essay by %s.", essays[i].Title, cfg.Name)
		}
	}

	for _, essay := range essays {
		if err := writeEssay(tmpl, outputDir, cfg, essay); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(outputDir, essay.Slug+".html"))
	}

	if err := writeIndex(tmpl, outputDir, cfg, essays); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", filepath.Join(outputDir, "index.html"))

	if err := writeCNAME(outputDir, cfg.Domain); err != nil {
		return err
	}

	if err := writeNoJekyll(outputDir); err != nil {
		return err
	}

	if err := writeRobots(outputDir, cfg.SiteURL); err != nil {
		return err
	}

	if err := writeSitemap(outputDir, cfg.SiteURL, essays); err != nil {
		return err
	}

	if err := writeLLMsTxt(outputDir, cfg, essays); err != nil {
		return err
	}

	if err := copyStatic("static", outputDir); err != nil {
		return err
	}

	fmt.Println("Blog generated successfully in the 'site' directory.")
	return nil
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Name == "" {
		cfg.Name = "Murthy"
	}
	cfg.Domain = normalizeDomain(cfg.Domain)
	cfg.SiteURL = strings.TrimSuffix(strings.TrimSpace(cfg.SiteURL), "/")
	if cfg.SiteURL == "" && cfg.Domain != "" {
		cfg.SiteURL = "https://" + cfg.Domain
	}
	if cfg.Description == "" {
		parts := []string{}
		if cfg.Focus != "" {
			parts = append(parts, cfg.Focus)
		}
		if cfg.Tagline != "" {
			parts = append(parts, cfg.Tagline)
		}
		cfg.Description = strings.Join(parts, ". ")
	}
	if cfg.CtaURL != "" {
		found := false
		for _, u := range cfg.SameAs {
			if u == cfg.CtaURL {
				found = true
				break
			}
		}
		if !found {
			cfg.SameAs = append(cfg.SameAs, cfg.CtaURL)
		}
	}
	return cfg, nil
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	return domain
}

func (c Config) Absolute(path string) string {
	path = strings.TrimPrefix(path, "/")
	if c.SiteURL == "" {
		return "/" + path
	}
	if path == "" {
		return c.SiteURL + "/"
	}
	return c.SiteURL + "/" + path
}

func (c Config) ImageURL() string {
	if c.Photo == "" {
		return ""
	}
	return c.Absolute(c.Photo)
}

func (c Config) HomeTitle() string {
	if c.Section != "" {
		return c.Name + " | " + c.Section
	}
	return c.Name
}

func (c Config) SiteName() string {
	if c.Section != "" {
		return c.Section
	}
	return c.Name
}

func writeCNAME(outputDir, domain string) error {
	path := filepath.Join(outputDir, "CNAME")
	if domain == "" {
		_ = os.Remove(path)
		return nil
	}
	if err := os.WriteFile(path, []byte(domain+"\n"), 0o644); err != nil {
		return fmt.Errorf("write CNAME: %w", err)
	}
	fmt.Printf("Generated %s (%s)\n", path, domain)
	return nil
}

func writeNoJekyll(outputDir string) error {
	path := filepath.Join(outputDir, ".nojekyll")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return fmt.Errorf("write .nojekyll: %w", err)
	}
	fmt.Printf("Generated %s\n", path)
	return nil
}

func writeRobots(outputDir, siteURL string) error {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	b.WriteString("\n")
	if siteURL != "" {
		b.WriteString("Sitemap: " + siteURL + "/sitemap.xml\n")
	}
	path := filepath.Join(outputDir, "robots.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write robots.txt: %w", err)
	}
	fmt.Printf("Generated %s\n", path)
	return nil
}

func writeSitemap(outputDir, siteURL string, essays []Essay) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	b.WriteString("  <url>\n")
	b.WriteString("    <loc>" + xmlEscape(siteURL+"/") + "</loc>\n")
	b.WriteString("    <changefreq>weekly</changefreq>\n")
	b.WriteString("    <priority>1.0</priority>\n")
	b.WriteString("  </url>\n")
	for _, essay := range essays {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>" + xmlEscape(essay.AbsoluteURL) + "</loc>\n")
		if !essay.Date.IsZero() {
			b.WriteString("    <lastmod>" + essay.Date.Format("2006-01-02") + "</lastmod>\n")
		}
		b.WriteString("    <changefreq>monthly</changefreq>\n")
		b.WriteString("    <priority>0.8</priority>\n")
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")
	path := filepath.Join(outputDir, "sitemap.xml")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write sitemap.xml: %w", err)
	}
	fmt.Printf("Generated %s\n", path)
	return nil
}

func writeLLMsTxt(outputDir string, cfg Config, essays []Essay) error {
	var b strings.Builder
	b.WriteString("# " + cfg.Name + "\n\n")
	if cfg.Section != "" {
		b.WriteString("> " + cfg.Section + " — personal essays by " + cfg.Name + ".\n\n")
	}
	if cfg.Description != "" {
		b.WriteString(cfg.Description + "\n\n")
	}
	if cfg.Focus != "" {
		b.WriteString(cfg.Focus + "\n\n")
	}
	b.WriteString("## Site\n\n")
	b.WriteString("- Home: " + cfg.SiteURL + "/\n")
	if cfg.Email != "" {
		b.WriteString("- Email: " + cfg.Email + "\n")
	}
	for _, u := range cfg.SameAs {
		b.WriteString("- " + u + "\n")
	}
	b.WriteString("\n## Essays\n\n")
	if len(essays) == 0 {
		b.WriteString("No essays published yet.\n")
	} else {
		for _, essay := range essays {
			line := "- [" + essay.Title + "](" + essay.AbsoluteURL + ")"
			if !essay.Date.IsZero() {
				line += ": " + essay.Date.Format("2006-01-02")
			}
			b.WriteString(line + "\n")
		}
	}
	path := filepath.Join(outputDir, "llms.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write llms.txt: %w", err)
	}
	fmt.Printf("Generated %s\n", path)
	return nil
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

func copyStatic(srcDir, outputDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read static: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		in := filepath.Join(srcDir, name)
		out := filepath.Join(outputDir, name)
		data, err := os.ReadFile(in)
		if err != nil {
			return fmt.Errorf("read %s: %w", in, err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
		fmt.Printf("Copied %s\n", out)
	}
	return nil
}

func loadEssays(dir, siteURL string) ([]Essay, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read posts: %w", err)
	}

	var essays []Essay
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		essay, err := parseEssay(filepath.Join(dir, entry.Name()), siteURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		essays = append(essays, essay)
	}
	return essays, nil
}

func parseEssay(path, siteURL string) (Essay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Essay{}, err
	}

	fm, body, err := splitFrontMatter(data)
	if err != nil {
		return Essay{}, err
	}

	slug := strings.TrimSuffix(filepath.Base(path), ".md")
	title := fm.Title
	if title == "" {
		title = slug
	}
	rel := slug + ".html"
	abs := rel
	if siteURL != "" {
		abs = siteURL + "/" + rel
	}

	return Essay{
		Title:       title,
		Date:        fm.Date,
		Slug:        slug,
		Content:     template.HTML(mdToHTML(body)),
		URL:         rel,
		AbsoluteURL: abs,
	}, nil
}

func splitFrontMatter(data []byte) (FrontMatter, []byte, error) {
	const delim = "---"
	trimmed := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !bytes.HasPrefix(trimmed, []byte(delim+"\n")) && !bytes.HasPrefix(trimmed, []byte(delim+"\r\n")) {
		return FrontMatter{}, trimmed, nil
	}

	rest := trimmed[len(delim):]
	if bytes.HasPrefix(rest, []byte("\r\n")) {
		rest = rest[2:]
	} else if bytes.HasPrefix(rest, []byte("\n")) {
		rest = rest[1:]
	}

	end := bytes.Index(rest, []byte("\n"+delim+"\n"))
	crlfEnd := bytes.Index(rest, []byte("\r\n"+delim+"\r\n"))
	sepLen := len("\n" + delim + "\n")
	if end < 0 || (crlfEnd >= 0 && crlfEnd < end) {
		end = crlfEnd
		sepLen = len("\r\n" + delim + "\r\n")
	}
	if end < 0 {
		alt := bytes.Index(rest, []byte("\n"+delim))
		if alt < 0 {
			return FrontMatter{}, nil, fmt.Errorf("missing closing frontmatter delimiter")
		}
		end = alt
		sepLen = len("\n" + delim)
		after := rest[end+sepLen:]
		after = bytes.TrimPrefix(after, []byte("\r\n"))
		after = bytes.TrimPrefix(after, []byte("\n"))
		var fm FrontMatter
		if err := yaml.Unmarshal(rest[:end], &fm); err != nil {
			return FrontMatter{}, nil, fmt.Errorf("frontmatter: %w", err)
		}
		return fm, after, nil
	}

	var fm FrontMatter
	if err := yaml.Unmarshal(rest[:end], &fm); err != nil {
		return FrontMatter{}, nil, fmt.Errorf("frontmatter: %w", err)
	}
	return fm, rest[end+sepLen:], nil
}

func mdToHTML(md []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)
	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank}
	renderer := html.NewRenderer(opts)
	return markdown.Render(doc, renderer)
}

func mustJSON(v any) template.JS {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(b)
}

func writeEssay(tmpl *template.Template, outputDir string, cfg Config, essay Essay) error {
	out, err := os.Create(filepath.Join(outputDir, essay.Slug+".html"))
	if err != nil {
		return err
	}
	defer out.Close()

	author := map[string]any{
		"@type": "Person",
		"name":  cfg.Name,
		"url":   cfg.SiteURL + "/",
	}
	if cfg.Email != "" {
		author["email"] = cfg.Email
	}
	posting := map[string]any{
		"@context":         "https://schema.org",
		"@type":            "BlogPosting",
		"headline":         essay.Title,
		"description":      essay.Description,
		"url":              essay.AbsoluteURL,
		"mainEntityOfPage": essay.AbsoluteURL,
		"author":           author,
		"publisher": map[string]any{
			"@type": "Person",
			"name":  cfg.Name,
		},
	}
	if !essay.Date.IsZero() {
		posting["datePublished"] = essay.Date.Format("2006-01-02")
		posting["dateModified"] = essay.Date.Format("2006-01-02")
	}
	if img := cfg.ImageURL(); img != "" {
		posting["image"] = img
	}

	page := EssayPage{
		Config:    cfg,
		Essay:     essay,
		PageTitle: essay.Title + " — " + cfg.Name,
		Canonical: essay.AbsoluteURL,
		ImageURL:  cfg.ImageURL(),
		SiteName:  cfg.SiteName(),
		JSONLD:    mustJSON(posting),
	}
	return tmpl.ExecuteTemplate(out, "essay.html", page)
}

func writeIndex(tmpl *template.Template, outputDir string, cfg Config, essays []Essay) error {
	out, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return err
	}
	defer out.Close()

	person := map[string]any{
		"@type":       "Person",
		"name":        cfg.Name,
		"url":         cfg.SiteURL + "/",
		"description": cfg.Description,
	}
	if cfg.Email != "" {
		person["email"] = cfg.Email
	}
	if cfg.Focus != "" {
		person["jobTitle"] = cfg.Focus
	}
	if len(cfg.SameAs) > 0 {
		person["sameAs"] = cfg.SameAs
	}
	if img := cfg.ImageURL(); img != "" {
		person["image"] = img
	}

	graph := []any{
		map[string]any{
			"@type":       "WebSite",
			"name":        cfg.SiteName(),
			"url":         cfg.SiteURL + "/",
			"description": cfg.Description,
			"author": map[string]any{
				"@type": "Person",
				"name":  cfg.Name,
			},
		},
		person,
	}
	ld := map[string]any{
		"@context": "https://schema.org",
		"@graph":   graph,
	}

	page := IndexPage{
		Config:    cfg,
		Essays:    essays,
		PageTitle: cfg.HomeTitle(),
		Canonical: cfg.SiteURL + "/",
		ImageURL:  cfg.ImageURL(),
		SiteName:  cfg.SiteName(),
		JSONLD:    mustJSON(ld),
	}
	return tmpl.ExecuteTemplate(out, "index.html", page)
}
