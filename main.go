package main

import (
	"bytes"
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
	Name   string `yaml:"name"`
	Bio    string `yaml:"bio"`
	Domain string `yaml:"domain"`
}

type FrontMatter struct {
	Title string    `yaml:"title"`
	Date  time.Time `yaml:"date"`
}

type Essay struct {
	Title   string
	Date    time.Time
	Slug    string
	Content template.HTML
	URL     string
}

type EssayPage struct {
	Config Config
	Essay  Essay
}

type IndexPage struct {
	Config Config
	Essays []Essay
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

	essays, err := loadEssays("posts")
	if err != nil {
		return err
	}

	sort.Slice(essays, func(i, j int) bool {
		return essays[i].Date.After(essays[j].Date)
	})

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
	return cfg, nil
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	return domain
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

func loadEssays(dir string) ([]Essay, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read posts: %w", err)
	}

	var essays []Essay
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		essay, err := parseEssay(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		essays = append(essays, essay)
	}
	return essays, nil
}

func parseEssay(path string) (Essay, error) {
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

	return Essay{
		Title:   title,
		Date:    fm.Date,
		Slug:    slug,
		Content: template.HTML(mdToHTML(body)),
		URL:     slug + ".html",
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
		// closing --- at end of frontmatter line only
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

func writeEssay(tmpl *template.Template, outputDir string, cfg Config, essay Essay) error {
	out, err := os.Create(filepath.Join(outputDir, essay.Slug+".html"))
	if err != nil {
		return err
	}
	defer out.Close()
	return tmpl.ExecuteTemplate(out, "essay.html", EssayPage{Config: cfg, Essay: essay})
}

func writeIndex(tmpl *template.Template, outputDir string, cfg Config, essays []Essay) error {
	out, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return err
	}
	defer out.Close()
	return tmpl.ExecuteTemplate(out, "index.html", IndexPage{Config: cfg, Essays: essays})
}
