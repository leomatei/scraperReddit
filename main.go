package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
)

type ScrapeResult struct {
	URL          string    `json:"url"`
	ResponseTime string    `json:"time"`
	H1Content    string    `json:"h1"`
	Comments     []Comment `json:"comments"`
	HTMLContent  string    `json:"html"`
}

type Comment struct {
	ID    string `json:"comment_id"`
	Body  string `json:"body"`
	Depth int    `json:"depth"`
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the Go Backend Server!")
}

// Scraping function to get H1 content and full HTML
func scrape(url string) (string, string) {
	c := colly.NewCollector()

	var htmlContent []byte
	c.OnResponse(func(r *colly.Response) {
		htmlContent = r.Body
	})

	err := c.Visit(url)
	if err != nil {
		log.Fatal(err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
	if err != nil {
		log.Println("Error parsing HTML:", err)
		return "", ""
	}

	h1Content := doc.Find("h1[slot='title']").Text()
	return h1Content, string(htmlContent)
}

// Fetch the first two top-level comments from Reddit
func fetchComments(postID string) ([]Comment, error) {
	url := fmt.Sprintf("https://www.reddit.com/comments/%s.json", postID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rawResponse []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, err
	}

	if len(rawResponse) < 2 {
		return nil, fmt.Errorf("unexpected response format")
	}

	commentsData := rawResponse[1].(map[string]interface{})["data"].(map[string]interface{})["children"].([]interface{})
	var comments []Comment
	for i, child := range commentsData {
		if i >= 2 { // Limit to 2 comments
			break
		}
		commentData := child.(map[string]interface{})["data"].(map[string]interface{})
		comment := Comment{
			ID:    commentData["id"].(string),
			Body:  commentData["body"].(string),
			Depth: int(commentData["depth"].(float64)),
		}
		comments = append(comments, comment)
	}

	return comments, nil
}

// Function to write ScrapeResult to a JSON file
func writeResultToFile(result ScrapeResult) {
	file, err := os.Create("scrape_results.json")
	if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		log.Println("Error writing JSON to file:", err)
	}
}

// Function to extract the post ID from the Reddit URL
func extractPostID(url string) (string, error) {
	// Example Reddit post URL structure: https://www.reddit.com/r/gaming/comments/1fz7efj/...
	// Split the URL into parts based on "/"
	parts := strings.Split(url, "/")

	// The post ID will be the part after "comments"
	for i, part := range parts {
		if part == "comments" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("could not extract post ID from URL")
}

// Scrape API handler
func scrapeHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	startTime := time.Now()

	// Perform scraping
	h1Content, htmlContent := scrape(url)
	htmlContent = strings.ReplaceAll(htmlContent, "\n", "")

	postID, err := extractPostID(url)
	if err != nil {
		http.Error(w, "Invalid Reddit post URL", http.StatusBadRequest)
		return
	}


	// Get comments from Reddit post
	comments, commErr := fetchComments(postID) 
	if commErr != nil {
		log.Println("Error fetching comments:", commErr)
	}

	// Calculate response time
	elapsedTime := time.Since(startTime)

	// Create a result and write to file
	result := ScrapeResult{
		URL:          url,
		ResponseTime: elapsedTime.String(),
		H1Content:    h1Content,
		Comments:     comments,
		HTMLContent:  htmlContent,
	}
	writeResultToFile(result)

	// Respond with scraped data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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