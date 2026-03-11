package detection

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	guuid "github.com/google/uuid"
	"go.minekube.com/brigodier"
	. "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// configHolder is a thread-safe wrapper that holds the current DetectionConfig
// and the resources directory path needed to reload it.
type configHolder struct {
	mu           sync.RWMutex
	cfg          *DetectionConfig
	resourcesDir string
}

// newConfigHolder creates a configHolder with the given initial config and
// resources directory path (as returned by filepath.Dir(paths.Config)).
func newConfigHolder(cfg *DetectionConfig, resourcesDir string) *configHolder {
	return &configHolder{cfg: cfg, resourcesDir: resourcesDir}
}

// get returns a snapshot of the current config. Safe to call from multiple goroutines.
func (h *configHolder) get() *DetectionConfig {
	h.mu.RLock()
	c := h.cfg
	h.mu.RUnlock()
	return c
}

// reload reloads the TOML config files from the stored resources directory and
// atomically replaces the current config on success.
func (h *configHolder) reload() error {
	newCfg, err := LoadDetectionConfig(h.resourcesDir)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.cfg = newCfg
	h.mu.Unlock()
	return nil
}

// resourcesDirFromBaseDir resolves the resources directory from a repo base dir.
// Convenience wrapper used by Init (T14).
func resourcesDirFromBaseDir(baseDir string) (string, error) {
	paths, err := ResolveSubmodulePaths(baseDir)
	if err != nil {
		return "", err
	}
	return filepath.Dir(paths.Config), nil
}

// gateUUIDToGoogle converts a Gate-internal UUID ([16]byte alias) to github.com/google/uuid.UUID.
// Gate's uuid.UUID is defined as `type UUID guuid.UUID` so a simple type-cast works.
func gateUUIDToGoogle(id [16]byte) guuid.UUID {
	return guuid.UUID(id)
}

// newDetectionCommand builds the /detection (alias: /hs) command tree.
//
// Subcommands:
//
//	/detection reload           – reloads TOML config from the submodule
//	/detection check <player>   – shows what is known about a player
//	/detection list             – lists all players with generic checks
//
// Permissions:
//
//	hackedserver.command           – required to use /detection at all
//	hackedserver.command.reload    – required for the reload subcommand
//	hackedserver.command.check     – required for the check subcommand
//	hackedserver.command.list      – required for the list subcommand
func newDetectionCommand(
	p *proxy.Proxy,
	store *PlayerStore,
	cfgHolder *configHolder,
) brigodier.LiteralNodeBuilder {
	const (
		permBase   = "hackedserver.command"
		permReload = "hackedserver.command.reload"
		permCheck  = "hackedserver.command.check"
		permList   = "hackedserver.command.list"
	)

	const playerArg = "player"

	return brigodier.Literal("detection").
		Requires(command.Requires(func(c *command.RequiresContext) bool {
			return c.Source.HasPermission(permBase)
		})).
		Executes(command.Command(func(c *command.Context) error {
			return c.Source.SendMessage(&Text{Content: "Usage: /detection <reload|check|list>"})
		})).
		Then(
			brigodier.Literal("reload").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return c.Source.HasPermission(permReload)
				})).
				Executes(command.Command(func(c *command.Context) error {
					return handleReload(c, cfgHolder)
				})),
		).
		Then(
			brigodier.Literal("check").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return c.Source.HasPermission(permCheck)
				})).
				Then(
					brigodier.Argument(playerArg, brigodier.String).
						Executes(command.Command(func(c *command.Context) error {
							return handleCheck(c, p, store, cfgHolder, c.String(playerArg))
						})),
				),
		).
		Then(
			brigodier.Literal("list").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return c.Source.HasPermission(permList)
				})).
				Executes(command.Command(func(c *command.Context) error {
					return handleList(c, p, store)
				})),
		)
}

// handleReload reloads the TOML configuration from the submodule.
func handleReload(c *command.Context, cfgHolder *configHolder) error {
	if err := cfgHolder.reload(); err != nil {
		return c.Source.SendMessage(&Text{Content: fmt.Sprintf("Detection: reload failed: %v", err)})
	}
	return c.Source.SendMessage(&Text{Content: "Detection: configuration reloaded."})
}

// handleCheck shows detected mod information for the named player.
func handleCheck(
	c *command.Context,
	p *proxy.Proxy,
	store *PlayerStore,
	cfgHolder *configHolder,
	username string,
) error {
	// Look up online player by name.
	target := p.PlayerByName(username)
	if target == nil {
		return c.Source.SendMessage(&Text{
			Content: fmt.Sprintf("Player %q is not online.", username),
		})
	}

	return formatCheckOutput(c, target.Username(), store.Get(gateUUIDToGoogle(target.ID())), cfgHolder.get())
}

// formatCheckOutput renders the /detection check output for a known player.
// It is extracted from handleCheck for testability without a real proxy.
func formatCheckOutput(
	c *command.Context,
	username string,
	dp *DetectedPlayer,
	cfg *DetectionConfig,
) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Detection info for %s:\n", username))

	// ── Generic checks ────────────────────────────────────────────────────────
	checks := dp.GenericChecks()
	sort.Strings(checks)
	if len(checks) > 0 {
		b.WriteString("  Generic checks: ")
		b.WriteString(strings.Join(checks, ", "))
		b.WriteString("\n")
	} else {
		b.WriteString("  Generic checks: none\n")
	}

	// ── Forge mods (if ShowModsInCheck is enabled) ─────────────────────────
	if cfg.Forge.Settings.ShowModsInCheck && dp.HasForgeModsData() {
		mods := dp.ForgeMods()
		if len(mods) > 0 {
			sort.Slice(mods, func(i, j int) bool {
				return mods[i].ModID < mods[j].ModID
			})
			b.WriteString("  Forge mods:\n")
			for _, m := range mods {
				if cfg.Forge.Settings.ShowModVersions && m.Version != "" {
					b.WriteString(fmt.Sprintf("    - %s (%s)\n", m.ModID, m.Version))
				} else {
					b.WriteString(fmt.Sprintf("    - %s\n", m.ModID))
				}
			}
		} else {
			b.WriteString("  Forge mods: none\n")
		}
	}

	// ── Lunar mods (if enabled and ShowModsInCheck is enabled) ────────────
	if cfg.Lunar.Enabled && cfg.Lunar.Settings.ShowModsInCheck && dp.HasLunarModsData() {
		mods := dp.LunarMods()
		if len(mods) > 0 {
			sort.Slice(mods, func(i, j int) bool {
				return mods[i].ID < mods[j].ID
			})
			b.WriteString("  Lunar mods:\n")
			for _, m := range mods {
				line := fmt.Sprintf("    - %s", m.ID)
				if cfg.Lunar.Settings.ShowModVersions && m.Version != "" {
					line += fmt.Sprintf(" (%s)", m.Version)
				}
				if cfg.Lunar.Settings.ShowModTypes && m.Type != "" {
					line += fmt.Sprintf(" [%s]", m.Type)
				}
				b.WriteString(line + "\n")
			}
		} else {
			b.WriteString("  Lunar mods: none\n")
		}
	}

	// ── Bedrock ───────────────────────────────────────────────────────────
	if dp.IsBedrockDetected() {
		b.WriteString("  Bedrock: yes\n")
	}

	return c.Source.SendMessage(&Text{Content: b.String()})
}

// handleList lists all online players that have at least one generic check triggered.
func handleList(
	c *command.Context,
	p *proxy.Proxy,
	store *PlayerStore,
) error {
	var entries []namedCheckEntry
	for _, player := range p.Players() {
		dp := store.Get(gateUUIDToGoogle(player.ID()))
		checks := dp.GenericChecks()
		if len(checks) > 0 {
			sort.Strings(checks)
			entries = append(entries, namedCheckEntry{name: player.Username(), checks: checks})
		}
	}
	return handleListFromEntries(c, entries)
}

// namedCheckEntry is a player name paired with its triggered generic check IDs.
type namedCheckEntry struct {
	name   string
	checks []string
}

// handleListFromEntries formats and sends the /detection list output.
// entries may be nil or empty to produce the "no players" message.
// Extracted for testability without a real *proxy.Proxy.
func handleListFromEntries(c *command.Context, entries []namedCheckEntry) error {
	if len(entries) == 0 {
		return c.Source.SendMessage(&Text{Content: "No players with generic checks detected."})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Detected players (%d):\n", len(entries)))
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("  %s: %s\n", e.name, strings.Join(e.checks, ", ")))
	}
	return c.Source.SendMessage(&Text{Content: b.String()})
}
