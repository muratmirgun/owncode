package permission

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/pubsub"
)

var ErrorPermissionDenied = errors.New("permission denied")

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type Service interface {
	pubsub.Suscriber[PermissionRequest]
	GrantPersistant(permission PermissionRequest)
	Grant(permission PermissionRequest)
	Deny(permission PermissionRequest)
	Request(opts CreatePermissionRequest) bool
	AutoApproveSession(sessionID string)
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]
	mu      sync.Mutex
	gate    chan struct{}
	yolo    bool
	changed chan struct{}

	sessionPermissions  []PermissionRequest
	pendingRequests     sync.Map
	autoApproveSessions []string
}

func (s *permissionService) GrantPersistant(p PermissionRequest) {
	s.mu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, p)
	s.mu.Unlock()
	s.respond(p.ID, true)
}
func (s *permissionService) respond(id string, allowed bool) {
	if ch, ok := s.pendingRequests.Load(id); ok {
		select {
		case ch.(chan bool) <- allowed:
		default:
		}
	}
}
func (s *permissionService) Grant(p PermissionRequest) { s.respond(p.ID, true) }
func (s *permissionService) Deny(p PermissionRequest)  { s.respond(p.ID, false) }
func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	return s.RequestContext(context.Background(), opts)
}

// Request preserves compatibility with permission services while forwarding cancellation.
func Request(ctx context.Context, service Service, opts CreatePermissionRequest) bool {
	if contextual, ok := service.(interface {
		RequestContext(context.Context, CreatePermissionRequest) bool
	}); ok {
		return contextual.RequestContext(ctx, opts)
	}
	if ctx.Err() != nil {
		return false
	}
	return service.Request(opts)
}

// RequestContext serializes prompts and releases pending requests on cancellation.
func (s *permissionService) RequestContext(ctx context.Context, opts CreatePermissionRequest) bool {
	if ctx.Err() != nil {
		return false
	}
	dir := filepath.Dir(opts.Path)
	if dir == "." {
		dir = config.WorkingDirectory()
	}
	p := PermissionRequest{ID: uuid.New().String(), Path: dir, SessionID: opts.SessionID, ToolName: opts.ToolName, Description: opts.Description, Action: opts.Action, Params: opts.Params}
	// Only interactive prompts share the gate. Already approved workers must
	// not wait behind another worker's unanswered prompt.
	for {
		allowed, changed := s.approval(p)
		if allowed {
			return ctx.Err() == nil
		}
		select {
		case s.gate <- struct{}{}:
			defer func() { <-s.gate }()
			goto acquired
		case <-changed:
		case <-ctx.Done():
			return false
		}
	}
acquired:
	if ctx.Err() != nil {
		return false
	}
	allowed, changed := s.approval(p)
	if allowed {
		return true
	}
	response := make(chan bool, 1)
	s.pendingRequests.Store(p.ID, response)
	defer s.pendingRequests.Delete(p.ID)
	s.Publish(pubsub.CreatedEvent, p)
	defer s.Publish(pubsub.DeletedEvent, p)
	for {
		select {
		case allowed := <-response:
			return allowed && ctx.Err() == nil
		case <-changed:
			allowed, changed = s.approval(p)
			if allowed {
				return ctx.Err() == nil
			}
		case <-ctx.Done():
			return false
		}
	}
}

func (s *permissionService) approval(p PermissionRequest) (bool, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	allowed := s.yolo || slices.Contains(s.autoApproveSessions, p.SessionID)
	for _, prior := range s.sessionPermissions {
		if prior.ToolName == p.ToolName && prior.Action == p.Action && prior.SessionID == p.SessionID && prior.Path == p.Path {
			allowed = true
			break
		}
	}
	return allowed, s.changed
}

// SetYOLO enables automatic permission approval for this service's lifetime.
// Disabling it preserves separate approvals explicitly granted by the user.
func SetYOLO(service Service, enabled bool) bool {
	s, ok := service.(*permissionService)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.yolo != enabled {
		s.yolo = enabled
		close(s.changed)
		s.changed = make(chan struct{})
	}
	return true
}

// YOLOEnabled reports whether this app run bypasses tool permission prompts.
func YOLOEnabled(service Service) bool {
	s, ok := service.(*permissionService)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.yolo
}

func (s *permissionService) AutoApproveSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoApproveSessions = append(s.autoApproveSessions, id)
}

func NewPermissionService() Service {
	return &permissionService{
		Broker:             pubsub.NewBroker[PermissionRequest](),
		gate:               make(chan struct{}, 1),
		changed:            make(chan struct{}),
		sessionPermissions: make([]PermissionRequest, 0),
	}
}
