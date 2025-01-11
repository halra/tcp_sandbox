package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"tcp_sandbox/domain"
	"time"
)

// -----------------------------------------------------------
// Handle Connection & Message Flow
// -----------------------------------------------------------

func handleConnection(conn net.Conn, t *domain.Tenant) {
	defer func() {
		removeConnection(t, conn)
		conn.Close()
	}()
	reader := bufio.NewReader(conn)
	var buffer []byte
	inMessage := false

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				log.Printf("Tenant %q client disconnected: %s\n", t.Name, conn.RemoteAddr())
			} else {
				logError(t, fmt.Errorf("read error from %s: %v", conn.RemoteAddr(), err))
			}
			return
		}

		atomic.AddUint64(&t.BytesReceived, 1)

		switch b {
		case t.StartByte:
			buffer = buffer[:0]
			inMessage = true
		case t.EndByte:
			if inMessage {
				inMessage = false
				message := string(buffer)
				log.Printf("Received from tenant %q: %s", t.Name, message)
				go handleCompleteMessage(t, message)

				response := append([]byte{t.StartByte}, append(buffer, t.EndByte)...)
				n, writeErr := conn.Write(response)
				if writeErr != nil {
					logError(t, fmt.Errorf("write error to %s: %v", conn.RemoteAddr(), writeErr))
				} else {
					atomic.AddUint64(&t.BytesSent, uint64(n))
				}
			}
		default:
			if inMessage {
				buffer = append(buffer, b)
			} else {
				log.Printf("Tenant %q: Unexpected byte %q", t.Name, b)
			}
		}
	}
}

// handleCompleteMessage is called for each received message.
func handleCompleteMessage(t *domain.Tenant, msg string) {

	var body []byte
	var contentType string

	switch strings.ToLower(t.MessageFormat) {
	case "json":
		// Construct a map and encode as JSON TODO
		bodyMap := map[string]string{
			"tenant":  t.Name,
			"message": msg,
		}
		jsonBody, err := json.Marshal(bodyMap)
		if err != nil {
			logError(t, fmt.Errorf("json marshal error: %w", err))
			return
		}
		body = jsonBody
		contentType = "application/json"

	case "xml":
		// Construct a simple struct and encode as XML TODO
		type XMLMessage struct {
			XMLName xml.Name `xml:"Message"`
			Tenant  string   `xml:"Tenant"`
			Content string   `xml:"Content"`
		}
		xm := XMLMessage{Tenant: t.Name, Content: msg}

		xmlBody, err := xml.Marshal(xm)
		if err != nil {
			logError(t, fmt.Errorf("xml marshal error: %w", err))
			return
		}
		body = xmlBody
		contentType = "application/xml"

	case "text":
		// Send raw text (plain) TODO
		textBody := msg
		body = []byte(textBody)
		contentType = "text/plain"

	default:
		// Fallback to JSON if format not recognized TODO should be plain/text ?
		bodyMap := map[string]string{
			"tenant":  t.Name,
			"message": msg,
		}
		jsonBody, err := json.Marshal(bodyMap)
		if err != nil {
			logError(t, fmt.Errorf("json marshal error: %w", err))
			return
		}
		body = jsonBody
		contentType = "application/json"
	}

	req, err := http.NewRequest("POST", t.Endpoint, bytes.NewReader(body))
	if err != nil {
		logError(t, fmt.Errorf("building request error: %w", err))
		return
	}

	// Simple token vs. OAuth
	if t.SimpleAuthToken != "" {
		req.Header.Set("X-Auth", t.SimpleAuthToken)
	} else {
		accessToken, err := getOrRefreshToken(t)
		if err != nil {
			logError(t, fmt.Errorf("unable to get token for tenant %q: %w", t.Name, err))
			return
		}
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", t.OAuthCredentials.TokenType, accessToken))
	}

	// Set content type according to the chosen format
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logError(t, fmt.Errorf("REST call error: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logError(t, fmt.Errorf("REST call responded with status %d", resp.StatusCode))
		return
	}
	log.Printf("[Tenant %q] REST call to %s succeeded. Status: %d",
		t.Name, t.Endpoint, resp.StatusCode)
}

// getOrRefreshToken checks if our token is still valid for at least 10 seconds. TODO handle failed or wrong token
func getOrRefreshToken(t *domain.Tenant) (string, error) {
	now := time.Now()
	if t.OAuthCredentials.TokenExpiry.After(now.Add(10 * time.Second)) {
		return t.OAuthCredentials.AccessToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", t.OAuthCredentials.ClientID)
	data.Set("client_secret", t.OAuthCredentials.ClientSecret)
	if len(t.OAuthCredentials.Scopes) > 0 {
		data.Set("scope", strings.Join(t.OAuthCredentials.Scopes, " "))
	}

	req, err := http.NewRequest("POST", t.OAuthCredentials.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error requesting new token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"` // in seconds
		TokenType   string `json:"token_type"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("invalid token response: %w", err)
	}

	t.OAuthCredentials.AccessToken = tokenResp.AccessToken
	if tokenResp.TokenType != "" {
		t.OAuthCredentials.TokenType = tokenResp.TokenType
	} else {
		t.OAuthCredentials.TokenType = "Bearer"
	}

	if tokenResp.ExpiresIn > 0 {
		t.OAuthCredentials.TokenExpiry = now.Add(time.Second * time.Duration(tokenResp.ExpiresIn))
	} else {
		t.OAuthCredentials.TokenExpiry = now.Add(1 * time.Hour)
	}

	log.Printf("[Tenant %q] New OAuth token (%s) expires at %v",
		t.Name, t.OAuthCredentials.TokenType, t.OAuthCredentials.TokenExpiry)
	return t.OAuthCredentials.AccessToken, nil
}
