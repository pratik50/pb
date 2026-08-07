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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	internalHTTP "github.com/parseablehq/pb/pkg/http"
	"github.com/parseablehq/pb/pkg/model"
	"github.com/spf13/cobra"
)

var SavedQueryList = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Example:      "pb sql list [-o | --output]",
	Short:        "List of saved queries",
	Long:         "\nShow the list of saved queries for active user",
	SilenceUsage: true,
	PreRunE:      PreRunDefaultProfile,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client := internalHTTP.DefaultClient(&DefaultProfile)

		// Check if the output flag is set
		if outputFlag != "" {
			format, err := validateOutputFormat(outputFlag)
			if err != nil {
				return err
			}
			userSavedQueries, err := fetchFilters(&client)
			if err != nil {
				return err
			}

			if format == outputJSON {
				return writeJSON(cmd.OutOrStdout(), userSavedQueries)
			}
			for _, query := range userSavedQueries {
				// Build the line conditionally
				var parts []string
				if query.Title != "" {
					parts = append(parts, query.Title)
				}
				if query.Stream != "" {
					parts = append(parts, query.Stream)
				}
				if query.Desc != "" {
					parts = append(parts, query.Desc)
				}
				if query.From != "" {
					parts = append(parts, query.From)
				}
				if query.To != "" {
					parts = append(parts, query.To)
				}

				// Join parts with commas and print each query on a new line
				fmt.Println(strings.Join(parts, ", "))
			}
			return nil

		}

		// Normal Saved Queries Menu if output flag not set
		p := model.SavedQueriesMenu()
		if _, err := p.Run(); err != nil {
			return err
		}

		a := model.QueryToApply()
		d := model.QueryToDelete()
		if a.SavedQueryID() != "" || strings.TrimSpace(a.Stream()) != "" {
			if err := savedQueryToPbQuery(a.Stream(), a.StartTime(), a.EndTime()); err != nil {
				return err
			}
		}
		if d.SavedQueryID() != "" {
			if err := deleteSavedQuery(&client, d.SavedQueryID(), d.Title()); err != nil {
				return err
			}
		}
		return nil
	},
}

// Delete a saved query from the list.
func deleteSavedQuery(client *internalHTTP.HTTPClient, savedQueryID, title string) error {
	fmt.Fprintf(os.Stderr, "\nAttempting to delete '%s'", title)
	deleteURL := `filters/` + savedQueryID
	req, err := client.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create saved-query delete request: %w", err)
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete saved query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read saved-query delete response: %w", readErr)
		}
		return responseStatusError("delete saved query", resp.StatusCode, resp.Status, body)
	}
	fmt.Printf("\nSaved Query deleted\n\n")
	return nil
}

// Convert a saved query to executable SQL query and print results to terminal.
func savedQueryToPbQuery(sqlQuery string, start string, end string) error {
	if strings.TrimSpace(sqlQuery) == "" {
		fmt.Println("Empty query selected.")
		return nil
	}

	if start == "" {
		start = "1h"
	}
	if end == "" {
		end = "now"
	}

	sqlQuery = quoteStreamNames(sqlQuery)
	sqlQuery = quoteFieldsWithDots(sqlQuery)

	fmt.Printf("Query: %s\n", sqlQuery)

	client := internalHTTP.DefaultClient(&DefaultProfile)
	err := fetchData(&client, sqlQuery, start, end, "")
	if err != nil {
		return fmt.Errorf("selected saved query failed: %w", err)
	}
	return nil
}

func init() {
	// Add the output flag to the SavedQueryList command
	SavedQueryList.Flags().StringVarP(&outputFlag, "output", "o", "", "Output format (text or json)")
}

type Item struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Stream string `json:"stream"`
	Desc   string `json:"desc"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}

func fetchFilters(client *internalHTTP.HTTPClient) ([]Item, error) {
	req, err := client.NewRequest("GET", "filters", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create saved-query request: %w", err)
	}

	resp, err := client.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch saved queries: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved-query response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		legacyMessage := fmt.Sprintf("failed to fetch saved queries: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		return nil, httpStatusCLIError(resp.StatusCode, legacyMessage, fmt.Sprintf("failed to fetch saved queries: %s", resp.Status))
	}

	var filters []model.Filter
	if err := json.Unmarshal(body, &filters); err != nil {
		return nil, fmt.Errorf("failed to decode saved-query response: %w", err)
	}

	// This returns only the SQL type filters
	userSavedQueries := make([]Item, 0, len(filters))
	for _, filter := range filters {
		if filter.Query.FilterQuery == nil {
			continue
		}
		userSavedQuery := Item{
			ID:     filter.FilterID,
			Title:  filter.FilterName,
			Stream: filter.StreamName,
			Desc:   *filter.Query.FilterQuery,
			From:   filter.TimeFilter.From,
			To:     filter.TimeFilter.To,
		}
		userSavedQueries = append(userSavedQueries, userSavedQuery)
	}
	return userSavedQueries, nil
}
