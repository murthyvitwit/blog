// main.go
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
)

type Page struct {
	Title   string
	Content template.HTML
}

func main() {
	// Load the HTML layout template
	tmpl, err := template.ParseFiles("templates/layout.html")
	if err != nil {
		log.Fatalf("Error loading template: %v", err)
	}

	// Ensure the output directory exists
	outputDir := "site"
	os.MkdirAll(outputDir, os.ModePerm)

	// Process each Markdown file in the content directory
	contentDir := "posts"
	files, err := os.ReadDir(contentDir)
	if err != nil {
		log.Fatalf("Error reading content directory: %v", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			err := processMarkdownFile(filepath.Join(contentDir, file.Name()), tmpl, outputDir)
			if err != nil {
				log.Printf("Error processing file %s: %v", file.Name(), err)
			}
		}
	}

	fmt.Println("Blog generated successfully in the 'site' directory.")

	http.Handle("/", http.FileServer(http.Dir(outputDir)))
	fmt.Println("Serving blog at http://localhost:8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

// processMarkdownFile converts a Markdown file to HTML and saves it to the output directory
func processMarkdownFile(mdFilePath string, tmpl *template.Template, outputDir string) error {
	// Read the Markdown file
	mdData, err := os.ReadFile(mdFilePath)
	if err != nil {
		return fmt.Errorf("could not read file %s: %w", mdFilePath, err)
	}

	// Convert Markdown to HTML
	htmlContent := markdown.ToHTML(mdData, nil, nil)

	// Create a Page instance with title and content
	title := strings.TrimSuffix(filepath.Base(mdFilePath), ".md")
	page := Page{
		Title:   title,
		Content: template.HTML(htmlContent), // Be cautious with untrusted input
	}

	// Create the output HTML file path
	outputFilePath := filepath.Join(outputDir, title+".html")
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("could not create output file %s: %w", outputFilePath, err)
	}
	defer outputFile.Close()

	// Render the page content to the output file
	err = tmpl.Execute(outputFile, page)
	if err != nil {
		return fmt.Errorf("could not render template: %w", err)
	}

	fmt.Printf("Generated %s\n", outputFilePath)
	return nil
}
