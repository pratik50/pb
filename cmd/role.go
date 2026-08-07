// Copyright (c) 2024 Parseable, Inc
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	internalHTTP "github.com/parseablehq/pb/pkg/http"
	"github.com/parseablehq/pb/pkg/model/role"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type RoleResource struct {
	Dataset string `json:"dataset,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Stream  string `json:"stream,omitempty"` // Legacy response compatibility.
}

type RoleData struct {
	Privilege string        `json:"privilege"`
	Resource  *RoleResource `json:"resource,omitempty"`
}

func (user *RoleData) Render() string {
	var s strings.Builder
	s.WriteString(StandardStyle.Render("Privilege: "))
	s.WriteString(StandardStyleAlt.Render(user.Privilege))
	s.WriteString("\n")
	if user.Resource != nil {
		if dataset := roleDataset(*user.Resource); dataset != "" {
			s.WriteString(StandardStyle.Render("Dataset:   "))
			s.WriteString(StandardStyleAlt.Render(dataset))
			s.WriteString("\n")
		}
		if user.Resource.Tag != "" {
			s.WriteString(StandardStyle.Render("Tag:       "))
			s.WriteString(StandardStyleAlt.Render(user.Resource.Tag))
			s.WriteString("\n")
		}
	}

	return s.String()
}

var AddRoleCmd = &cobra.Command{
	Use:     "add role-name",
	Example: "  pb role add ingestors",
	Short:   "Add a new role",
	Long:    "Add a new role using an interactive prompt that asks only for its privilege.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		cmd.Annotations = make(map[string]string)
		defer func() {
			cmd.Annotations["executionTime"] = time.Since(startTime).String()
		}()

		name := args[0]

		var roles []string
		client := internalHTTP.DefaultClient(&DefaultProfile)
		if err := fetchRoles(&client, &roles); err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error fetching roles: %s", err.Error())
			return err
		}

		if slices.Contains(roles, name) {
			commandErr := newCLIError(ErrorConflict, fmt.Sprintf("role %s already exists; use a different name", name), nil)
			cmd.Annotations["errors"] = commandErr.Error()
			return commandErr
		}

		_m, err := tea.NewProgram(role.New()).Run()
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error initializing program: %s", err.Error())
			return err
		}

		m := _m.(role.Model)
		privilege := m.Selection.Value()

		if !m.Success {
			fmt.Println("aborted by user")
			return nil
		}

		roleData := newRoleData(privilege)
		roleDataJSON, err := json.Marshal([]RoleData{roleData})
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error encoding role: %s", err.Error())
			return err
		}
		var putBody io.Reader = bytes.NewBuffer(roleDataJSON)

		req, err := client.NewRequest("PUT", "role/"+name, putBody)
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error creating request: %s", err.Error())
			return err
		}

		resp, err := client.Client.Do(req)
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error performing request: %s", err.Error())
			return err
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error reading response: %s", err.Error())
			return err
		}
		if resp.StatusCode != http.StatusOK {
			requestErr := responseStatusError("create role", resp.StatusCode, resp.Status, bodyBytes)
			cmd.Annotations["errors"] = requestErr.Error()
			return requestErr
		}
		fmt.Printf("Added role %s", name)
		return nil
	},
}

var RemoveRoleCmd = &cobra.Command{
	Use:     "remove role-name",
	Aliases: []string{"rm"},
	Example: "  pb role remove ingestor\n  pb role rm ingestor",
	Short:   "Delete a role",
	Long:    "Delete an existing role. If the role name is missing, show the available role names without sending a delete request.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		cmd.Annotations = make(map[string]string)
		defer func() {
			cmd.Annotations["executionTime"] = time.Since(startTime).String()
		}()

		name := args[0]
		client := internalHTTP.DefaultClient(&DefaultProfile)
		var roles []string
		if err := fetchRoles(&client, &roles); err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error fetching roles: %s", err.Error())
			return err
		}
		if !slices.Contains(roles, name) {
			commandErr := newCLIError(ErrorNotFound, missingRoleMessage(name, roles), nil)
			cmd.Annotations["errors"] = commandErr.Error()
			return commandErr
		}

		req, err := client.NewRequest("DELETE", "role/"+name, nil)
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error creating delete request: %s", err.Error())
			return err
		}

		resp, err := client.Client.Do(req)
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error performing delete request: %s", err.Error())
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				cmd.Annotations["errors"] = fmt.Sprintf("Error reading response: %s", err.Error())
				return err
			}
			requestErr := responseStatusError("delete role", resp.StatusCode, resp.Status, bodyBytes)
			cmd.Annotations["errors"] = requestErr.Error()
			return requestErr
		}
		fmt.Printf("Removed role %s\n", StyleBold.Render(name))
		return nil
	},
}

var ListRoleCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List all roles",
	Example:      "  pb role list",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		startTime := time.Now()
		cmd.Annotations = make(map[string]string)
		defer func() {
			cmd.Annotations["executionTime"] = time.Since(startTime).String()
		}()

		var roles []string
		client := internalHTTP.DefaultClient(&DefaultProfile)
		err := fetchRoles(&client, &roles)
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error fetching roles: %s", err.Error())
			return err
		}

		outputFormat, err := commandOutputFormat(cmd)
		if err != nil {
			cmd.Annotations["errors"] = fmt.Sprintf("Error retrieving output flag: %s", err.Error())
			return err
		}

		roleResponses := make([]struct {
			data []RoleData
			err  error
		}, len(roles))

		var wg sync.WaitGroup
		for idx, role := range roles {
			wg.Add(1)
			go func(idx int, role string) {
				defer wg.Done()
				roleResponses[idx].data, roleResponses[idx].err = fetchSpecificRole(&client, role)
			}(idx, role)
		}
		wg.Wait()
		var fetchErrors []string
		for idx, roleName := range roles {
			if roleResponses[idx].err != nil {
				errMsg := fmt.Sprintf("Error fetching role data for %s: %v", roleName, roleResponses[idx].err)
				fetchErrors = append(fetchErrors, errMsg)
				cmd.Annotations["errors"] += errMsg + "\n"
			}
		}
		var detailsErr error
		if len(fetchErrors) > 0 {
			detailsErr = fmt.Errorf("failed to fetch details for %d role(s): %s", len(fetchErrors), strings.Join(fetchErrors, "; "))
		}

		if outputFormat == outputJSON {
			if detailsErr != nil {
				return detailsErr
			}
			allRoles := map[string][]RoleData{}
			for idx, roleName := range roles {
				if roleResponses[idx].err == nil {
					allRoles[roleName] = roleResponses[idx].data
				}
			}
			if err := writeJSON(cmd.OutOrStdout(), allRoles); err != nil {
				return err
			}
			return nil
		}

		printRoleTable(roles, roleResponses)
		return detailsErr
	},
}

func printRoleTable(roles []string, roleResponses []struct {
	data []RoleData
	err  error
},
) {
	const maxRoleWidth = 42
	const maxStreamWidth = 48

	roleWidth := lipgloss.Width("ROLE")
	privilegeWidth := lipgloss.Width("PRIVILEGE")
	streamWidth := lipgloss.Width("DATASET")

	for idx, roleName := range roles {
		roleW := lipgloss.Width(roleName)
		if roleW > maxRoleWidth {
			roleW = maxRoleWidth
		}
		if roleW > roleWidth {
			roleWidth = roleW
		}
		for _, action := range roleResponses[idx].data {
			if w := lipgloss.Width(action.Privilege); w > privilegeWidth {
				privilegeWidth = w
			}
			stream := roleStream(action)
			streamW := lipgloss.Width(stream)
			if streamW > maxStreamWidth {
				streamW = maxStreamWidth
			}
			if streamW > streamWidth {
				streamWidth = streamW
			}
		}
	}

	headerStyle := SelectedStyle.Bold(true)
	bodyStyle := StandardStyle
	mutedStyle := StandardStyleAlt
	ruleStyle := StandardStyleRule
	leaderStyle := StandardStyleRule
	privilegeColumn := roleWidth + 4
	streamColumn := privilegeColumn + privilegeWidth + 4

	renderRoleGuide := func(roleCell string) string {
		if roleCell == "" {
			return strings.Repeat(" ", privilegeColumn)
		}
		leaderWidth := privilegeColumn - lipgloss.Width(roleCell) - 1
		if leaderWidth < 2 {
			leaderWidth = 2
		}
		return bodyStyle.Render(roleCell) + " " + leaderStyle.Render(strings.Repeat(".", leaderWidth))
	}

	renderPrivilegeGuide := func(privilegeCell string, style lipgloss.Style) string {
		privilegeCell = truncateCell(privilegeCell, privilegeWidth)
		leaderWidth := streamColumn - privilegeColumn - lipgloss.Width(privilegeCell) - 1
		if leaderWidth < 2 {
			leaderWidth = 2
		}
		return style.Render(privilegeCell) + " " + leaderStyle.Render(strings.Repeat(".", leaderWidth))
	}

	printRow := func(roleCell, privilegeCell string, privilegeStyle lipgloss.Style, streamCell string, streamStyle lipgloss.Style) {
		fmt.Printf("%s %s %s\n",
			renderRoleGuide(roleCell),
			renderPrivilegeGuide(privilegeCell, privilegeStyle),
			streamStyle.Render(truncateCell(streamCell, maxStreamWidth)),
		)
	}

	fmt.Println()
	fmt.Printf("%s%s%s\n",
		headerStyle.Render(padRight("ROLE", privilegeColumn+1)),
		headerStyle.Render(padRight("PRIVILEGE", privilegeWidth+5)),
		headerStyle.Render("DATASET"),
	)
	fmt.Printf("%s%s%s\n",
		ruleStyle.Render(strings.Repeat("─", privilegeColumn+1)),
		ruleStyle.Render(strings.Repeat("─", privilegeWidth+5)),
		ruleStyle.Render(strings.Repeat("─", streamWidth)),
	)

	for idx, roleName := range roles {
		fetchRes := roleResponses[idx]
		if fetchRes.err != nil {
			printRow(
				truncateCell(roleName, maxRoleWidth),
				"-",
				mutedStyle,
				fmt.Sprintf("error: %v", fetchRes.err),
				mutedStyle,
			)
			continue
		}

		if len(fetchRes.data) == 0 {
			printRow(truncateCell(roleName, maxRoleWidth), "-", mutedStyle, "-", mutedStyle)
			continue
		}

		for actionIdx, action := range fetchRes.data {
			roleCell := ""
			if actionIdx == 0 {
				roleCell = truncateCell(roleName, maxRoleWidth)
			}
			stream := roleStream(action)
			streamStyle := bodyStyle
			if stream == "-" {
				streamStyle = mutedStyle
			}
			printRow(roleCell, orDash(action.Privilege), bodyStyle, stream, streamStyle)
		}
	}
	fmt.Println()
}

func roleStream(action RoleData) string {
	if action.Resource == nil {
		return "-"
	}
	if dataset := roleDataset(*action.Resource); dataset != "" {
		return dataset
	}
	return "-"
}

func roleDataset(resource RoleResource) string {
	if resource.Dataset != "" {
		return resource.Dataset
	}
	return resource.Stream
}

func newRoleData(privilege string) RoleData {
	return RoleData{Privilege: privilege}
}

func missingRoleMessage(name string, roles []string) string {
	if len(roles) == 0 {
		return fmt.Sprintf("role %s doesn't exist. No roles are available", name)
	}

	roleNames := append([]string(nil), roles...)
	slices.Sort(roleNames)
	return fmt.Sprintf("role %s doesn't exist. Available role names: %s", name, strings.Join(roleNames, ", "))
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func fetchRoles(client *internalHTTP.HTTPClient, data *[]string) error {
	req, err := client.NewRequest("GET", "role", nil)
	if err != nil {
		return err
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		return err
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var roleMap map[string]json.RawMessage
		if err = json.Unmarshal(bytes, &roleMap); err != nil {
			return err
		}
		for name := range roleMap {
			*data = append(*data, name)
		}
	} else {
		body := string(bytes)
		legacyMessage := fmt.Sprintf("request failed\nstatus code: %s\nresponse: %s", resp.Status, body)
		return httpStatusCLIError(resp.StatusCode, legacyMessage, fmt.Sprintf("request failed: %s", resp.Status))
	}

	return nil
}

func fetchSpecificRole(client *internalHTTP.HTTPClient, role string) (res []RoleData, err error) {
	req, err := client.NewRequest("GET", fmt.Sprintf("role/%s", role), nil)
	if err != nil {
		return
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		return
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var wrapper struct {
			Actions []RoleData `json:"actions"`
		}
		if err = json.Unmarshal(bytes, &wrapper); err != nil {
			return
		}
		res = wrapper.Actions
	} else {
		body := string(bytes)
		legacyMessage := fmt.Sprintf("request failed\nstatus code: %s\nresponse: %s", resp.Status, body)
		err = httpStatusCLIError(resp.StatusCode, legacyMessage, fmt.Sprintf("request failed: %s", resp.Status))
		return
	}

	return
}

func init() {
	// Add the --output flag with default value "text"
	ListRoleCmd.Flags().StringP("output", "o", "text", "Output format: 'text' or 'json'")
}
