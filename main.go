package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
)

// Define a struct to hold the scrape result
type ScrapeResult struct {
	URL          string `json:"url"`
	ResponseTime string `json:"time"`
	H1Content    string `json:"h1"` // Field for H1 content
	HTMLContent  string `json:"html"`
}


func homePage(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome to the Go Backend Server!")
}

// Scraping function
func scrape(url string) (string, string) {
	c := colly.NewCollector()

	var h1Content string
	var htmlContent []byte // Change this to []byte

	// On every request, use Goquery to parse the HTML
	c.OnResponse(func(r *colly.Response) {
		// Store the full HTML content
		htmlContent = r.Body
	})

	// Handle visiting errors
	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Something went wrong:", err)
	})

	// Visit the website
	err := c.Visit(url)
	if err != nil {
		log.Fatal(err)
	}

	// After visiting, parse the HTML
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
	if err != nil {
		log.Println("Error parsing HTML:", err)
		return "", ""
	}

	// Extract the h1 content
	h1Content = doc.Find("h1[slot='title']").Text()

	return h1Content, string(htmlContent) // Return H1 content and HTML content as a string
}


// Function to write the ScrapeResult to a JSON file, replacing previous content
func writeResultToFile(result ScrapeResult) {
	// Open the file with O_TRUNC to erase previous content
	file, err := os.OpenFile("scrape_results.json", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false) // Disable HTML escaping

	// Marshal the ScrapeResult struct to JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling to JSON:", err)
		return
	}

	// Write the JSON data to the file, replacing previous content
	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Println("Error writing to file:", err)
	}
}


// Scrape API handler
func scrapeHandler(w http.ResponseWriter, r *http.Request) {
	// Get the "url" query parameter
	url := r.URL.Query().Get("url")

	fmt.Fprintf(w, "Welcome to the Go Backend Server!")

	if url == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	startTime := time.Now()

	h1Content, htmlContent := scrape(url)

	// End time after scraping
	elapsedTime := time.Since(startTime)

	result := ScrapeResult{
		URL:          url,
		ResponseTime: elapsedTime.String(),
		H1Content:    h1Content,
		HTMLContent:  htmlContent,
	}

	// Write the result to a JSON file
	writeResultToFile(result)

	// Return the h1 content and HTML as the response
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Scraped H1 from %s:\n\n%s\n\n", url, h1Content)
	fmt.Fprintf(w, "Scraped HTML:\n\n%s\n", htmlContent)
	fmt.Fprintf(w, "Response Time: %s", elapsedTime)
}


func handleRequests() {
    http.HandleFunc("/", homePage)
	http.HandleFunc("/scrape", scrapeHandler) // Scraping API route
    log.Fatal(http.ListenAndServe(":8080", nil))
}


func main() {
    fmt.Println("Starting server on port 8080...")
    handleRequests()
}
