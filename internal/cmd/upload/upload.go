package upload

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdUpload() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload [command]",
		Short: "Manage file uploads",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUploadList(cmd)
		},
	}
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdFile())
	cmd.AddCommand(newCmdRename())
	cmd.AddCommand(newCmdDelete())
	cmd.AddCommand(newCmdDownload())
	return cmd
}

func runUploadList(cmd *cobra.Command) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	data, err := api.ParseResponseRaw(c.ListUploads(api.Ctx()))
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "uploads").FinalizeServerSide(0)

	if !output.IsJSON() {
		m := cmdutil.AsMap(data)
		if m != nil {
			used, _ := m["usedBytes"].(float64)
			quota, _ := m["quotaBytes"].(float64)
			if quota > 0 {
				output.Dim(fmt.Sprintf("  Usage: %s / %s", humanSize(int64(used)), humanSize(int64(quota))))
			}
		}
	}

	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "ID", Key: "id"},
		{Header: "Filename", Key: "filename"},
		{Header: "Type", Key: "contentType"},
		{Header: "Size", Key: "size"},
	}, list.Total, list.Page)
}

func newCmdList() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your uploads",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUploadList(cmd)
		},
	}
}

func newCmdFile() *cobra.Command {
	var contentType string
	cmd := &cobra.Command{
		Use:     "file <filepath>",
		Aliases: []string{"add"},
		Short:   "Upload a file",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			f, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			stat, err := f.Stat()
			if err != nil {
				return err
			}

			if contentType == "" {
				contentType = guessContentType(filePath)
			}

			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}

			// Step 1: Create upload
			size := fmt.Sprintf("%d", stat.Size())
			reqBody := openapi.CreateUploadJSONRequestBody{
				Filename: filepath.Base(filePath),
				Size:     size,
			}
			if contentType != "" {
				reqBody.ContentType = &contentType
			}
			createResp, err := api.ParseResponseRaw(c.CreateUpload(api.Ctx(), reqBody))
			if err != nil {
				return err
			}
			cm := cmdutil.AsMap(createResp)
			uploadURL, _ := cm["url"].(string)
			uploadKey, _ := cm["key"].(string)

			if uploadURL == "" {
				return fmt.Errorf("server did not return an upload URL")
			}

			// Step 2: PUT to S3
			req, err := http.NewRequest("PUT", uploadURL, f)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", contentType)
			req.ContentLength = stat.Size()

			httpClient := &http.Client{}
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			_ = resp.Body.Close()
			if resp.StatusCode >= 400 {
				return fmt.Errorf("S3 upload failed with status %d", resp.StatusCode)
			}

			// Step 3: Complete
			completeBody := openapi.CompleteUploadJSONRequestBody{
				Key:      uploadKey,
				Filename: filepath.Base(filePath),
			}
			if contentType != "" {
				completeBody.ContentType = &contentType
			}
			_, err = api.ParseResponseRaw(c.CompleteUpload(api.Ctx(), completeBody))
			if err != nil {
				return err
			}

			output.Success(fmt.Sprintf("Uploaded %s (key: %s)", filepath.Base(filePath), uploadKey))
			return nil
		},
	}
	cmd.Flags().StringVar(&contentType, "content-type", "", "Content type")
	return cmd
}

func newCmdRename() *cobra.Command {
	var filename string
	cmd := &cobra.Command{
		Use:     "rename <upload-id>",
		Aliases: []string{"mv"},
		Short:   "Rename an upload",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.RenameUpload(api.Ctx(), args[0], openapi.RenameUploadJSONRequestBody{Filename: filename}))
			if err != nil {
				return err
			}
			output.Success("Upload renamed.")
			return nil
		},
	}
	cmd.Flags().StringVar(&filename, "filename", "", "New filename (required)")
	_ = cmd.MarkFlagRequired("filename")
	return cmd
}

func newCmdDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete [upload-id]",
		Aliases: []string{"rm"},
		Short:   "Delete an upload",
		Long:    "Delete an upload. When run interactively without an ID, shows your uploads and lets you pick one.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			var row map[string]any
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("upload id is required in non-interactive mode")
				}
				picked, err := promptUploadPick(cmd, "Pick an upload to delete")
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
				row = picked
			} else {
				row = map[string]any{"id": id}
			}
			label := uploadLabelFromRow(row)
			if !cmdutil.Confirm(fmt.Sprintf("Delete %s?", label), yes) {
				return nil
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.DeleteUploadWithBody(api.Ctx(), id, "application/json", strings.NewReader("{}")))
			if err != nil {
				return err
			}
			output.Success("Upload deleted.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	return cmd
}

func newCmdDownload() *cobra.Command {
	var outFile string
	cmd := &cobra.Command{
		Use:     "download <upload-id>",
		Aliases: []string{"dl"},
		Short:   "Download a file",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			resp, err := c.DownloadUpload(api.Ctx(), args[0])
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= 400 {
				errBody, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("download failed: HTTP %d: %s", resp.StatusCode, string(errBody))
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if outFile != "" {
				if err := os.WriteFile(outFile, data, 0o644); err != nil {
					return err
				}
				output.Success(fmt.Sprintf("Saved to %s", outFile))
			} else {
				_, _ = os.Stdout.Write(data)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "Save to file")
	return cmd
}

// promptUploadPick loads the user's uploads and lets them pick one.
func promptUploadPick(cmd *cobra.Command, prompt string) (map[string]any, error) {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return nil, err
	}
	data, err := api.ParseResponseRaw(c.ListUploads(api.Ctx()))
	if err != nil {
		return nil, err
	}
	_, rows, _, _ := cmdutil.ExtractList(data, "uploads")
	if len(rows) == 0 {
		output.Dim("  No uploads found.")
		return nil, nil
	}
	return cmdutil.PromptPick(rows, []output.Column{
		{Header: "Filename", Key: "filename"},
		{Header: "Size", Key: "size"},
		{Header: "ID", Key: "id"},
	}, "id", prompt)
}

// uploadLabelFromRow returns a short summary for confirm messages.
func uploadLabelFromRow(row map[string]any) string {
	if row == nil {
		return "this upload"
	}
	filename, _ := row["filename"].(string)
	if filename != "" {
		return filename
	}
	id, _ := row["id"].(string)
	if id != "" {
		return "upload " + id
	}
	return "this upload"
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func guessContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}
