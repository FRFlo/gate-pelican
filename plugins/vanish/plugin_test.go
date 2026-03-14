package vanish

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	guuid "github.com/google/uuid"
	"go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/util/permission"
)

type mockSource struct {
	perms    map[string]bool
	messages []string
}

func newMockSource(perms ...string) *mockSource {
	m := &mockSource{perms: make(map[string]bool)}
	for _, p := range perms {
		m.perms[p] = true
	}
	return m
}

func (m *mockSource) HasPermission(perm string) bool {
	return m.perms[perm]
}

func (m *mockSource) PermissionValue(perm string) permission.TriState {
	if m.HasPermission(perm) {
		return permission.True
	}
	return permission.False
}

func (m *mockSource) SendMessage(msg component.Component, opts ...command.MessageOption) error {
	if t, ok := msg.(*component.Text); ok {
		m.messages = append(m.messages, t.Content)
	}
	return nil
}

func (m *mockSource) lastMessage() string {
	if len(m.messages) == 0 {
		return ""
	}
	return m.messages[len(m.messages)-1]
}

func TestStoreToggleAndLookup(t *testing.T) {
	s := newStore()
	id := guuid.MustParse("550e8400-e29b-41d4-a716-446655440010")

	if s.IsVanished(id) {
		t.Fatal("expected player to be visible initially")
	}

	if !s.Toggle(id) {
		t.Fatal("expected toggle to vanish player")
	}

	if !s.IsVanished(id) {
		t.Fatal("expected player to be vanished after toggle")
	}

	if s.Toggle(id) {
		t.Fatal("expected second toggle to unvanish player")
	}

	if s.IsVanished(id) {
		t.Fatal("expected player to be visible after second toggle")
	}
}

func TestStoreRemove(t *testing.T) {
	s := newStore()
	id := guuid.MustParse("550e8400-e29b-41d4-a716-446655440011")
	s.Toggle(id)

	s.Remove(id)

	if s.IsVanished(id) {
		t.Fatal("expected remove to clear vanished state")
	}
}

func TestStoreSnapshot(t *testing.T) {
	s := newStore()
	first := guuid.MustParse("550e8400-e29b-41d4-a716-446655440012")
	second := guuid.MustParse("550e8400-e29b-41d4-a716-446655440013")
	s.Toggle(first)
	s.Toggle(second)

	snap := s.Snapshot()

	if len(snap) != 2 {
		t.Fatalf("expected 2 vanished players, got %d", len(snap))
	}
}

func TestListedForViewer(t *testing.T) {
	if !listedForViewer(false, false) {
		t.Fatal("visible target should be listed")
	}
	if listedForViewer(true, false) {
		t.Fatal("vanished target should be hidden from normal viewers")
	}
	if !listedForViewer(true, true) {
		t.Fatal("vanished target should be listed for staff viewers")
	}
}

func TestVanishCommandRequiresPermission(t *testing.T) {
	src := newMockSource()

	var mgr command.Manager
	mgr.Register(newVanishCommand(nil, newStore(), logr.Discard()))

	if err := mgr.Do(context.Background(), src, "vanish"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(src.lastMessage(), "do not have permission") {
		t.Fatalf("expected permission message, got %q", src.lastMessage())
	}
}

func TestCanManageOthers(t *testing.T) {
	if canManageOthers(nil) {
		t.Fatal("nil source cannot manage others")
	}
	if canManageOthers(newMockSource(permCommand)) {
		t.Fatalf("source missing %s should not manage others", permOthers)
	}
	if !canManageOthers(newMockSource(permCommand, permOthers)) {
		t.Fatal("source with both permissions should manage others")
	}
}
