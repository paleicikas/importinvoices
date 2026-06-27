package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"
)

type TemplateFile struct {
	Filename string
	Content  string
}

type TemplateMeta struct {
	ID          string
	Type        string
	Title       string
	Description string
	Country     string
	Website     string
	Active      bool
	IsSystem    bool
	Files       []TemplateFile
	Request     *APIRequest
}

type APIRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

// RenderTemplateFiles renders one or more template files into a single output or ZIP archive.
func RenderTemplateFiles(files []TemplateFile, payload Payload, w io.Writer) (contentType, filename string, err error) {
	rendered := make([]TemplateFile, 0, len(files))
	for _, f := range files {
		content, renderErr := RenderTemplate(f.Filename, f.Content, payload)
		if renderErr != nil {
			return "", "", renderErr
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		rendered = append(rendered, TemplateFile{Filename: f.Filename, Content: content})
	}
	if len(rendered) == 0 {
		return "text/plain", "export.txt", nil
	}
	if len(rendered) == 1 {
		return contentTypeForFilename(rendered[0].Filename), rendered[0].Filename, writeString(w, rendered[0].Content)
	}
	zw := zip.NewWriter(w)
	for _, f := range rendered {
		fw, createErr := zw.Create(f.Filename)
		if createErr != nil {
			_ = zw.Close()
			return "", "", createErr
		}
		if _, writeErr := fw.Write([]byte(f.Content)); writeErr != nil {
			_ = zw.Close()
			return "", "", writeErr
		}
	}
	if err := zw.Close(); err != nil {
		return "", "", err
	}
	return "application/zip", fmt.Sprintf("export_%s.zip", payload.ExportedAt.Format("20060102_150405")), nil
}

func contentTypeForFilename(name string) string {
	switch strings.ToLower(filepathExt(name)) {
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".csv":
		return "text/csv; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}

func filepathExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}

func writeString(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}

// ExecuteAPI sends export payload to a configured HTTP endpoint.
func ExecuteAPI(ctx context.Context, req APIRequest, payload Payload) (int, string, error) {
	if err := ValidateExternalURL(req.URL); err != nil {
		return 0, "", err
	}
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = http.MethodPost
	}

	body := req.Body
	if strings.TrimSpace(body) != "" {
		t, err := template.New("api-body").Funcs(templateFuncs()).Parse(body)
		if err != nil {
			return 0, "", fmt.Errorf("parse API body template: %w", err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, payload); err != nil {
			return 0, "", fmt.Errorf("render API body template: %w", err)
		}
		body = buf.String()
	} else {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, "", err
		}
		body = string(b)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, strings.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return resp.StatusCode, string(respBody), fmt.Errorf("API export failed with status %s", resp.Status)
	}
	return resp.StatusCode, string(respBody), nil
}

func ValidateExternalURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported API URL scheme: %s", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("API URL host is required")
	}
	if host == "localhost" || host == "metadata" || host == "metadata.google.internal" || host == "169.254.169.254" {
		return fmt.Errorf("internal API URLs are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("internal API URLs are not allowed")
		}
	}
	return nil
}

func ParseAPIRequest(content string) (APIRequest, error) {
	var req APIRequest
	if err := json.Unmarshal([]byte(content), &req); err == nil && req.URL != "" {
		return req, nil
	}
	req.URL = strings.TrimSpace(content)
	req.Method = http.MethodPost
	return req, nil
}
