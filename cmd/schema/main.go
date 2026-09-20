package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
)

// JSONSchemaType represents a JSON Schema type
type JSONSchemaType struct {
	Type                 string           `json:"type,omitempty"`
	Description          string           `json:"description,omitempty"`
	Properties           map[string]any   `json:"properties,omitempty"`
	Required             []string         `json:"required,omitempty"`
	AdditionalProperties any              `json:"additionalProperties,omitempty"`
	Enum                 []any            `json:"enum,omitempty"`
	Items                map[string]any   `json:"items,omitempty"`
	OneOf                []map[string]any `json:"oneOf,omitempty"`
	AnyOf                []map[string]any `json:"anyOf,omitempty"`
	Default              any              `json:"default,omitempty"`
}

func main() {
	schema := generateSchema()

	// Pretty print the schema
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding schema: %v\n", err)
		os.Exit(1)
	}
}

func generateSchema() map[string]any {
	schema := map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"title":       "OwnCode Configuration",
		"description": "Configuration schema for the OwnCode application",
		"type":        "object",
		"properties":  map[string]any{},
	}

	properties := schema["properties"].(map[string]any)
	properties["activeProfile"] = map[string]any{"type": "string", "default": "build", "description": "Built-in build/plan/witch or a configured profile name."}
	witchLanes := map[string]any{}
	for _, lane := range config.WitchLanes() {
		witchLanes[lane] = map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
			"model":     map[string]any{"type": "string", "description": "Configured model ID; omit to use the base chat model."},
			"reasoning": map[string]any{"type": "string", "description": "A reasoning level supported by this role's model."},
		}}
	}
	properties["witch"] = map[string]any{"type": "object", "additionalProperties": false, "description": "Global settings for the built-in Witch orchestrator.", "properties": map[string]any{
		"lanes": map[string]any{"type": "object", "additionalProperties": false, "properties": witchLanes},
	}}
	properties["profiles"] = map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"description": map[string]any{"type": "string"}, "model": map[string]any{"type": "string"}, "reasoning": map[string]any{"type": "string"}, "prompt": map[string]any{"type": "string"}, "readOnly": map[string]any{"type": "boolean"}, "tools": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	}}}
	properties["extensions"] = map[string]any{"type": "object", "description": "Trusted metadata hooks. Only global configuration can enable them.", "additionalProperties": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"command"}, "properties": map[string]any{
		"enabled": map[string]any{"type": "boolean", "default": false}, "command": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}, "description": "Absolute executable path, followed by arguments."},
	}}}

	// Add Data configuration
	schema["properties"].(map[string]any)["data"] = map[string]any{
		"type":        "object",
		"description": "Storage configuration",
		"properties": map[string]any{
			"directory": map[string]any{
				"type":        "string",
				"description": "Directory where application data is stored",
				"default":     ".owncode",
			},
		},
		"required": []string{"directory"},
	}

	// Add working directory
	schema["properties"].(map[string]any)["wd"] = map[string]any{
		"type":        "string",
		"description": "Working directory for the application",
	}

	// Add debug flags
	schema["properties"].(map[string]any)["debug"] = map[string]any{
		"type":        "boolean",
		"description": "Enable debug mode",
		"default":     false,
	}

	schema["properties"].(map[string]any)["debugLSP"] = map[string]any{
		"type":        "boolean",
		"description": "Enable LSP debug mode",
		"default":     false,
	}

	schema["properties"].(map[string]any)["contextPaths"] = map[string]any{
		"type":        "array",
		"description": "Context paths for the application",
		"items": map[string]any{
			"type": "string",
		},
		"default": []string{
			".github/copilot-instructions.md",
			".cursorrules",
			".cursor/rules/",
			"CLAUDE.md",
			"CLAUDE.local.md",
			"owncode.md",
			"owncode.local.md",
			"OwnCode.md",
			"OwnCode.local.md",
			"OWNCODE.md",
			"OWNCODE.local.md",
		},
	}

	schema["properties"].(map[string]any)["autoCompact"] = map[string]any{
		"type": "boolean", "default": true, "description": "Compact automatically after a completed turn reaches the context threshold.",
	}
	schema["properties"].(map[string]any)["compaction"] = map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"method": map[string]any{"type": "string", "enum": []string{"summary", "shake", "snapcompact", "jev", "native"}, "default": "summary"},
			"jev": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"apiKey":       map[string]any{"type": "string", "description": "Required for Jev compaction. Keep in a private local config.", "writeOnly": true},
				"model":        map[string]any{"type": "string", "default": "jev-latest"},
				"targetTokens": map[string]any{"type": "integer", "minimum": 1, "description": "Text token target. Default: 70% of the active text context."},
			}},
			"mode":      map[string]any{"type": "string", "enum": []string{"balanced", "brief", "handoff"}, "default": "balanced"},
			"focus":     map[string]any{"type": "string", "description": "Additional instructions about information to retain in the summary."},
			"threshold": map[string]any{"type": "integer", "minimum": 50, "maximum": 95, "default": 95},
		},
	}

	schema["properties"].(map[string]any)["tui"] = map[string]any{
		"type":        "object",
		"description": "Terminal User Interface configuration",
		"properties": map[string]any{
			"theme": map[string]any{
				"type":        "string",
				"description": "TUI theme name",
				"default":     "owncode",
				"enum": []string{
					"owncode",
					"catppuccin",
					"dracula",
					"flexoki",
					"gruvbox",
					"monokai",
					"onedark",
					"tokyonight",
					"tron",
				},
			},
		},
	}

	// Add MCP servers
	schema["properties"].(map[string]any)["mcpServers"] = map[string]any{
		"type":        "object",
		"description": "Model Control Protocol server configurations",
		"additionalProperties": map[string]any{
			"type":        "object",
			"description": "MCP server configuration",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Command to execute for the MCP server",
				},
				"env": map[string]any{
					"type":        "array",
					"description": "Environment variables for the MCP server",
					"items": map[string]any{
						"type": "string",
					},
				},
				"args": map[string]any{
					"type":        "array",
					"description": "Command arguments for the MCP server",
					"items": map[string]any{
						"type": "string",
					},
				},
				"type": map[string]any{
					"type":        "string",
					"description": "Type of MCP server",
					"enum":        []string{"stdio", "sse"},
					"default":     "stdio",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "URL for SSE type MCP servers",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "HTTP headers for SSE type MCP servers",
					"additionalProperties": map[string]any{
						"type": "string",
					},
				},
			},
			"required": []string{"command"},
		},
	}

	// Add providers
	providerSchema := map[string]any{
		"type":        "object",
		"description": "LLM provider configurations",
		"additionalProperties": map[string]any{
			"type":        "object",
			"description": "Provider configuration",
			"properties": map[string]any{
				"apiKey": map[string]any{
					"type":        "string",
					"description": "API key for the provider",
				},
				"baseURL": map[string]any{"type": "string", "format": "uri", "description": "OpenAI-compatible endpoint"},
				"models": map[string]any{
					"type": "object",
					"additionalProperties": map[string]any{
						"type":     "object",
						"required": []string{"name", "contextWindow", "maxTokens"},
						"properties": map[string]any{
							"name":             map[string]any{"type": "string"},
							"contextWindow":    map[string]any{"type": "integer", "minimum": 1},
							"maxTokens":        map[string]any{"type": "integer", "minimum": 1},
							"reasoning":        map[string]any{"type": "boolean"},
							"reasoningLevels":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"defaultReasoning": map[string]any{"type": "string"},
							"attachments":      map[string]any{"type": "boolean"},
							"interleaved":      map[string]any{"enum": []string{"reasoning_content"}},
							"options":          map[string]any{"type": "object"},
						},
					},
				},
				"disabled": map[string]any{
					"type":        "boolean",
					"description": "Whether the provider is disabled",
					"default":     false,
				},
			},
		},
	}

	// Add known providers
	knownProviders := []string{
		string(models.ProviderAnthropic),
		string(models.ProviderOpenAI),
		string(models.ProviderGemini),
		string(models.ProviderGROQ),
		string(models.ProviderOpenRouter),
		string(models.ProviderBedrock),
		string(models.ProviderAzure),
		string(models.ProviderVertexAI),
	}

	providerSchema["additionalProperties"].(map[string]any)["properties"].(map[string]any)["provider"] = map[string]any{
		"type":        "string",
		"description": "Provider type",
		"enum":        knownProviders,
	}

	schema["properties"].(map[string]any)["providers"] = providerSchema

	// Add agents
	agentSchema := map[string]any{
		"type":        "object",
		"description": "Agent configurations",
		"additionalProperties": map[string]any{
			"type":        "object",
			"description": "Agent configuration",
			"properties": map[string]any{
				"model": map[string]any{
					"type":        "string",
					"description": "Model ID for the agent",
				},
				"maxTokens": map[string]any{
					"type":        "integer",
					"description": "Maximum tokens for the agent",
					"minimum":     1,
				},
				"reasoningEffort": map[string]any{
					"type":        "string",
					"description": "A reasoning level supported by the selected model, or on/off for compatible thinking models",
				},
			},
			"required": []string{"model"},
		},
	}

	// Add model enum
	modelEnum := []string{}
	for modelID := range models.SupportedModels {
		modelEnum = append(modelEnum, string(modelID))
	}
	slices.Sort(modelEnum)
	agentSchema["additionalProperties"].(map[string]any)["properties"].(map[string]any)["model"].(map[string]any)["anyOf"] = []map[string]any{
		{"enum": modelEnum},
		{"pattern": "^[^/]+/.+$"},
	}

	// Add specific agent properties
	agentProperties := map[string]any{}
	knownAgents := []string{
		string(config.AgentCoder),
		string(config.AgentTask),
		string(config.AgentTitle),
	}

	for _, agentName := range knownAgents {
		agentProperties[agentName] = map[string]any{
			"$ref": "#/definitions/agent",
		}
	}

	// Create a combined schema that allows both specific agents and additional ones
	combinedAgentSchema := map[string]any{
		"type":                 "object",
		"description":          "Agent configurations",
		"properties":           agentProperties,
		"additionalProperties": agentSchema["additionalProperties"],
	}

	schema["properties"].(map[string]any)["agents"] = combinedAgentSchema
	schema["definitions"] = map[string]any{
		"agent": agentSchema["additionalProperties"],
	}

	// Add LSP configuration
	schema["properties"].(map[string]any)["lsp"] = map[string]any{
		"type":        "object",
		"description": "Language Server Protocol configurations",
		"additionalProperties": map[string]any{
			"type":        "object",
			"description": "LSP configuration for a language",
			"properties": map[string]any{
				"disabled": map[string]any{
					"type":        "boolean",
					"description": "Whether the LSP is disabled",
					"default":     false,
				},
				"command": map[string]any{
					"type":        "string",
					"description": "Command to execute for the LSP server",
				},
				"args": map[string]any{
					"type":        "array",
					"description": "Command arguments for the LSP server",
					"items": map[string]any{
						"type": "string",
					},
				},
				"options": map[string]any{
					"type":        "object",
					"description": "Additional options for the LSP server",
				},
			},
			"required": []string{"command"},
		},
	}

	return schema
}
