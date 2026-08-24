package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/smart-mcp-proxy/mcpproxy-go/internal/cliclient"
	"github.com/smart-mcp-proxy/mcpproxy-go/internal/config"
)

var (
	profileName    string
	profileServers string
	profileTools   string
	profileConfig  string
)

func GetProfileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage reusable API-key scopes and their allowed MCP tools",
		Long: `Manage named access profiles for groups of API keys. Pin agent API keys
to one or more access profiles with mcpproxy token update-access-profiles;
every assigned key is then limited to the profiles' servers and, optionally,
to the specific MCP tools selected for each server. Profiles therefore provide
a reusable second layer of authorization for API keys, beyond each key's own
server scope and permission tier.

A profile selects upstream servers and may allow only specific tools on each
selected server. A tool not selected by the profile is not discoverable or
callable by API keys pinned to that profile.

Tool selections use server=tool1,tool2;server2=tool3 syntax. A server omitted
from --tools allows all tools on that server; server= with no tools denies all.

Examples:
  mcpproxy profile list
  mcpproxy profile create --name readonly --servers github,docs --tools 'github=search_repositories,get_issue'
  mcpproxy profile update readonly --servers github --tools 'github=search_repositories'
  mcpproxy profile delete readonly`,
	}
	cmd.PersistentFlags().StringVar(&profileConfig, "config", "", "Path to configuration file")
	cmd.AddCommand(newProfileListCmd(), newProfileShowCmd(), newProfileCreateCmd(), newProfileUpdateCmd(), newProfileDeleteCmd())
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List configured profiles", RunE: runProfileList}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{Use: "show <name>", Short: "Show a profile and its tool scope", Args: cobra.ExactArgs(1), RunE: runProfileShow}
}

func newProfileCreateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "create", Short: "Create a profile", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return runProfileWrite(false, "") }}
	cmd.Flags().StringVar(&profileName, "name", "", "Profile name (required)")
	cmd.Flags().StringVar(&profileServers, "servers", "", "Comma-separated upstream server names (required)")
	cmd.Flags().StringVar(&profileTools, "tools", "", "Per-server tool allowlists: server=tool1,tool2;server2=tool3")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("servers")
	return cmd
}

func newProfileUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "update <name>", Short: "Update a profile", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return runProfileWrite(true, args[0]) }}
	cmd.Flags().StringVar(&profileServers, "servers", "", "Comma-separated upstream server names")
	cmd.Flags().StringVar(&profileTools, "tools", "", "Per-server tool allowlists: server=tool1,tool2;server2=tool3")
	return cmd
}

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{Use: "delete <name>", Short: "Delete a profile", Args: cobra.ExactArgs(1), RunE: runProfileDelete}
}

func profileClient() (*cliclient.Client, error) {
	cfg, err := loadCLIConfig(profileConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	logger, _ := zap.NewProduction()
	client, ok := newDaemonClient(cfg, logger.Sugar())
	if !ok {
		return nil, fmt.Errorf("mcpproxy daemon is not reachable. Start with: mcpproxy serve")
	}
	return client, nil
}

func getProfilesConfig(ctx context.Context, client *cliclient.Client) ([]interface{}, error) {
	resp, err := client.DoRaw(ctx, http.MethodGet, "/api/v1/config", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var envelope map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to read configuration: %v", envelope["error"])
	}
	data, _ := envelope["data"].(map[string]interface{})
	config, _ := data["config"].(map[string]interface{})
	profiles, _ := config["profiles"].([]interface{})
	return profiles, nil
}

func putProfilesConfig(ctx context.Context, client *cliclient.Client, profiles []interface{}) error {
	body, _ := json.Marshal(map[string]interface{}{"profiles": profiles})
	resp, err := client.DoRaw(ctx, http.MethodPatch, "/api/v1/config", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var e map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("failed to save profiles: %v", e["error"])
	}
	return nil
}

func profileContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func runProfileList(_ *cobra.Command, _ []string) error {
	client, err := profileClient()
	if err != nil {
		return err
	}
	ctx, cancel := profileContext()
	defer cancel()
	profiles, err := getProfilesConfig(ctx, client)
	if err != nil {
		return err
	}
	if ResolveOutputFormat() == "json" {
		b, _ := json.MarshalIndent(map[string]interface{}{"profiles": profiles}, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	for _, raw := range profiles {
		p, _ := raw.(map[string]interface{})
		fmt.Printf("%-20s %v servers\n", p["name"], len(asInterfaceSlice(p["servers"])))
	}
	if len(profiles) == 0 {
		fmt.Println("No profiles configured.")
	}
	return nil
}

func runProfileShow(_ *cobra.Command, args []string) error {
	client, err := profileClient()
	if err != nil {
		return err
	}
	ctx, cancel := profileContext()
	defer cancel()
	profiles, err := getProfilesConfig(ctx, client)
	if err != nil {
		return err
	}
	for _, raw := range profiles {
		p, _ := raw.(map[string]interface{})
		if p["name"] == args[0] {
			b, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(b))
			return nil
		}
	}
	return fmt.Errorf("profile %q not found", args[0])
}

func runProfileWrite(update bool, oldName string) error {
	if !update && !config.IsValidProfileSlug(profileName) {
		return fmt.Errorf("invalid profile name %q", profileName)
	}
	if update {
		profileName = oldName
	}
	servers := splitAndTrim(profileServers)
	if len(servers) == 0 {
		return fmt.Errorf("at least one server is required")
	}
	tools, err := parseProfileTools(profileTools)
	if err != nil {
		return err
	}
	client, err := profileClient()
	if err != nil {
		return err
	}
	ctx, cancel := profileContext()
	defer cancel()
	profiles, err := getProfilesConfig(ctx, client)
	if err != nil {
		return err
	}
	if update {
		found := false
		for _, raw := range profiles {
			if p, _ := raw.(map[string]interface{}); p["name"] == oldName {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("profile %q not found", oldName)
		}
	}
	profiles = filterProfile(profiles, profileName)
	entry := map[string]interface{}{"name": profileName, "servers": servers}
	if tools != nil {
		entry["tools"] = tools
	}
	if err := putProfilesConfig(ctx, client, append(profiles, entry)); err != nil {
		return err
	}
	fmt.Printf("Profile %q saved.\n", profileName)
	return nil
}

func runProfileDelete(_ *cobra.Command, args []string) error {
	client, err := profileClient()
	if err != nil {
		return err
	}
	ctx, cancel := profileContext()
	defer cancel()
	profiles, err := getProfilesConfig(ctx, client)
	if err != nil {
		return err
	}
	next := filterProfile(profiles, args[0])
	if len(next) == len(profiles) {
		return fmt.Errorf("profile %q not found", args[0])
	}
	if err := putProfilesConfig(ctx, client, next); err != nil {
		return err
	}
	fmt.Printf("Profile %q deleted.\n", args[0])
	return nil
}

func filterProfile(profiles []interface{}, name string) []interface{} {
	out := make([]interface{}, 0, len(profiles))
	for _, raw := range profiles {
		p, _ := raw.(map[string]interface{})
		if p["name"] != name {
			out = append(out, raw)
		}
	}
	return out
}
func asInterfaceSlice(value interface{}) []interface{} { v, _ := value.([]interface{}); return v }

func parseProfileTools(raw string) (map[string][]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	out := map[string][]string{}
	for _, group := range strings.Split(raw, ";") {
		parts := strings.SplitN(group, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid --tools entry %q; expected server=tool1,tool2", group)
		}
		out[strings.TrimSpace(parts[0])] = splitAndTrim(parts[1])
	}
	return out, nil
}
