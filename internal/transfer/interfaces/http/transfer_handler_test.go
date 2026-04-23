package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/felipersas/payflow/internal/transfer/application/services"
	"github.com/felipersas/payflow/internal/transfer/domain/entities"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type mockTransferRepo struct {
	transfers map[string]*entities.Transfer
	mu        sync.Mutex
}

func newMockTransferRepo() *mockTransferRepo {
	return &mockTransferRepo{
		transfers: make(map[string]*entities.Transfer),
	}
}

func (m *mockTransferRepo) Create(_ context.Context, transfer *entities.Transfer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transfers[transfer.ID] = transfer
	return nil
}

func (m *mockTransferRepo) GetByID(_ context.Context, id string) (*entities.Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.transfers[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (m *mockTransferRepo) GetByReference(_ context.Context, reference string) (*entities.Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.transfers {
		if t.ID == reference {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockTransferRepo) UpdateStatus(_ context.Context, id string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.transfers[id]; ok {
		t.Status = status
		t.UpdatedAt = time.Now().UTC()
	}
	return nil
}

type publishedMsg struct {
	routingKey string
	event      any
}

type mockPublisher struct {
	messages []publishedMsg
	mu       sync.Mutex
}

func (m *mockPublisher) Publish(_ context.Context, routingKey string, event any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, publishedMsg{
		routingKey: routingKey,
		event:      event,
	})
	return nil
}

func (m *mockPublisher) Close() error {
	return nil
}

func setupTransferHandler() (*TransferHandler, *mockTransferRepo, *mockPublisher) {
	repo := newMockTransferRepo()
	pub := &mockPublisher{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewTransferService(repo, pub, logger)
	return NewTransferHandler(svc), repo, pub
}

func TestCreateTransfer_Success(t *testing.T) {
	h, _, _ := setupTransferHandler()
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	body := map[string]any{
		"from_account_id": uuid.New().String(),
		"to_account_id":   uuid.New().String(),
		"amount":          100.50,
		"currency":        "BRL",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["transfer_id"] == nil {
		t.Error("response missing transfer_id")
	}
	if resp["status"] == nil {
		t.Error("response missing status")
	}
	if resp["status"] != "pending" {
		t.Errorf("status = %v, want pending", resp["status"])
	}
}

func TestCreateTransfer_InvalidBody(t *testing.T) {
	h, _, _ := setupTransferHandler()
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateTransfer_InvalidInput(t *testing.T) {
	h, _, _ := setupTransferHandler()
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	body := map[string]any{
		"from_account_id": "",
		"to_account_id":   uuid.New().String(),
		"amount":          100.50,
		"currency":        "BRL",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetTransfer_Success(t *testing.T) {
	h, repo, _ := setupTransferHandler()
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	// Create a transfer via service directly
	transfer, _ := entities.NewTransfer(uuid.New().String(), uuid.New().String(), 10050, "BRL")
	repo.Create(context.Background(), transfer)

	req := httptest.NewRequest("GET", "/"+transfer.ID, nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["transfer_id"] == nil {
		t.Error("response missing transfer_id")
	}
	if resp["transfer_id"] != transfer.ID {
		t.Errorf("transfer_id = %v, want %v", resp["transfer_id"], transfer.ID)
	}
}

func TestGetTransfer_NotFound(t *testing.T) {
	h, _, _ := setupTransferHandler()
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	req := httptest.NewRequest("GET", "/nonexistent-id", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
