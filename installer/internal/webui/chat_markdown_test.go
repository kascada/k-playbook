package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Die Seite setzt das HTML ohne eigenen Parser ein. Deshalb muss hier gelten:
// Markdown wird zu HTML, rohes HTML und javascript:-Verweise kommen nicht durch.
func TestChatMarkdownRendertSicher(t *testing.T) {
	texts := []string{
		"- **atlassian** – Jira & Confluence\n- **k-playbook** – Projekt-Kontext",
		"<script>alert(1)</script>\n\nText",
		"[klick](javascript:alert(1))",
	}
	payload, _ := json.Marshal(chatMarkdownRequest{Texts: texts})
	recorder := serveChat(t, http.MethodPost, "/api/chat/markdown", string(payload))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
	}
	var response chatMarkdownResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if len(response.HTML) != len(texts) {
		t.Fatalf("%d Ergebnisse, erwartet %d", len(response.HTML), len(texts))
	}
	if !strings.Contains(response.HTML[0], "<ul>") || !strings.Contains(response.HTML[0], "<strong>atlassian</strong>") {
		t.Errorf("Liste mit Fettschrift nicht gerendert: %s", response.HTML[0])
	}
	if strings.Contains(response.HTML[1], "<script") {
		t.Errorf("rohes HTML durchgereicht: %s", response.HTML[1])
	}
	if strings.Contains(response.HTML[2], "javascript:") {
		t.Errorf("javascript:-Verweis durchgereicht: %s", response.HTML[2])
	}
}

func TestChatMarkdownBegrenztAnzahl(t *testing.T) {
	texts := make([]string, chatMarkdownLimit+1)
	for i := range texts {
		texts[i] = fmt.Sprintf("Text %d", i)
	}
	payload, _ := json.Marshal(chatMarkdownRequest{Texts: texts})
	if recorder := serveChat(t, http.MethodPost, "/api/chat/markdown", string(payload)); recorder.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, erwartet 400", recorder.Code)
	}
}
