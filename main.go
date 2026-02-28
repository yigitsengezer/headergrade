package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

//go:embed static/*
var staticFS embed.FS

type ScanRequest struct {
	TargetURL       string `json:"url"`
	FollowRedirects bool   `json:"follow_redirects"`
}

type HeaderInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Value       string   `json:"value,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type ScanResult struct {
	Grade          string       `json:"grade"`
	Site           string       `json:"site"`
	IPAddress      string       `json:"ip_address"`
	ReportTime     string       `json:"report_time"`
	MissingHeaders []HeaderInfo `json:"missing_headers"`
	PresentHeaders []HeaderInfo `json:"present_headers"`
	RawHeaders     [][]string   `json:"raw_headers"`
	Error          string       `json:"error,omitempty"`
}

var securityHeaders = map[string]string{
	"Strict-Transport-Security": "HTTP Strict Transport Security is an excellent feature to support on your site and strengthens your implementation of TLS by getting the User Agent to enforce the use of HTTPS.",
	"Content-Security-Policy":   "Content Security Policy is an effective measure to protect your site from XSS attacks. By whitelisting sources of approved content, you can prevent the browser from loading malicious assets.",
	"X-Frame-Options":           "X-Frame-Options tells the browser whether you want to allow your site to be framed or not. By preventing a browser from framing your site you can defend against attacks like clickjacking.",
	"X-Content-Type-Options":    "X-Content-Type-Options stops a browser from trying to MIME-sniff the content type and forces it to stick with the declared content-type. The only valid value for this header is \"nosniff\".",
	"Referrer-Policy":           "Referrer Policy is a new header that allows a site to control how much information the browser includes with navigations away from a document and should be set by all sites.",
	"Permissions-Policy":        "Permissions Policy is a new header that allows a site to control which features and APIs can be used in the browser.",
}

func calculateGrade(missingCount, totalCount int) string {
	score := totalCount - missingCount
	switch {
	case score == totalCount:
		return "A+"
	case score >= totalCount-1:
		return "A"
	case score >= totalCount-2:
		return "B"
	case score >= totalCount-3:
		return "C"
	case score >= totalCount-4:
		return "D"
	default:
		return "F"
	}
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(req.TargetURL, "http://") && !strings.HasPrefix(req.TargetURL, "https://") {
		req.TargetURL = "https://" + req.TargetURL
	}

	parsedURL, err := url.Parse(req.TargetURL)
	if err != nil {
		json.NewEncoder(w).Encode(ScanResult{Error: "Invalid URL format"})
		return
	}

	// Build TLS client with Chrome fingerprint
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(15),
		tls_client.WithClientProfile(profiles.Chrome_124),
	}
	if !req.FollowRedirects {
		options = append(options, tls_client.WithNotFollowRedirects())
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		json.NewEncoder(w).Encode(ScanResult{Error: "Failed to create TLS client: " + err.Error()})
		return
	}

	httpReq, err := fhttp.NewRequest("GET", req.TargetURL, nil)
	if err != nil {
		json.NewEncoder(w).Encode(ScanResult{Error: "Failed to create request"})
		return
	}
	httpReq.Header = fhttp.Header{
		"User-Agent":                {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"},
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Language":           {"en-US,en;q=0.9"},
		"Accept-Encoding":           {"gzip, deflate, br"},
		"Sec-CH-UA":                 {`"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`},
		"Sec-CH-UA-Mobile":          {"?0"},
		"Sec-CH-UA-Platform":        {`"Windows"`},
		"Sec-Fetch-Dest":            {"document"},
		"Sec-Fetch-Mode":            {"navigate"},
		"Sec-Fetch-Site":            {"none"},
		"Sec-Fetch-User":            {"?1"},
		"Upgrade-Insecure-Requests": {"1"},
		"Cache-Control":             {"max-age=0"},
		fhttp.HeaderOrderKey: {
			"user-agent",
			"accept",
			"accept-language",
			"accept-encoding",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform",
			"sec-fetch-dest",
			"sec-fetch-mode",
			"sec-fetch-site",
			"sec-fetch-user",
			"upgrade-insecure-requests",
			"cache-control",
		},
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		json.NewEncoder(w).Encode(ScanResult{Error: "Failed to reach the target URL: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// Try to get IP address
	ipAddress := ""
	host := parsedURL.Hostname()
	ips, err := net.LookupIP(host)
	if err == nil && len(ips) > 0 {
		ipAddress = ips[0].String()
	}

	result := ScanResult{
		Site:           req.TargetURL,
		IPAddress:      ipAddress,
		ReportTime:     time.Now().UTC().Format(time.RFC1123),
		MissingHeaders: []HeaderInfo{},
		PresentHeaders: []HeaderInfo{},
		RawHeaders:     [][]string{},
	}

	proto := resp.Proto
	if proto == "HTTP/2.0" {
		proto = "HTTP/2"
	}
	result.RawHeaders = append(result.RawHeaders, []string{
		proto,
		fmt.Sprintf("%d", resp.StatusCode),
	})

	var headerKeys []string
	for k := range resp.Header {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	rawMap := make(map[string]string)
	for _, k := range headerKeys {
		headerName := http.CanonicalHeaderKey(k)
		rawMap[headerName] = strings.Join(resp.Header[k], ", ")

		for _, v := range resp.Header[k] {
			result.RawHeaders = append(result.RawHeaders, []string{
				strings.ToLower(k),
				v,
			})
		}
	}

	for headerName, description := range securityHeaders {
		val, exists := rawMap[headerName]
		var warnings []string

		if exists && val != "" {
			if headerName == "Content-Security-Policy" {
				if strings.Contains(val, "'unsafe-inline'") {
					warnings = append(warnings, "This policy contains 'unsafe-inline' which is dangerous.")
				}
				if strings.Contains(val, "'unsafe-eval'") {
					warnings = append(warnings, "This policy contains 'unsafe-eval' which is dangerous.")
				}
			} else if headerName == "Strict-Transport-Security" {
				re := regexp.MustCompile(`max-age=(\d+)`)
				matches := re.FindStringSubmatch(val)
				if len(matches) > 1 {
					maxAge, _ := strconv.Atoi(matches[1])
					if maxAge < 2592000 {
						warnings = append(warnings, "The \"max-age\" directive is too small. The minimum recommended value is 2592000 (30 days).")
					}
				} else {
					warnings = append(warnings, "Could not find a valid \"max-age\" directive.")
				}
			}

			result.PresentHeaders = append(result.PresentHeaders, HeaderInfo{
				Name:        headerName,
				Description: description,
				Value:       val,
				Warnings:    warnings,
			})
		} else {
			result.MissingHeaders = append(result.MissingHeaders, HeaderInfo{
				Name:        headerName,
				Description: description,
			})
		}
	}

	result.Grade = calculateGrade(len(result.MissingHeaders), len(securityHeaders))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func main() {
	var port string
	flag.StringVar(&port, "p", "8002", "port to run the server on")
	flag.Parse()

	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}

	fileServer := http.FileServer(http.FS(subFS))
	http.Handle("/", fileServer)
	http.HandleFunc("/api/scan", handleScan)

	address := ":" + port
	fmt.Printf("Server is running. Visit: http://localhost%s\n", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
