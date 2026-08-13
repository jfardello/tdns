package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const manSection = "1"

func init() {
	rootCmd.AddCommand(newManCommand(rootCmd))
}

func newManCommand(root *cobra.Command) *cobra.Command {
	var outputDir string
	var force bool

	command := &cobra.Command{
		Use:   "man",
		Short: "Generate manual pages from the command help",
		Long: `Generate manual pages from the TDNS Cobra command tree.

Without --output-dir, the tdns(1) page is written to standard output. With an
output directory, pages for tdns and all its available subcommands are written
to that directory. Existing pages are preserved unless --force is specified.`,
		Args: cobra.NoArgs,
		// Manual generation must not initialize logging, configuration, or a
		// remote administration client through an inherited persistent hook.
		PersistentPreRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			header := manHeader()
			if outputDir == "" {
				return renderRootManPage(root, header, cmd.OutOrStdout())
			}
			return generateManTree(root, header, outputDir, force)
		},
	}
	command.Flags().StringVarP(&outputDir, "output-dir", "o", "", "write the complete manual tree to this directory")
	command.Flags().BoolVar(&force, "force", false, "overwrite existing manual pages in the output directory")

	return command
}

func manHeader() *doc.GenManHeader {
	return &doc.GenManHeader{
		Section: manSection,
		Source:  "TDNS " + valueOrDefault(ver, "dev"),
		Manual:  "TDNS Manual",
	}
}

func renderRootManPage(root *cobra.Command, header *doc.GenManHeader, output io.Writer) error {
	if err := doc.GenMan(root, header, output); err != nil {
		return fmt.Errorf("generate tdns manual page: %w", err)
	}
	return nil
}

func generateManTree(root *cobra.Command, header *doc.GenManHeader, outputDir string, force bool) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create manual output directory: %w", err)
	}

	pages, err := manPagePaths(root, outputDir)
	if err != nil {
		return err
	}
	if !force {
		for _, page := range pages {
			if _, err := os.Stat(page); err == nil {
				return fmt.Errorf("manual page %s already exists; use --force to overwrite it", page)
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("inspect manual page %s: %w", page, err)
			}
		}
	}

	if err := doc.GenManTree(root, header, outputDir); err != nil {
		return fmt.Errorf("generate manual page tree: %w", err)
	}
	return nil
}

func manPagePaths(root *cobra.Command, outputDir string) ([]string, error) {
	seen := make(map[string]string)
	pages := make([]string, 0)

	var visit func(*cobra.Command) error
	visit = func(command *cobra.Command) error {
		name := strings.ReplaceAll(command.CommandPath(), " ", "-") + "." + manSection
		if previous, exists := seen[name]; exists {
			return fmt.Errorf("commands %q and %q generate the same manual page %q", previous, command.CommandPath(), name)
		}
		seen[name] = command.CommandPath()
		pages = append(pages, filepath.Join(outputDir, name))

		for _, child := range command.Commands() {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}

	if err := visit(root); err != nil {
		return nil, err
	}
	return pages, nil
}
