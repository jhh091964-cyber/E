package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"net/http"
	"net/url"
	"time"
)

// Provider implements Cloudflare DNS provider
type Provider struct {
	apiToken string
	dryRun   bool
	client   *http.Client
}

// NewProvider creates a new Cloudflare DNS provider
func NewProvider(apiToken string, dryRun bool) *Provider {
	return &Provider{
		apiToken: apiToken,
		dryRun:   dryRun,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Zone represents a Cloudflare zone
type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// DNSRecord represents a DNS record
type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
	Priority int   `json:"priority,omitempty"`
}

// APIResponse represents Cloudflare API response
type APIResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Messages []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"messages"`
	Result interface{} `json:"result"`
}

// DNSListResponse represents list records response
type DNSListResponse struct {
	Success bool        `json:"success"`
	Result  []DNSRecord `json:"result"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// GetZoneID gets zone ID by zone name
func (p *Provider) GetZoneID(zoneName string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", zoneName)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	var apiResp struct {
		Success bool   `json:"success"`
		Result  []Zone `json:"result"`
	}
	
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", err
	}
	
	if !apiResp.Success || len(apiResp.Result) == 0 {
		return "", fmt.Errorf("zone not found: %s", zoneName)
	}
	
	return apiResp.Result[0].ID, nil
}

// FindRecord finds a DNS record by type and name
func (p *Provider) FindRecord(recordType, name string) (*DNSRecord, error) {
	if p.dryRun {
		return nil, nil // In dry-run mode, pretend record doesn't exist
	}
	
	zoneName := extractZoneName(name)
	zoneID, err := p.GetZoneID(zoneName)
	if err != nil {
		return nil, err
	}
	
	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("type", recordType)
	queryParams.Set("name", name)
	queryParams.Set("per_page", "100")
	
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?%s", zoneID, queryParams.Encode())
	
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var listResp DNSListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, err
	}
	
	if !listResp.Success {
		if len(listResp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s", listResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("failed to list records")
	}
	
	// Return first matching record
	if len(listResp.Result) > 0 {
		return &listResp.Result[0], nil
	}
	
	return nil, nil // Not found
}

// UpsertRecord creates or updates a DNS record
func (p *Provider) UpsertRecord(recordType, name, content string, priority *int) error {
	if p.dryRun {
		// In dry-run mode, just return without doing anything
		// The caller will log what would happen
		return nil
	}
	
	// Try to find existing record
	existing, err := p.FindRecord(recordType, name)
	if err != nil {
		return fmt.Errorf("failed to check existing record: %w", err)
	}
	
	zoneName := extractZoneName(name)
	zoneID, err := p.GetZoneID(zoneName)
	if err != nil {
		return err
	}
	
	record := DNSRecord{
		Type:    recordType,
		Name:    name,
		Content: content,
		TTL:     3600,
		Proxied: false,
	}
	
	if priority != nil {
		record.Priority = *priority
	}
	
	if existing != nil {
		// Update existing record
		return p.updateRecord(zoneID, existing.ID, record)
	} else {
		// Create new record
		return p.createRecord(zoneID, record)
	}
}

// CreateARecord creates an A record
func (p *Provider) CreateARecord(zone, name, ip string) error {
	if p.dryRun {
		// In dry-run mode, just return without doing anything
		return nil
	}
	
	zoneID, err := p.GetZoneID(zone)
	if err != nil {
		return err
	}
	
	fullName := fmt.Sprintf("%s.%s", name, zone)
	
	record := DNSRecord{
		Type:    "A",
		Name:    fullName,
		Content: ip,
		TTL:     3600,
		Proxied: false,
	}
	
	return p.createRecord(zoneID, record)
}

// CreateMXRecord creates an MX record
func (p *Provider) CreateMXRecord(zone, mailServer string, priority int) error {
	if p.dryRun {
		// In dry-run mode, just return without doing anything
		return nil
	}
	
	zoneID, err := p.GetZoneID(zone)
	if err != nil {
		return err
	}
	
	record := DNSRecord{
		Type:     "MX",
		Name:     zone,
		Content:  fmt.Sprintf("%s.%s", mailServer, zone),
		TTL:      3600,
		Priority: priority,
	}
	
	return p.createRecord(zoneID, record)
}

// CreateTXTRecord creates a TXT record
func (p *Provider) CreateTXTRecord(zone, name, content string) error {
	if p.dryRun {
		// In dry-run mode, just return without doing anything
		return nil
	}
	
	zoneID, err := p.GetZoneID(zone)
	if err != nil {
		return err
	}
	
	var fullName string
	if name == "@" {
		fullName = zone
	} else {
		fullName = fmt.Sprintf("%s.%s", name, zone)
	}
	
	record := DNSRecord{
		Type:    "TXT",
		Name:    fullName,
		Content: content,
		TTL:     3600,
	}
	
	return p.createRecord(zoneID, record)
}

// createRecord creates a DNS record
func (p *Provider) createRecord(zoneID string, record DNSRecord) error {
	if p.dryRun {
		// In dry-run mode, just return without doing anything
		return nil
	}
	
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return err
	}
	
	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			return fmt.Errorf("API error: %s", apiResp.Errors[0].Message)
		}
		return fmt.Errorf("failed to create record")
	}
	
	return nil
}

// updateRecord updates an existing DNS record
func (p *Provider) updateRecord(zoneID, recordID string, record DNSRecord) error {
	if p.dryRun {
		// In dry-run mode, just return without doing anything
		return nil
	}
	
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)
	
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return err
	}
	
	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			return fmt.Errorf("API error: %s", apiResp.Errors[0].Message)
		}
		return fmt.Errorf("failed to update record")
	}
	
	return nil
}

// ApplyDNSRecords applies all DNS records for a mail server
func (p *Provider) ApplyDNSRecords(zone, host, domain, ip, dkimPublicKey, spfTemplate, dmarcTemplate, dkimSelector string) error {
	// A record
	if err := p.CreateARecord(zone, host, ip); err != nil {
		return fmt.Errorf("failed to create A record: %v", err)
	}
	
	// MX record
	if err := p.CreateMXRecord(zone, host, 10); err != nil {
		return fmt.Errorf("failed to create MX record: %v", err)
	}
	
	// SPF record
	spfRecord := spfTemplate
	if spfRecord == "" {
		spfRecord = fmt.Sprintf("v=spf1 mx -all")
	}
	if err := p.CreateTXTRecord(zone, "@", spfRecord); err != nil {
		return fmt.Errorf("failed to create SPF record: %v", err)
	}
	
	// DMARC record
	dmarcRecord := dmarcTemplate
	if dmarcRecord == "" {
		dmarcRecord = fmt.Sprintf("v=DMARC1; p=none; rua=mailto:dmarc@%s", zone)
	}
	if err := p.CreateTXTRecord(zone, "_dmarc", dmarcRecord); err != nil {
		return fmt.Errorf("failed to create DMARC record: %v", err)
	}
	
	// DKIM record
	if dkimPublicKey != "" {
		dkimRecordName := fmt.Sprintf("%s._domainkey", dkimSelector)
		if err := p.CreateTXTRecord(zone, dkimRecordName, dkimPublicKey); err != nil {
			return fmt.Errorf("failed to create DKIM record: %v", err)
		}
	}
	
	return nil
}

// extractZoneName extracts zone name from full domain name
func extractZoneName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return fullName
}