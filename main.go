package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
//
// Required:
//
//	CF_API_TOKEN      - Cloudflare API token
//	CF_ZONE_ID        - Cloudflare Zone ID
//	CF_RECORD_NAME    - DNS record name, e.g. home.example.com
//	CF_RECORD_TYPE    - DNS record type: A or AAAA
//
// Optional:
//
//	CF_CHECK_INTERVAL - Check interval in seconds (default 300)
//	CF_TTL            - DNS TTL in seconds
//	CF_PROXIED        - Proxy through Cloudflare: true or false
type Config struct {
	APIToken             string
	ZoneId               string
	RecordName           string
	RecordType           string
	CheckIntervalSeconds uint
	TTL                  *int
	Proxied              *bool
}

// CloudflareError represents a single error from the Cloudflare API
type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e CloudflareError) Error() string {
	return fmt.Sprintf("code=%d message=%s", e.Code, e.Message)
}

// CloudflareResponse is used for GET dns_records (result is an array)
type CloudflareResponse struct {
	Success bool              `json:"success"`
	Errors  []CloudflareError `json:"errors"`
	Result  []DNSRecord       `json:"result"`
}

// CloudflareBaseResponse is used for PUT dns_records (result is a single object, ignored)
type CloudflareBaseResponse struct {
	Success bool              `json:"success"`
	Errors  []CloudflareError `json:"errors"`
}

// DNSRecord struct for DNS record
type DNSRecord struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied *bool  `json:"proxied,omitempty"`
}

// updatePayload is the request body for Cloudflare DNS record updates
type updatePayload struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied *bool  `json:"proxied,omitempty"`
}

var lastReqURL string
var lastIP string

func main() {
	var runOnce bool
	flag.BoolVar(&runOnce, "once", false, "run once and exit")
	flag.Parse()

	config, err := loadConfigFromEnv()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	if err := validateConfig(&config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	if config.CheckIntervalSeconds == 0 {
		config.CheckIntervalSeconds = 300
		log.Printf("CF_CHECK_INTERVAL not set, using default %d seconds", config.CheckIntervalSeconds)
	}

	log.Println("DDNS client started...")
	log.Printf("Monitoring domain: %s", config.RecordName)
	log.Printf("Check interval: %d seconds", config.CheckIntervalSeconds)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(config.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()

	runUpdate(&config)
	if runOnce {
		log.Print("Single run completed, program exiting.")
		return
	}

	times := 1
	for {
		select {
		case <-ticker.C:
			log.Printf("Scheduled check triggered %d ...\n", times)
			runUpdate(&config)
			times++
		case s := <-sigCh:
			log.Printf("Received signal %v, exiting.\n", s)
			return
		}
	}
}

// runUpdate is the core update logic
func runUpdate(config *Config) {
	log.Println("---------------------------------")
	log.Println("Starting IP address check...")

	publicIP, err := getPublicIP(config.RecordType)
	if err != nil {
		log.Printf("Error: Failed to get public IP: %v", err)
		return
	}
	log.Printf("Current public IP: %s", publicIP)

	if lastIP != "" && publicIP == lastIP {
		log.Println("IP is the same as last time, skipping Cloudflare check.")
		log.Println("---------------------------------")
		return
	}

	record, err := getDNSRecord(config)
	if err != nil {
		log.Printf("Error: Failed to get Cloudflare DNS record: %v", err)
		return
	}
	log.Printf("Cloudflare record IP: %s", record.Content)

	if publicIP == record.Content {
		log.Println("IP address unchanged, no update needed.")
		lastIP = publicIP
	} else {
		log.Printf("IP address changed from %s to %s. Updating...", record.Content, publicIP)
		if err := updateDNSRecord(config, record, publicIP); err != nil {
			log.Printf("Error: Failed to update DNS record: %v", err)
		} else {
			log.Println("DNS record updated successfully!")
			lastIP = publicIP
		}
	}
	log.Println("---------------------------------")
}

// loadConfigFromEnv reads all configuration from environment variables.
func loadConfigFromEnv() (Config, error) {
	var config Config

	config.APIToken = os.Getenv("CF_API_TOKEN")
	config.ZoneId = os.Getenv("CF_ZONE_ID")
	config.RecordName = os.Getenv("CF_RECORD_NAME")
	config.RecordType = os.Getenv("CF_RECORD_TYPE")

	if v := os.Getenv("CF_CHECK_INTERVAL"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return config, fmt.Errorf("invalid CF_CHECK_INTERVAL %q: %w", v, err)
		}
		config.CheckIntervalSeconds = uint(n)
	}
	if v := os.Getenv("CF_TTL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return config, fmt.Errorf("invalid CF_TTL %q: %w", v, err)
		}
		config.TTL = &n
	}
	if v := os.Getenv("CF_PROXIED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return config, fmt.Errorf("invalid CF_PROXIED %q (use true or false): %w", v, err)
		}
		config.Proxied = &b
	}

	return config, nil
}

// validateConfig checks that required fields are present and valid
func validateConfig(config *Config) error {
	if config.APIToken == "" {
		return fmt.Errorf("CF_API_TOKEN is required")
	}
	if config.ZoneId == "" {
		return fmt.Errorf("CF_ZONE_ID is required")
	}
	if config.RecordName == "" {
		return fmt.Errorf("CF_RECORD_NAME is required")
	}
	if config.RecordType != "A" && config.RecordType != "AAAA" {
		return fmt.Errorf("CF_RECORD_TYPE must be 'A' or 'AAAA', got %q", config.RecordType)
	}
	return nil
}

// getPublicIP gets public IP from external services
func getPublicIP(recordType string) (string, error) {
	var urls []string
	switch recordType {
	case "AAAA":
		urls = []string{
			"https://ipv6.icanhazip.com",
			"https://ifconfig.co/ip?v=6",
			"https://api6.ipify.org",
		}
	default:
		urls = []string{
			"https://ipv4.icanhazip.com",
			"https://ifconfig.co/ip?v=4",
			"https://api.ipify.org",
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}

	if lastReqURL != "" {
		if ip := httpGetIP(lastReqURL, recordType, client); ip != "" {
			log.Printf("Successfully got IP using last service: %s", lastReqURL)
			return ip, nil
		}
	}

	for _, url := range urls {
		if url == lastReqURL {
			continue
		}
		if ip := httpGetIP(url, recordType, client); ip != "" {
			lastReqURL = url
			log.Printf("Successfully got IP using service: %s", url)
			return ip, nil
		}
		log.Printf("Service %s failed to get IP, trying next...", url)
		time.Sleep(1 * time.Second)
	}
	return "", fmt.Errorf("all IP services failed or IP doesn't match record type")
}

func httpGetIP(url, recordType string, client *http.Client) string {
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Cannot access %s, trying next...", url)
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	ip := string(bytes.TrimSpace(body))
	if isValidIPForType(ip, recordType) {
		return ip
	}
	return ""
}

func isValidIPForType(ip, recordType string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if recordType == "AAAA" {
		return parsed.To4() == nil
	}
	return parsed.To4() != nil
}

// getDNSRecord gets specified DNS record information from Cloudflare
func getDNSRecord(config *Config) (DNSRecord, error) {
	var record DNSRecord
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=%s&name=%s",
		config.ZoneId, config.RecordType, config.RecordName)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return record, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cf-ddns/1.0")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return record, err
	}
	defer resp.Body.Close()

	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return record, err
	}
	if !cfResp.Success {
		return record, fmt.Errorf("cloudflare API error: %v", cfResp.Errors)
	}
	if len(cfResp.Result) == 0 {
		return record, fmt.Errorf("DNS record not found: %s", config.RecordName)
	}
	return cfResp.Result[0], nil
}

// updateDNSRecord updates DNS record on Cloudflare
func updateDNSRecord(config *Config, existing DNSRecord, ip string) error {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", config.ZoneId, existing.Id)

	ttl := existing.TTL
	if config.TTL != nil {
		ttl = *config.TTL
	} else if ttl == 0 {
		ttl = 1
	}

	proxied := existing.Proxied
	if config.Proxied != nil {
		proxied = config.Proxied
	}

	payload := updatePayload{
		Type:    config.RecordType,
		Name:    config.RecordName,
		Content: ip,
		TTL:     ttl,
		Proxied: proxied,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cf-ddns/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cfResp CloudflareBaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return err
	}
	if !cfResp.Success {
		return fmt.Errorf("cloudflare API update failed: %v", cfResp.Errors)
	}
	return nil
}
