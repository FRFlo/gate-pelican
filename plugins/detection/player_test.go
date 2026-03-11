package detection

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// TestPlayerConcurrency — race-detector target
// ---------------------------------------------------------------------------

// TestPlayerConcurrencyGenericChecks hammers AddGenericCheck / HasGenericCheck
// from many goroutines simultaneously. The -race flag must produce no data races.
func TestPlayerConcurrencyGenericChecks(t *testing.T) {
	p := newDetectedPlayer(uuid.New())
	const goroutines = 50
	const ops = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		id := fmt.Sprintf("check-%d", i)
		go func(checkID string) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				p.AddGenericCheck(checkID)
			}
		}(id)
		go func(checkID string) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				p.HasGenericCheck(checkID)
			}
		}(id)
	}

	wg.Wait()
	// All writes are complete; verify at least one check was recorded.
	checks := p.GenericChecks()
	if len(checks) == 0 {
		t.Fatal("expected at least one generic check to be recorded")
	}
}

// TestPlayerConcurrencyLunarMods hammers SetLunarMods / LunarMods / HasLunarMod
// from many goroutines simultaneously.
func TestPlayerConcurrencyLunarMods(t *testing.T) {
	p := newDetectedPlayer(uuid.New())
	const goroutines = 30

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	mods := []LunarModInfo{
		{ID: "optifine", DisplayName: "OptiFine", Version: "1.0", Type: "FORGE"},
		{ID: "iris", DisplayName: "Iris", Version: "2.0", Type: "FABRIC"},
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			p.SetLunarMods(mods)
		}()
		go func() {
			defer wg.Done()
			p.LunarMods()
			p.HasLunarMod("optifine")
			p.HasLunarModsData()
		}()
	}

	wg.Wait()
}

// TestPlayerConcurrencyForgeMods hammers AddForgeMods / ForgeMods / HasForgeMod.
func TestPlayerConcurrencyForgeMods(t *testing.T) {
	p := newDetectedPlayer(uuid.New())
	const goroutines = 30

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	mods := []ForgeModInfo{
		{ModID: "forgewurst", Version: "1.0"},
		{ModID: "forgepatch", Version: "2.0"},
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			p.AddForgeMods(mods)
		}()
		go func() {
			defer wg.Done()
			p.ForgeMods()
			p.HasForgeMod("forgewurst")
			p.HasForgeModsData()
		}()
	}

	wg.Wait()
}

// TestPlayerConcurrencyPendingActions hammers QueuePendingAction / ExecutePendingActions
// concurrently to ensure no races on the action slice.
func TestPlayerConcurrencyPendingActions(t *testing.T) {
	p := newDetectedPlayer(uuid.New())
	const goroutines = 40

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			p.QueuePendingAction(func() {})
		}()
	}

	wg.Wait() // all actions queued

	if !p.HasPendingActions() {
		t.Fatal("expected pending actions to be queued")
	}

	// A single drain should execute all queued actions and leave none behind.
	p.ExecutePendingActions()

	if p.HasPendingActions() {
		t.Fatal("all pending actions should have been drained by ExecutePendingActions")
	}
}

// TestPlayerConcurrencyBedrockAndForgeType exercises the scalar booleans and
// pointer-based forge client type under concurrent access.
func TestPlayerConcurrencyBedrockAndForgeType(t *testing.T) {
	p := newDetectedPlayer(uuid.New())
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			p.SetBedrockDetected(true)
		}()
		go func() {
			defer wg.Done()
			p.IsBedrockDetected()
		}()
		go func() {
			defer wg.Done()
			p.SetForgeClientType(ForgeClientForge)
		}()
		go func() {
			defer wg.Done()
			p.ForgeClientType()
		}()
	}

	wg.Wait()
}

// TestPlayerConcurrency is the umbrella test called by the plan QA scenario.
// It delegates to the specific sub-tests above and is designed to be run with
// `-race` so the detector can catch any missed lock.
func TestPlayerConcurrency(t *testing.T) {
	t.Run("GenericChecks", TestPlayerConcurrencyGenericChecks)
	t.Run("LunarMods", TestPlayerConcurrencyLunarMods)
	t.Run("ForgeMods", TestPlayerConcurrencyForgeMods)
	t.Run("PendingActions", TestPlayerConcurrencyPendingActions)
	t.Run("BedrockAndForgeType", TestPlayerConcurrencyBedrockAndForgeType)
}

// ---------------------------------------------------------------------------
// TestPlayerStore — lifecycle and semantic tests
// ---------------------------------------------------------------------------

func TestPlayerStore(t *testing.T) {
	t.Run("RegisterAndGet", testPlayerStoreRegisterAndGet)
	t.Run("GetCreatesIfAbsent", testPlayerStoreGetCreatesIfAbsent)
	t.Run("RegisterPreservesExisting", testPlayerStoreRegisterPreservesExisting)
	t.Run("Remove", testPlayerStoreRemove)
	t.Run("Cleanup", testPlayerStoreCleanup)
	t.Run("Players", testPlayerStorePlayers)
	t.Run("Len", testPlayerStoreLen)
	t.Run("ConcurrentGetOrCreate", testPlayerStoreConcurrentGetOrCreate)
}

func testPlayerStoreRegisterAndGet(t *testing.T) {
	s := NewPlayerStore()
	id := uuid.New()

	s.Register(id)
	p := s.Get(id)

	if p == nil {
		t.Fatal("expected non-nil player after Register+Get")
	}
	if p.UUID != id {
		t.Fatalf("UUID mismatch: got %v, want %v", p.UUID, id)
	}
}

// Get must create the player on demand (mirrors HackedServer.getPlayer computeIfAbsent).
func testPlayerStoreGetCreatesIfAbsent(t *testing.T) {
	s := NewPlayerStore()
	id := uuid.New()

	if s.Len() != 0 {
		t.Fatal("expected empty store")
	}

	p := s.Get(id)
	if p == nil {
		t.Fatal("Get must create and return a player")
	}
	if s.Len() != 1 {
		t.Fatalf("expected Len==1 after implicit create, got %d", s.Len())
	}
}

// Register after Get must not overwrite the existing entry (pending actions must survive).
func testPlayerStoreRegisterPreservesExisting(t *testing.T) {
	s := NewPlayerStore()
	id := uuid.New()

	// Simulate a handler calling Get before the login event fires Register.
	early := s.Get(id)

	called := false
	early.QueuePendingAction(func() { called = true })

	// Now the login event fires Register.
	s.Register(id)

	// The player in the store must still be the same pointer (no overwrite).
	stored := s.Get(id)
	if stored != early {
		t.Fatal("Register must not replace an existing player created by Get")
	}

	// Pending actions accumulated before Register must survive.
	stored.ExecutePendingActions()
	if !called {
		t.Fatal("pending actions were lost after Register over Get-created player")
	}
}

func testPlayerStoreRemove(t *testing.T) {
	s := NewPlayerStore()
	id := uuid.New()

	s.Register(id)
	if s.Len() != 1 {
		t.Fatal("expected Len==1 before Remove")
	}

	s.Remove(id)
	if s.Len() != 0 {
		t.Fatal("expected Len==0 after Remove")
	}

	// Get after Remove must create a fresh player.
	fresh := s.Get(id)
	if fresh == nil {
		t.Fatal("expected new player after Remove+Get")
	}
	if fresh.HasPendingActions() {
		t.Fatal("freshly created player must not have pending actions")
	}
}

func testPlayerStoreCleanup(t *testing.T) {
	s := NewPlayerStore()

	for i := 0; i < 5; i++ {
		s.Register(uuid.New())
	}
	if s.Len() != 5 {
		t.Fatalf("expected 5 players before Cleanup, got %d", s.Len())
	}

	s.Cleanup()
	if s.Len() != 0 {
		t.Fatalf("expected 0 players after Cleanup, got %d", s.Len())
	}
}

func testPlayerStorePlayers(t *testing.T) {
	s := NewPlayerStore()
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for _, id := range ids {
		s.Register(id)
	}

	players := s.Players()
	if len(players) != 3 {
		t.Fatalf("expected 3 players, got %d", len(players))
	}

	// Verify the returned slice is a snapshot (not aliased into internal map).
	s.Cleanup()
	if len(players) != 3 {
		t.Fatal("Players() snapshot must not be affected by subsequent Cleanup")
	}
}

func testPlayerStoreLen(t *testing.T) {
	s := NewPlayerStore()
	if s.Len() != 0 {
		t.Fatal("expected Len==0 on empty store")
	}

	id := uuid.New()
	s.Register(id)
	if s.Len() != 1 {
		t.Fatal("expected Len==1 after Register")
	}

	s.Remove(id)
	if s.Len() != 0 {
		t.Fatal("expected Len==0 after Remove")
	}
}

// Concurrent Get for the same UUID must return the same player pointer from all goroutines.
func testPlayerStoreConcurrentGetOrCreate(t *testing.T) {
	s := NewPlayerStore()
	id := uuid.New()
	const goroutines = 100

	results := make([]*DetectedPlayer, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i] = s.Get(id)
		}()
	}
	wg.Wait()

	// All goroutines must have received the same player pointer.
	first := results[0]
	for i, p := range results {
		if p != first {
			t.Fatalf("goroutine %d received a different player pointer", i)
		}
	}

	// Exactly one entry must exist in the store.
	if s.Len() != 1 {
		t.Fatalf("expected Len==1 after concurrent Gets, got %d", s.Len())
	}
}

// ---------------------------------------------------------------------------
// Unit tests for DetectedPlayer field semantics
// ---------------------------------------------------------------------------

func TestDetectedPlayerGenericChecks(t *testing.T) {
	p := newDetectedPlayer(uuid.New())

	if p.HasGenericCheck("x") {
		t.Fatal("fresh player must not have any checks")
	}

	p.AddGenericCheck("alpha")
	p.AddGenericCheck("beta")
	p.AddGenericCheck("alpha") // duplicate — must not error or duplicate

	if !p.HasGenericCheck("alpha") {
		t.Fatal("alpha check not found")
	}
	if !p.HasGenericCheck("beta") {
		t.Fatal("beta check not found")
	}
	if len(p.GenericChecks()) != 2 {
		t.Fatalf("expected 2 distinct checks, got %d", len(p.GenericChecks()))
	}
}

func TestDetectedPlayerLunarMods(t *testing.T) {
	p := newDetectedPlayer(uuid.New())

	if p.HasLunarModsData() {
		t.Fatal("fresh player must not have lunar data")
	}
	if p.HasLunarMod("optifine") {
		t.Fatal("fresh player must not have any lunar mods")
	}

	p.SetLunarMods([]LunarModInfo{
		{ID: "OptiFine", DisplayName: "OptiFine", Version: "1.0", Type: "FORGE"},
	})

	if !p.HasLunarModsData() {
		t.Fatal("lunar data must be marked known after SetLunarMods")
	}
	// IDs are normalized to lowercase.
	if !p.HasLunarMod("optifine") {
		t.Fatal("optifine not found (lowercase)")
	}
	if !p.HasLunarMod("OPTIFINE") {
		t.Fatal("optifine not found (uppercase)")
	}
	if len(p.LunarMods()) != 1 {
		t.Fatalf("expected 1 lunar mod, got %d", len(p.LunarMods()))
	}

	// SetLunarMods replaces the list.
	p.SetLunarMods([]LunarModInfo{
		{ID: "iris", DisplayName: "Iris", Version: "2.0", Type: "FABRIC"},
	})
	if p.HasLunarMod("optifine") {
		t.Fatal("optifine should have been replaced")
	}
	if !p.HasLunarMod("iris") {
		t.Fatal("iris should now be present")
	}
}

func TestDetectedPlayerForgeMods(t *testing.T) {
	p := newDetectedPlayer(uuid.New())

	if p.HasForgeModsData() {
		t.Fatal("fresh player must not have forge data")
	}

	p.AddForgeMods([]ForgeModInfo{
		{ModID: "ForgeWurst", Version: "1.0"},
	})

	if !p.HasForgeModsData() {
		t.Fatal("forge data must be marked known after AddForgeMods")
	}
	if !p.HasForgeMod("forgewurst") {
		t.Fatal("forgewurst not found (lowercase)")
	}
	if !p.HasForgeMod("FORGEWURST") {
		t.Fatal("forgewurst not found (uppercase)")
	}

	// AddForgeMods is additive, unlike SetLunarMods which replaces.
	p.AddForgeMods([]ForgeModInfo{
		{ModID: "forgepatch", Version: "2.0"},
	})
	if !p.HasForgeMod("forgewurst") {
		t.Fatal("forgewurst should still be present after second AddForgeMods")
	}
	if !p.HasForgeMod("forgepatch") {
		t.Fatal("forgepatch should be present after second AddForgeMods")
	}
}

func TestDetectedPlayerForgeClientType(t *testing.T) {
	p := newDetectedPlayer(uuid.New())

	_, ok := p.ForgeClientType()
	if ok {
		t.Fatal("fresh player must not have forge client type")
	}

	p.SetForgeClientType(ForgeClientForge)
	ct, ok := p.ForgeClientType()
	if !ok {
		t.Fatal("forge client type must be set")
	}
	if ct != ForgeClientForge {
		t.Fatalf("expected ForgeClientForge, got %v", ct)
	}

	p.SetForgeClientType(ForgeClientNeoForge)
	ct, ok = p.ForgeClientType()
	if !ok || ct != ForgeClientNeoForge {
		t.Fatalf("expected ForgeClientNeoForge, got %v ok=%v", ct, ok)
	}
}

func TestDetectedPlayerBedrock(t *testing.T) {
	p := newDetectedPlayer(uuid.New())

	if p.IsBedrockDetected() {
		t.Fatal("fresh player must not be bedrock-detected")
	}

	p.SetBedrockDetected(true)
	if !p.IsBedrockDetected() {
		t.Fatal("bedrock must be detected after SetBedrockDetected(true)")
	}

	p.SetBedrockDetected(false)
	if p.IsBedrockDetected() {
		t.Fatal("bedrock must not be detected after SetBedrockDetected(false)")
	}
}

func TestDetectedPlayerPendingActions(t *testing.T) {
	p := newDetectedPlayer(uuid.New())

	if p.HasPendingActions() {
		t.Fatal("fresh player must have no pending actions")
	}

	var order []int
	p.QueuePendingAction(func() { order = append(order, 1) })
	p.QueuePendingAction(func() { order = append(order, 2) })
	p.QueuePendingAction(func() { order = append(order, 3) })

	if !p.HasPendingActions() {
		t.Fatal("player should have pending actions")
	}

	p.ExecutePendingActions()

	if p.HasPendingActions() {
		t.Fatal("player must have no pending actions after ExecutePendingActions")
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 actions executed, got %d", len(order))
	}
	// FIFO order (append-then-iterate maintains insertion order).
	for i, v := range order {
		if v != i+1 {
			t.Fatalf("action[%d]=%d, want %d", i, v, i+1)
		}
	}

	// Nil actions must be ignored gracefully.
	p.QueuePendingAction(nil)
	if p.HasPendingActions() {
		t.Fatal("nil action should not have been queued")
	}

	// ExecutePendingActions on empty player must be a no-op.
	p.ExecutePendingActions()
}

func TestLunarModInfoTypeHelpers(t *testing.T) {
	fabric := LunarModInfo{Type: "FABRIC"}
	if !fabric.IsFabric() {
		t.Error("expected IsFabric() for FABRIC type")
	}
	if fabric.IsForge() {
		t.Error("expected !IsForge() for FABRIC type")
	}

	forge := LunarModInfo{Type: "FORGE"}
	if !forge.IsForge() {
		t.Error("expected IsForge() for FORGE type")
	}
	if forge.IsFabric() {
		t.Error("expected !IsFabric() for FORGE type")
	}

	empty := LunarModInfo{}
	if empty.IsFabric() || empty.IsForge() {
		t.Error("empty type should be neither Fabric nor Forge")
	}
}

func TestForgeModInfoString(t *testing.T) {
	withVersion := ForgeModInfo{ModID: "wurst", Version: "1.0"}
	if withVersion.String() != "wurst (1.0)" {
		t.Errorf("unexpected String(): %q", withVersion.String())
	}

	noVersion := ForgeModInfo{ModID: "wurst"}
	if noVersion.String() != "wurst" {
		t.Errorf("unexpected String(): %q", noVersion.String())
	}
}

func TestForgeClientTypeString(t *testing.T) {
	if ForgeClientForge.String() != "Forge" {
		t.Errorf("unexpected String for ForgeClientForge: %q", ForgeClientForge.String())
	}
	if ForgeClientNeoForge.String() != "NeoForge" {
		t.Errorf("unexpected String for ForgeClientNeoForge: %q", ForgeClientNeoForge.String())
	}
}
