package publication

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/output"
)

type listOpts struct {
	typeName string
	source   string
	query    string
	page     int
	pageSize int
}

// NewCmdPublication exposes public USTC news and notices.
func NewCmdPublication() *cobra.Command {
	opts := listOpts{}
	cmd := &cobra.Command{
		Use:     "publication [command]",
		Aliases: []string{"publications"},
		Short:   "Browse public news and notices",
		Long:    "List and view public USTC news and notices.",
		Example: `  # List recent notices
  life-ustc catalog publication --type notice --limit 20

  # Search all publication types
  life-ustc catalog publication --query scholarship

  # View one publication
  life-ustc catalog publication get <publication-id>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}
	addListFlags(cmd, &opts)
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdGet())
	return cmd
}

func addListFlags(cmd *cobra.Command, opts *listOpts) {
	cmd.Flags().StringVar(&opts.typeName, "type", "", "Publication type (news or notice)")
	cmd.Flags().StringVar(&opts.source, "source", "", "Publication source identifier")
	cmd.Flags().StringVarP(&opts.query, "query", "q", "", "Search title and publication text")
	cmd.Flags().IntVarP(&opts.page, "page", "p", 0, "Page number")
	cmd.Flags().IntVarP(&opts.pageSize, "limit", "L", 0, "Number of items per page")
}

func newCmdList() *cobra.Command {
	opts := listOpts{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List publications",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}
	addListFlags(cmd, &opts)
	return cmd
}

func runList(cmd *cobra.Command, opts listOpts) error {
	if err := validateListBounds(opts); err != nil {
		return err
	}
	typeValue, err := normalizeType(opts.typeName)
	if err != nil {
		return err
	}
	client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	data, err := fetchList(client, opts, typeValue)
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "Title", Key: "revision.title"},
		{Header: "Type", Key: "publicationType"},
		{Header: "Published", Key: "revision.publishedAt"},
		{Header: "Source", Key: "source.name"},
		{Header: "ID", Key: "id"},
	}, list.Total, list.Page)
}

func fetchList(client *api.Client, opts listOpts, typeValue *string) (any, error) {
	if err := validateListBounds(opts); err != nil {
		return nil, err
	}
	params := url.Values{}
	if typeValue != nil {
		params.Set("type", string(*typeValue))
	}
	if value := strings.TrimSpace(opts.source); value != "" {
		params.Set("source", value)
	}
	if value := strings.TrimSpace(opts.query); value != "" {
		params.Set("query", value)
	}
	if opts.page > 0 {
		params.Set("page", strconv.Itoa(opts.page))
	}
	if opts.pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(opts.pageSize))
	}
	resp, err := client.DoRaw(api.Ctx(), http.MethodGet, "/api/publications", params, nil, "", nil)
	return api.ParseResponseRaw(resp, err)
}

func validateListBounds(opts listOpts) error {
	if opts.page < 0 {
		return fmt.Errorf("--page must be zero or greater")
	}
	if opts.pageSize < 0 {
		return fmt.Errorf("--limit must be zero or greater")
	}
	return nil
}

func normalizeType(value string) (*string, error) {
	if value == "" {
		return nil, nil
	}
	if value != "news" && value != "notice" {
		return nil, fmt.Errorf("--type must be news or notice")
	}
	typeValue := value
	return &typeValue, nil
}

func newCmdGet() *cobra.Command {
	return &cobra.Command{
		Use:     "get <publication-id>",
		Aliases: []string{"show"},
		Short:   "View a publication",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			id := strings.TrimSpace(args[0])
			if id == "" {
				return fmt.Errorf("publication ID must not be empty")
			}
			data, err := fetchGet(client, id)
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}
			m := cmdutil.AsMap(data)
			if err := output.OutputDetail(data, []output.FieldDef{
				{Key: "id", Label: "ID"},
				{Key: "publicationType", Label: "Type"},
				{Key: "revision.title", Label: "Title"},
				{Key: "revision.author", Label: "Author", SkipEmpty: true},
				{Key: "revision.category", Label: "Category", SkipEmpty: true},
				{Key: "revision.publishedAt", Label: "Published", SkipEmpty: true},
				{Key: "revision.updatedAtSource", Label: "Source updated", SkipEmpty: true},
				{Key: "revision.summary", Label: "Summary", SkipEmpty: true},
				{Key: "source.name", Label: "Source"},
				{Key: "source.organizationLevel", Label: "Organization"},
				{Key: "canonicalUrl", Label: "URL"},
				{Key: "revision.sourcePageUrl", Label: "Source page", SkipEmpty: true},
				{Key: "revision.bodyText", Label: "Body", SkipEmpty: true},
			}, "Publication"); err != nil {
				return err
			}
			if objects := cmdutil.RowsFromAny(cmdutil.AsMap(m["revision"])["objects"]); len(objects) > 0 {
				fmt.Println()
				output.Bold("  Objects")
				output.Table(objects, []output.Column{
					{Header: "Kind", Key: "kind"},
					{Header: "Type", Key: "contentType"},
					{Header: "Size", Key: "size"},
					{Header: "Status", Key: "status"},
					{Header: "URL", Key: "url"},
				})
			}
			return nil
		},
	}
}

func fetchGet(client *api.Client, id string) (any, error) {
	resp, err := client.DoRaw(api.Ctx(), http.MethodGet, "/api/publications/"+url.PathEscape(id), nil, nil, "", nil)
	return api.ParseResponseRaw(resp, err)
}
