// Package kephtest ist ein Kephalaion im Speicher für Tests: eine Collection
// mit Inhalt, Revisionen und Stempeln hinter denselben Methoden wie
// kephalaion.Client (List, Read, Create, Write, Rename, Delete). Er prüft
// Namen und Revisionen wie der Hub. Nur Tests importieren das Paket.
package kephtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Stamp ist der Zeitpunkt, den Store für jeden Vorgang setzt.
const Stamp = "2026-10-20T08:00:00.000Z"

// Doc ist ein Dokument im Store.
type Doc struct {
	Content  string
	Revision int64
	Created  string
	Updated  string
}

// Store ist die Collection im Speicher. Fail lässt das Werkzeug mit dem
// Namen (list, read, create, write, rename, delete) mit dem Fehler scheitern.
type Store struct {
	mu       sync.Mutex
	Docs     map[string]*Doc
	Fail     map[string]error
	Calls    []string
	revision int64
}

// New baut einen leeren Store.
func New() *Store {
	return &Store{Docs: map[string]*Doc{}, Fail: map[string]error{}}
}

// Put legt ein Dokument ohne Aufruf an.
func (s *Store) Put(name, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	s.Docs[name] = &Doc{Content: content, Revision: s.revision, Created: Stamp, Updated: Stamp}
}

// Content ist der Inhalt eines Dokuments, ok sagt, ob es lebt.
func (s *Store) Content(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, ok := s.Docs[name]
	if !ok {
		return "", false
	}
	return doc.Content, true
}

// Names sind die Namen der lebenden Dokumente, sortiert.
func (s *Store) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.Docs))
	for name := range s.Docs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Store) record(tool, detail string) error {
	s.Calls = append(s.Calls, tool+" "+detail)
	return s.Fail[tool]
}

func toolError(tool, code, message string) error {
	return &kephalaion.ToolError{Tool: tool, Code: code, Message: message}
}

func (s *Store) List(_ context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.record("list", req.Path); err != nil {
		return nil, err
	}
	out := []kephalaion.Entry{}
	for name, doc := range s.Docs {
		rest, ok := strings.CutPrefix(name, req.Path+"/")
		if req.Path == "" {
			rest, ok = name, true
		}
		if !ok || (!req.Recursive && strings.Contains(rest, "/")) {
			continue
		}
		out = append(out, kephalaion.Entry{Kind: "document", Name: name, Revision: doc.Revision,
			Created: &kephalaion.Stamp{At: doc.Created, By: "kamran"}, Updated: &kephalaion.Stamp{At: doc.Updated, By: "kamran"}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) Read(_ context.Context, name string) (kephalaion.ReadResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.record("read", name); err != nil {
		return kephalaion.ReadResult{}, err
	}
	doc, ok := s.Docs[name]
	if !ok {
		return kephalaion.ReadResult{Kind: kephalaion.ReadKindNone, Name: name}, nil
	}
	content := doc.Content
	return kephalaion.ReadResult{Kind: kephalaion.ReadKindDocument, Name: name, Revision: doc.Revision, Content: &content,
		Created: &kephalaion.Stamp{At: doc.Created}, Updated: &kephalaion.Stamp{At: doc.Updated}}, nil
}

func (s *Store) Create(_ context.Context, name, content string) (kephalaion.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.record("create", name); err != nil {
		return kephalaion.Document{}, err
	}
	if _, ok := s.Docs[name]; ok {
		return kephalaion.Document{}, toolError("create", kephalaion.CodeNameTaken, "belegt")
	}
	s.revision++
	s.Docs[name] = &Doc{Content: content, Revision: s.revision, Created: Stamp, Updated: Stamp}
	return kephalaion.Document{Name: name, Revision: s.revision, Created: &kephalaion.Stamp{At: Stamp}}, nil
}

func (s *Store) Write(_ context.Context, name, content string, base int64) (kephalaion.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.record("write", fmt.Sprintf("%s @%d", name, base)); err != nil {
		return kephalaion.Document{}, err
	}
	doc, ok := s.Docs[name]
	if !ok {
		return kephalaion.Document{}, toolError("write", kephalaion.CodeNotFound, "fehlt")
	}
	if base > 0 && base != doc.Revision {
		return kephalaion.Document{}, toolError("write", kephalaion.CodeStaleRevision, "veraltet")
	}
	s.revision++
	doc.Content, doc.Revision = content, s.revision
	return kephalaion.Document{Name: name, Revision: doc.Revision}, nil
}

func (s *Store) Rename(_ context.Context, name, newName string) (kephalaion.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.record("rename", name+" "+newName); err != nil {
		return kephalaion.Document{}, err
	}
	doc, ok := s.Docs[name]
	if !ok {
		return kephalaion.Document{}, toolError("rename", kephalaion.CodeNotFound, "fehlt")
	}
	if _, ok := s.Docs[newName]; ok {
		return kephalaion.Document{}, toolError("rename", kephalaion.CodeNameTaken, "Ziel belegt")
	}
	delete(s.Docs, name)
	s.revision++
	doc.Revision = s.revision
	s.Docs[newName] = doc
	return kephalaion.Document{Name: newName, Revision: doc.Revision}, nil
}

func (s *Store) Delete(_ context.Context, name string, base int64) (kephalaion.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.record("delete", fmt.Sprintf("%s @%d", name, base)); err != nil {
		return kephalaion.Document{}, err
	}
	doc, ok := s.Docs[name]
	if !ok {
		return kephalaion.Document{}, toolError("delete", kephalaion.CodeNotFound, "fehlt")
	}
	if base > 0 && base != doc.Revision {
		return kephalaion.Document{}, toolError("delete", kephalaion.CodeStaleRevision, "veraltet")
	}
	delete(s.Docs, name)
	s.revision++
	return kephalaion.Document{Name: name, Revision: s.revision, Deleted: true}, nil
}
