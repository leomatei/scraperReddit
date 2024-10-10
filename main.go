package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/joho/godotenv"
)


func loadEnvVars() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("error loading .env file")
	}
	return nil
}
type CapSolverResponse struct {
    TaskID string `json:"taskId"` 
    Error  string `json:"error"`  
    Status string `json:"status"` 
}

type LoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type ScrapeResult struct {
	URL          string    `json:"url"`
	ResponseTime string    `json:"time"`
	H1Content    string    `json:"h1"`
	Comments     []Comment `json:"comments"`
	HTMLContent  string    `json:"html"`
}
type CapSolverResult struct {
    Status  string `json:"status"` 
    Solution struct {
        GRecaptchaResponse string `json:"gRecaptchaResponse"` 
    } `json:"solution"`
    Error string `json:"error"` 
}

type Comment struct {
	ID    string `json:"comment_id"`
	Body  string `json:"body"`
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the Go Backend Server!")
}


func extractCaptchaSiteKey(body []byte) (string, error) {
	// log.Println("here0",url)
	// resp, err := http.Get(url)
	// if err != nil {
	// 	return "", err
	// }
	// defer resp.Body.Close()
	log.Println("here1")


	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	// log.Println("here2")

	var siteKey string
	log.Println("here2",doc.Find("div.g-recaptcha"))
	doc.Find("div.g-recaptcha").Each(func(i int, s *goquery.Selection) {
		
		siteKey, _ = s.Attr("data-sitekey")
	})
log.Println("here3")
	if siteKey == "" {
		return "", fmt.Errorf("Captcha site key not found")
	}
log.Println("here4")
	return siteKey, nil
}

func solveCaptcha(siteKey, pageURL string) (string, error) {
	apiKey := os.Getenv("CAPSOLVER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("CAPSOLVER_API_KEY not set in environment variables")
	}
	fmt.Println("trying to solve captcha")
	requestPayload := map[string]interface{}{
		"clientKey": apiKey, 
		"task": map[string]interface{}{
			"type":       "ReCaptchaV2TaskProxyless",
			"websiteURL": pageURL,
			// "websiteKey": siteKey,
		},
	}



	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode CapSolver request: %v", err)
	}
	fmt.Println("trying to get payload",payloadBytes)


	resp, err := http.Post("https://api.capsolver.com/createTask", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to send CapSolver request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Println("response from createtask",resp,err)

	var capSolverResp CapSolverResponse
	if err := json.NewDecoder(resp.Body).Decode(&capSolverResp); err != nil {
		return "", fmt.Errorf("failed to decode CapSolver response: %v", err)
	}
	fmt.Println("cap solver resp",capSolverResp)

	return capSolverResp.TaskID, nil
}

func pollCaptchaSolution(taskID string) (string, error) {
	apiKey := os.Getenv("CAPSOLVER_API_KEY")
	for {
		requestPayload := map[string]interface{}{
			"clientKey": apiKey, 
			"taskId":    taskID,
		}

		payloadBytes, err := json.Marshal(requestPayload)
		if err != nil {
			return "", fmt.Errorf("failed to encode CapSolver request: %v", err)
		}

		resp, err := http.Post("https://api.capsolver.com/getTaskResult", "application/json", bytes.NewReader(payloadBytes))
		if err != nil {
			return "", fmt.Errorf("failed to send CapSolver result request: %v", err)
		}
		defer resp.Body.Close()

		var capSolverResult CapSolverResult
		if err := json.NewDecoder(resp.Body).Decode(&capSolverResult); err != nil {
			return "", fmt.Errorf("failed to decode CapSolver result response: %v", err)
		}

		if capSolverResult.Status == "ready" {
			return capSolverResult.Solution.GRecaptchaResponse, nil
		}

		time.Sleep(1 * time.Second)
	}
}

type LoginResponse struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Cookies     []*http.Cookie    `json:"cookies"`
	Body        string            `json:"body"`
}

func writeLoginResponseToFile(response LoginResponse) error {
	file, err := os.Create("login_response.json")
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") 

	err = encoder.Encode(response)
	if err != nil {
		return fmt.Errorf("error writing JSON to file: %v", err)
	}

	return nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var loginData LoginPayload

	err := json.NewDecoder(r.Body).Decode(&loginData)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	loginURL := "https://www.reddit.com/svc/shreddit/account/login"
	loginReq, err := http.NewRequest("POST", loginURL, bytes.NewReader([]byte(fmt.Sprintf("username=%s&password=%s", loginData.Username, loginData.Password))))
	if err != nil {
		http.Error(w, "Failed to send login request", http.StatusInternalServerError)
		return
	}

	loginReq.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{}
	loginResp, err := client.Do(loginReq)
	if err != nil {
		http.Error(w, "Failed to complete login request", http.StatusInternalServerError)
		return
	}
	defer loginResp.Body.Close()


	

	headers := make(map[string]string)
	for k, v := range loginResp.Header {
		headers[k] = strings.Join(v, ", ")
	}
	cookies := loginResp.Cookies()
	bodyBytes, err := io.ReadAll(loginResp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	loginResponse := LoginResponse{
		StatusCode: loginResp.StatusCode,
		Headers:    headers,
		Cookies:    cookies,
		Body:       string(bodyBytes),
	}

	err = writeLoginResponseToFile(loginResponse)
	if err != nil {
		http.Error(w, "Failed to write login response to file", http.StatusInternalServerError)
		return
	}

	if loginResp.StatusCode == http.StatusOK {
		w.Write([]byte("Login successful and response saved to file"))
	} else {
		http.Error(w, "Login failed", http.StatusUnauthorized)
	}
}

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
		if i >= 2 { 
			break
		}
		commentData := child.(map[string]interface{})["data"].(map[string]interface{})
		comment := Comment{
			ID:    commentData["id"].(string),
			Body:  commentData["body"].(string),
		}
		comments = append(comments, comment)
	}

	return comments, nil
}

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

func extractPostID(url string) (string, error) {
	parts := strings.Split(url, "/")

	for i, part := range parts {
		if part == "comments" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("could not extract post ID from URL")
}

func scrapeHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	startTime := time.Now()

	h1Content, htmlContent := scrape(url)
	htmlContent = strings.ReplaceAll(htmlContent, "\n", "")

	postID, err := extractPostID(url)
	if err != nil {
		http.Error(w, "Invalid Reddit post URL", http.StatusBadRequest)
		return
	}


	comments, commErr := fetchComments(postID) 
	if commErr != nil {
		log.Println("Error fetching comments:", commErr)
	}

	elapsedTime := time.Since(startTime)

	result := ScrapeResult{
		URL:          url,
		ResponseTime: elapsedTime.String(),
		H1Content:    h1Content,
		Comments:     comments,
		HTMLContent:  htmlContent,
	}
	writeResultToFile(result)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleRequests() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/scrape", scrapeHandler) 
	http.HandleFunc("/login", loginHandler) 
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	loadEnvVars()
	fmt.Println("Starting server on port 8080...")
	handleRequests()
}