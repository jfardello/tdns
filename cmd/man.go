package cmd

import (
	"bytes"
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
output directory, one page for tdns and one page for each top-level command are
written to that directory. Nested subcommands are included in their top-level
command's page. Existing pages are preserved unless --force is specified.`,
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
	command.Flags().StringVarP(&outputDir, "output-dir", "o", "", "write the consolidated manual pages to this directory")
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
	headerCopy := *header
	if err := doc.GenMan(root, &headerCopy, output); err != nil {
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

	commands := manPageCommands(root)
	for index, command := range commands {
		page, err := os.Create(pages[index])
		if err != nil {
			return fmt.Errorf("create manual page %s: %w", pages[index], err)
		}

		if index == 0 {
			err = renderRootManPage(command, header, page)
		} else {
			err = renderCommandFamilyManPage(command, header, page)
		}
		closeErr := page.Close()
		if err != nil {
			return fmt.Errorf("generate manual page %s: %w", pages[index], err)
		}
		if closeErr != nil {
			return fmt.Errorf("close manual page %s: %w", pages[index], closeErr)
		}
	}
	return nil
}

func manPagePaths(root *cobra.Command, outputDir string) ([]string, error) {
	seen := make(map[string]string)
	commands := manPageCommands(root)
	pages := make([]string, 0, len(commands))
	for _, command := range commands {
		name := strings.ReplaceAll(command.CommandPath(), " ", "-") + "." + manSection
		if previous, exists := seen[name]; exists {
			return nil, fmt.Errorf("commands %q and %q generate the same manual page %q", previous, command.CommandPath(), name)
		}
		seen[name] = command.CommandPath()
		pages = append(pages, filepath.Join(outputDir, name))
	}
	return pages, nil
}

func manPageCommands(root *cobra.Command) []*cobra.Command {
	commands := []*cobra.Command{root}
	for _, child := range root.Commands() {
		if child.IsAvailableCommand() && !child.IsAdditionalHelpTopicCommand() {
			commands = append(commands, child)
		}
	}
	return commands
}

func renderCommandFamilyManPage(command *cobra.Command, header *doc.GenManHeader, output io.Writer) error {
	var page bytes.Buffer
	headerCopy := *header
	if err := doc.GenMan(command, &headerCopy, &page); err != nil {
		return err
	}

	descendants := availableDescendants(command)
	if len(descendants) == 0 {
		_, err := output.Write(page.Bytes())
		return err
	}

	if _, err := io.WriteString(output, withoutSeeAlso(page.String())); err != nil {
		return err
	}
	if _, err := io.WriteString(output, ".SH SUBCOMMANDS\n"); err != nil {
		return err
	}

	for _, descendant := range descendants {
		page.Reset()
		headerCopy = *header
		if err := doc.GenMan(descendant, &headerCopy, &page); err != nil {
			return err
		}
		fragment := withoutManHeader(withoutSeeAlso(page.String()))
		fragment = strings.Replace(fragment, ".SH NAME\n", ".SS "+strings.ToUpper(strings.ReplaceAll(descendant.CommandPath(), " ", "\\-"))+"\n", 1)
		fragment = strings.ReplaceAll(fragment, ".SH ", ".SS ")
		if _, err := io.WriteString(output, fragment); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(output, ".SH SEE ALSO\n\\fB%s(%s)\\fP\n", strings.ReplaceAll(command.Root().CommandPath(), " ", "-"), manSection)
	return err
}

func availableDescendants(command *cobra.Command) []*cobra.Command {
	var descendants []*cobra.Command
	for _, child := range command.Commands() {
		if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		descendants = append(descendants, child)
		descendants = append(descendants, availableDescendants(child)...)
	}
	return descendants
}

func withoutManHeader(page string) string {
	if header := strings.Index(page, ".TH "); header >= 0 {
		if newline := strings.IndexByte(page[header:], '\n'); newline >= 0 {
			return page[header+newline+1:]
		}
	}
	return page
}

func withoutSeeAlso(page string) string {
	const seeAlso = ".SH SEE ALSO\n"
	if index := strings.Index(page, seeAlso); index >= 0 {
		return page[:index]
	}
	return page
}
