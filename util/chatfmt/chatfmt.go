package chatfmt

import (
	"strings"

	sharedcfg "github.com/minekube/gate-plugin-template/plugins/sharedconfig"
	"github.com/minekube/gate-plugin-template/util/mini"
	"go.minekube.com/common/minecraft/component"
)

func Render(pluginName string, pluginPrefix string, message string) component.Component {
	cfg, err := sharedcfg.Load()
	if err != nil || cfg == nil {
		if pluginPrefix == "" {
			pluginPrefix = "<gray>[<aqua>{plugin}</aqua>]</gray> "
		}
		prefix := strings.ReplaceAll(pluginPrefix, "{plugin}", pluginName)
		return mini.Parse(prefix + "<gray>" + message + "</gray>")
	}

	prefixTemplate := cfg.Global.Chat.Format.Prefix
	if pluginPrefix != "" {
		prefixTemplate = pluginPrefix
	}

	prefix := strings.ReplaceAll(prefixTemplate, "{plugin}", pluginName)
	body := strings.ReplaceAll(cfg.Global.Chat.Format.Message, "{message}", message)
	return mini.Parse(prefix + body)
}

func ApplyPlaceholders(template string, placeholders map[string]string) string {
	out := template
	for k, v := range placeholders {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}
