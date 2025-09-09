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
	"syscall"
	"time"
)

// Config struct for mapping config.json file
type Config struct {
	APIToken             string `json:"api_token"`
	ZoneId               string `json:"zone_Id"`
	RecordName           string `json:"record_name"`
	RecordType           string `json:"record_type"`
	CheckIntervalSeconds uint   `json:"check_interval_seconds"`
	TTL                  *int   `json:"ttl,omitempty"`
	Proxied              *bool  `json:"proxied,omitempty"`
}

// CloudflareResponse struct for Cloudflare API response
type CloudflareResponse struct {
	Success bool        `json:"success"`
	Errors  []any       `json:"errors"`
	Result  []DNSRecord `json:"result"`
}

// DNSRecord struct for DNS record
type DNSRecord struct {
	Id      string `json:"Id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied *bool  `json:"proxied,omitempty"`
}

var LastReqURL string
var LastIP string

func main() {
	var configPath string
	var runOnce bool
	flag.StringVar(&configPath, "config", "config.json", "config file path")
	flag.BoolVar(&runOnce, "once", false, "run once and exit")
	flag.Parse()

	// 1. Load configuration
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration file: %v", err)
	}

	if config.CheckIntervalSeconds == 0 {
		config.CheckIntervalSeconds = 300 // Default 5 minutes
		log.Printf("Warning: Check interval not set, using default value %d seconds", config.CheckIntervalSeconds)
	}

	log.Println("DDNS client started...")
	log.Printf("Monitoring domain: %s", config.RecordName)
	log.Printf("Check interval: %d seconds", config.CheckIntervalSeconds)

	// Signal handling for graceful exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 2. Set up timer for periodic checks
	ticker := time.NewTicker(time.Duration(config.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()

	// Execute immediately once
	runUpdate(&config)
	times := 1
	if runOnce {
		log.Print("Single run completed, program exiting.")
		return
	}

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

	// 1. Get current public IP
	publicIP, err := getPublicIP(config.RecordType)
	if err != nil {
		log.Printf("Error: Failed to get public IP: %v", err)
		return
	}
	log.Printf("Current public IP: %s", publicIP)

	// If same as last time, skip Cloudflare API call to reduce requests
	if LastIP != "" && publicIP == LastIP {
		log.Println("IP is the same as last time, skipping Cloudflare check.")
		log.Println("---------------------------------")
		return
	}

	// 2. Get DNS record from Cloudflare
	record, err := getDNSRecord(*config)
	if err != nil {
		log.Printf("Error: Failed to get Cloudflare DNS record: %v", err)
		return
	}
	log.Printf("Cloudflare record IP: %s", record.Content)

	// 3. Compare IP addresses and decide whether to update
	if publicIP == record.Content {
		log.Println("IP address unchanged, no update needed.")
		LastIP = publicIP
	} else {
		log.Printf("IP address changed from %s to %s. Updating...", record.Content, publicIP)
		err := updateDNSRecord(*config, record, publicIP)
		if err != nil {
			log.Printf("Error: Failed to update DNS record: %v", err)
		} else {
			log.Println("DNS record updated successfully!")
			LastIP = publicIP
		}
	}
	log.Println("---------------------------------")
}

// loadConfig loads configuration from JSON file
func loadConfig(file string) (Config, error) {
	var config Config
	configFile, err := os.Open(file)
	if err != nil {
		return config, err
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	return config, err
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
	default: // "A" or others fallback to IPv4
		urls = []string{
			"https://ipv4.icanhazip.com",
			"https://ifconfig.co/ip?v=4",
			"https://api.ipify.org",
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	ip := httpGetIP(LastReqURL, recordType, client)
	if ip != "" {
		log.Printf("Successfully got IP using last service: %s", LastReqURL)
		return ip, nil
	}

	// Last service unavailable or not set, try services in the list
	for _, url := range urls {
		if url == LastReqURL {
			continue // Already tried
		}
		ip = httpGetIP(url, recordType, client)
		if ip != "" {
			LastReqURL = url
			log.Printf("Successfully got IP using service: %s", url)
			return ip, nil
		}
		log.Printf("Service %s failed to get IP, trying next...", url)
		time.Sleep(1 * time.Second) // Small delay to avoid rapid requests
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
	if valIdIPForType(ip, recordType) {
		return ip
	}
	return ""
}

func valIdIPForType(ip, recordType string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if recordType == "AAAA" {
		return parsed.To4() == nil // IPv6
	}
	return parsed.To4() != nil // IPv4
}

// getDNSRecord gets specified DNS record information from Cloudflare
func getDNSRecord(config Config) (DNSRecord, error) {
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
		return record, fmt.Errorf("Cloudflare API error: %+v", cfResp.Errors)
	}

	if len(cfResp.Result) == 0 {
		return record, fmt.Errorf("DNS record not found: %s", config.RecordName)
	}

	return cfResp.Result[0], nil
}

// updateDNSRecord updates DNS record on Cloudflare
func updateDNSRecord(config Config, existing DNSRecord, ip string) error {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", config.ZoneId, existing.Id)

	// Create request body
	payload := map[string]any{
		"type":    config.RecordType,
		"name":    config.RecordName,
		"content": ip,
	}
	// ttl: prefer ttl from config file; otherwise use existing record; default 1 (auto)
	ttl := existing.TTL
	if config.TTL != nil {
		ttl = *config.TTL
	} else if ttl == 0 {
		ttl = 1
	}
	payload["ttl"] = ttl
	// proxied: prefer proxied from config file; otherwise use existing record (if exists)
	if config.Proxied != nil {
		payload["proxied"] = *config.Proxied
	} else if existing.Proxied != nil {
		payload["proxied"] = *existing.Proxied
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

	var cfResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return err
	}

	// Check if response is successful
	if success, ok := cfResp["success"].(bool); !ok || !success {
		return fmt.Errorf("Cloudflare API update failed: %+v", cfResp["errors"])
	}

	return nil
}
