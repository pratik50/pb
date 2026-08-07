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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/parseablehq/pb/pkg/analytics"
	"github.com/parseablehq/pb/pkg/config"
	internalHTTP "github.com/parseablehq/pb/pkg/http"
	"github.com/parseablehq/pb/pkg/ui"
	"github.com/spf13/cobra"
)

var StatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Check connection status for the active profile",
	Example: "  pb status\n  pb status -o json",
	RunE: func(cmd *cobra.Command, _ []string) error {
		outputFormat, err := commandOutputFormat(cmd)
		if err != nil {
			return err
		}

		fileConfig, err := config.ReadConfigFromFile()
		if err != nil {
			return statusPreflightError(outputFormat, "no profile configured. run: pb login")
		}

		profileName := fileConfig.DefaultProfile
		profile, exists := fileConfig.Profiles[profileName]
		if !exists || profileName == "" {
			return statusPreflightError(outputFormat, "no active profile. run: pb login")
		}

		client := internalHTTP.DefaultClient(&profile)
		about, err := analytics.FetchAbout(&client)
		if err != nil {
			statusMessage := statusErrorMessage(err)
			if outputFormat == outputJSON {
				if jsonErr := printStatusJSON(statusOutput{
					Status:  "error",
					Healthy: false,
					Profile: profileName,
					URL:     profile.URL,
					Error:   statusMessage,
				}); jsonErr != nil {
					return jsonErr
				}
				return MarkErrorRendered(fmt.Errorf("status check failed: %s", statusMessage))
			}
			errStyle := lipgloss.NewStyle().Foreground(ui.Active.Err).Bold(true)
			fmt.Fprintf(cmd.ErrOrStderr(), "Profile : %s\n", profileName)
			fmt.Fprintf(cmd.ErrOrStderr(), "URL     : %s\n", profile.URL)
			fmt.Fprintf(cmd.ErrOrStderr(), "Status  : %s\n", errStyle.Render("✗ Not connected"))
			fmt.Fprintf(cmd.ErrOrStderr(), "Error   : %s\n", statusMessage)
			return fmt.Errorf("status check failed: %s", statusMessage)
		}

		if outputFormat == outputJSON {
			return printStatusJSON(statusOutput{
				Status:  "ok",
				Healthy: true,
				Profile: profileName,
				URL:     profile.URL,
				Version: about.Version,
			})
		}

		okStyle := lipgloss.NewStyle().Foreground(ui.Active.Ok).Bold(true)
		fmt.Printf("Profile : %s\n", profileName)
		fmt.Printf("URL     : %s\n", profile.URL)
		fmt.Printf("Status  : %s\n", okStyle.Render("✓ Connected"))
		fmt.Printf("Version : %s\n", about.Version)
		return nil
	},
}

type statusOutput struct {
	Status  string `json:"status"`
	Healthy bool   `json:"healthy"`
	Profile string `json:"profile,omitempty"`
	URL     string `json:"url,omitempty"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

func statusPreflightError(outputFormat, message string) error {
	if outputFormat == outputJSON {
		if err := printStatusJSON(statusOutput{
			Status:  "error",
			Healthy: false,
			Error:   message,
		}); err != nil {
			return err
		}
		return MarkErrorRendered(errors.New(message))
	}
	return errors.New(message)
}

func printStatusJSON(result statusOutput) error {
	return writeJSON(os.Stdout, result)
}

func statusErrorMessage(err error) string {
	message := err.Error()
	if strings.Contains(message, "Status Code: 401") || strings.Contains(message, "Status Code: 403") {
		return "Authentication failed: invalid username/password or API key"
	}
	return message
}

func init() {
	StatusCmd.Flags().StringP("output", "o", "text", "Output format (text|json)")
}
