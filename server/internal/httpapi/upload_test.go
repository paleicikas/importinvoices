package httpapi

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"testing"
	"strings"
)

func TestUploadHandlers(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	// 1. Upload page
	resp, err := client.Get(ts.URL + "/upload")
	if err != nil {
		t.Fatalf("GET /upload: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Upload") {
		t.Error("missing Upload title")
	}

	// 2. Successful upload (PNG)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("files", "test.png")
	// Use 1x1 PNG from testdata
	pngData := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDAT\x08\xd7c\xf8\xff\xff? \x00\x05\xfe\x02\xfe\xdcD\x05\xe8\x00\x00\x00\x00IEND\xaeB`\x82")
	part.Write(pngData)
	writer.WriteField(csrfFormField, csrfTokenFromJar(client, ts.URL))
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /upload: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
}

func TestUploadPage(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	resp, err := client.Get(ts.URL + "/upload")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestUpload_Errors(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	token := fetchCSRFCookie(t, client, ts.URL+"/upload")

	// Test empty upload
	resp := postForm(t, client, ts.URL+"/upload", url.Values{
		csrfFormField: {token},
	})
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}

	// Test invalid multipart
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/upload", strings.NewReader("invalid"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary=foo")
	req.Header.Set(csrfHeaderName, token)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
}
