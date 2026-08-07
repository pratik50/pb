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
	"fmt"
	"time"

	"github.com/parseablehq/pb/pkg/analytics"
	internalHTTP "github.com/parseablehq/pb/pkg/http"
	"github.com/spf13/cobra"
)

// VersionCmd is the command for printing version information
var VersionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print version",
	Long:    "Print version and commit information",
	Example: "  pb version",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if cmd.Annotations == nil {
			cmd.Annotations = make(map[string]string)
		}

		startTime := time.Now()
		defer func() {
			// Capture the execution time in annotations
			cmd.Annotations["executionTime"] = time.Since(startTime).String()
		}()

		err := PrintVersion(cmd, "1.0.0", "abc123") // Replace with actual version and commit values
		if err != nil {
			cmd.Annotations["error"] = err.Error()
		}
		return err
	},
}

func init() {
	VersionCmd.Flags().StringP("output", "o", "text", "Output format (text|json)")
}

// PrintVersion prints version information
func PrintVersion(cmd *cobra.Command, version, commit string) error {
	format, err := commandOutputFormat(cmd)
	if err != nil {
		return err
	}
	client := internalHTTP.DefaultClient(&DefaultProfile)

	// Fetch server information
	if err := PreRun(); err != nil {
		return fmt.Errorf("error in PreRun: %w", err)
	}

	about, err := analytics.FetchAbout(&client)
	if err != nil {
		return fmt.Errorf("error fetching server information: %w", err)
	}

	// Output as JSON if specified
	if format == outputJSON {
		versionInfo := map[string]interface{}{
			"client": map[string]string{
				"version": version,
				"commit":  commit,
			},
			"server": map[string]string{
				"url":     DefaultProfile.URL,
				"version": about.Version,
				"commit":  about.Commit,
			},
		}
		return writeJSON(cmd.OutOrStdout(), versionInfo)
	}

	// Default: Output as text
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n%s \n", StandardStyleAlt.Render("pb version"))
	fmt.Fprintf(out, "- %s %s\n", StandardStyleBold.Render("version: "), version)
	fmt.Fprintf(out, "- %s %s\n\n", StandardStyleBold.Render("commit:  "), commit)

	fmt.Fprintf(out, "%s %s \n", StandardStyleAlt.Render("Connected to"), StandardStyleBold.Render(DefaultProfile.URL))
	fmt.Fprintf(out, "- %s %s\n", StandardStyleBold.Render("version: "), about.Version)
	fmt.Fprintf(out, "- %s %s\n\n", StandardStyleBold.Render("commit:  "), about.Commit)

	return nil
}
