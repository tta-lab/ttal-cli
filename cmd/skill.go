package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/skill"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the skill registry",
	Long:  `List, get, find, add, remove, and migrate skills in the runtime-agnostic skill registry.`,
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available skills",
	Long: `List skills from the registry.

If TTAL_AGENT_NAME is set, lists only skills in that agent's allow-list.
Use --all to list all skills regardless of agent.

Example:
  ttal skill list
  ttal skill list --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		listAll, _ := cmd.Flags().GetBool("all")
		return runSkillList(listAll)
	},
}

var skillGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Print skill content",
	Long: `Print the content of a skill by name via flicknote.

Example:
  ttal skill get breathe
  ttal skill get sp-debugging`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillGet(args[0])
	},
}

var skillFindCmd = &cobra.Command{
	Use:   "find <keyword> [keyword...]",
	Short: "Search skills by keyword",
	Long: `Search skills by keyword (OR match) against name, description, and content.

Example:
  ttal skill find debug
  ttal skill find git commit`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		findAll, _ := cmd.Flags().GetBool("all")
		return runSkillFind(args, findAll)
	},
}

var skillAddCmd = &cobra.Command{
	Use:   "add <name> [flicknote-id]",
	Short: "Register a skill",
	Long: `Register a skill in the registry.

Mode 1: Register an existing flicknote note by ID.
  ttal skill add breathe a1b2c3d4 --category command --description "..."

Mode 2: Upload a file to flicknote and register it.
  ttal skill add breathe --file skills/breathe/SKILL.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileFlag, _ := cmd.Flags().GetString("file")
		category, _ := cmd.Flags().GetString("category")
		description, _ := cmd.Flags().GetString("description")
		force, _ := cmd.Flags().GetBool("force")

		if fileFlag != "" && len(args) > 1 {
			return fmt.Errorf("cannot use --file with a flicknote ID argument")
		}
		if fileFlag == "" && len(args) < 2 {
			return fmt.Errorf("provide a flicknote ID or use --file <path>")
		}

		name := args[0]
		if fileFlag != "" {
			return runSkillAddFile(name, fileFlag, category, description, force)
		}
		return runSkillAddID(name, args[1], category, description, force)
	},
}

var skillRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister a skill",
	Long: `Remove a skill from the registry and all agent allow-lists.
The flicknote note is NOT deleted — manage it manually if needed.

Example:
  ttal skill remove breathe`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillRemove(args[0])
	},
}

var skillImportCmd = &cobra.Command{
	Use:   "import <folder>",
	Short: "Import skill files from a folder into flicknote",
	Long: `Scan a folder for skill files and upsert them into flicknote.

Supports two layouts:
  1. <name>/SKILL.md  — directory-based skills
  2. <name>.md        — flat file skills

Skill name is derived from directory name or filename (without .md).
Frontmatter name/description override derived values.
Category is auto-detected: sp-* dirs → methodology, other dirs → tool,
flat .md files → reference. Use --category to override.

Existing notes are updated in-place; new notes are uploaded.
Dry-run by default. Use --apply to actually upload and register.

Example:
  ttal skill import skills
  ttal skill import skills --apply
  ttal skill import commands --apply --category command`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apply, _ := cmd.Flags().GetBool("apply")
		category, _ := cmd.Flags().GetString("category")
		return runSkillImport(args[0], apply, category)
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillGetCmd)
	skillCmd.AddCommand(skillFindCmd)
	skillCmd.AddCommand(skillAddCmd)
	skillCmd.AddCommand(skillRemoveCmd)
	skillCmd.AddCommand(skillImportCmd)

	skillListCmd.Flags().Bool("all", false, "List all skills (ignore agent filter)")
	skillFindCmd.Flags().Bool("all", false, "Search all skills (ignore agent filter)")
	skillAddCmd.Flags().String("file", "", "Path to file to upload to flicknote")
	skillAddCmd.Flags().String("category", "", "Skill category (command, methodology, reference, tool)")
	skillAddCmd.Flags().String("description", "", "Skill description")
	skillAddCmd.Flags().Bool("force", false, "Overwrite existing registration")
	skillImportCmd.Flags().Bool("apply", false, "Actually upload and register (default is dry-run)")
	skillImportCmd.Flags().String("category", "", "Override auto-detected category (command, methodology, reference, tool)") //nolint:lll
}

func loadRegistry() (*skill.Registry, error) {
	return skill.Load(skill.DefaultPath())
}

// buildSkillTable returns a lipgloss table with skill styling.
// dimCols lists column indices that should receive dim styling.
func buildSkillTable(headers []string, rows [][]string, dimCols ...int) *table.Table {
	dimColSet := make(map[int]bool, len(dimCols))
	for _, c := range dimCols {
		dimColSet[c] = true
	}
	dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if dimColSet[col] {
				return dimStyle
			}
			return cellStyle
		}).
		Headers(headers...).
		Rows(rows...)
}

func runSkillList(listAll bool) error {
	r, err := loadRegistry()
	if err != nil {
		return err
	}

	var skills []skill.Skill
	agentName := os.Getenv("TTAL_AGENT_NAME")
	if listAll || agentName == "" {
		skills = r.List()
	} else {
		skills = r.ListForAgent(agentName)
	}

	if len(skills) == 0 {
		fmt.Println("No skills registered.")
		return nil
	}

	showCategory := listAll || agentName == ""

	var rows [][]string
	if showCategory {
		for _, s := range skills {
			rows = append(rows, []string{s.Name, s.Category, s.Description})
		}
		lipgloss.Println(buildSkillTable([]string{"Name", "Category", "Description"}, rows, 1))
	} else {
		for _, s := range skills {
			rows = append(rows, []string{s.Name, s.Description})
		}
		lipgloss.Println(buildSkillTable([]string{"Name", "Description"}, rows))
	}
	return nil
}

func runSkillGet(name string) error {
	r, err := loadRegistry()
	if err != nil {
		return err
	}

	s, err := r.Get(name)
	if err != nil {
		return err
	}

	cmd := exec.Command("flicknote", "content", s.FlicknoteID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// flicknoteSearchResult is a partial parse of flicknote find --json output.
type flicknoteSearchResult struct {
	ID string `json:"id"`
}

type matchSource string

const (
	matchName    matchSource = "name"
	matchContent matchSource = "content"
	matchBoth    matchSource = "name+content"
)

type findResult struct {
	s      skill.Skill
	source matchSource
}

// findByNameDesc returns skills whose name or description matches any keyword.
func findByNameDesc(candidates []skill.Skill, keywords []string) map[string]*findResult {
	results := make(map[string]*findResult)
	for _, s := range candidates {
		for _, kw := range keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(strings.ToLower(s.Name), kwLower) ||
				strings.Contains(strings.ToLower(s.Description), kwLower) {
				sc := s
				results[s.Name] = &findResult{s: sc, source: matchName}
				break
			}
		}
	}
	return results
}

// mergeContentMatches adds content matches from flicknote into results.
func mergeContentMatches(r *skill.Registry, keywords []string, candidateSet map[string]bool, results map[string]*findResult) { //nolint:lll
	flickArgs := append([]string{"find", "--project", "ttal.skills", "--json"}, keywords...)
	out, err := exec.Command("flicknote", flickArgs...).Output()
	if err != nil || len(out) == 0 {
		return
	}
	var found []flicknoteSearchResult
	if err := json.Unmarshal(out, &found); err != nil {
		return
	}
	for _, f := range found {
		s, ok := r.ReverseLookup(f.ID)
		if !ok || !candidateSet[s.Name] {
			continue
		}
		if existing, alreadyFound := results[s.Name]; alreadyFound {
			existing.source = matchBoth
		} else {
			sc := *s
			results[s.Name] = &findResult{s: sc, source: matchContent}
		}
	}
}

func runSkillFind(keywords []string, findAll bool) error {
	r, err := loadRegistry()
	if err != nil {
		return err
	}

	agentName := os.Getenv("TTAL_AGENT_NAME")
	var candidates []skill.Skill
	if findAll || agentName == "" {
		candidates = r.List()
	} else {
		candidates = r.ListForAgent(agentName)
	}

	candidateSet := make(map[string]bool, len(candidates))
	for _, s := range candidates {
		candidateSet[s.Name] = true
	}

	results := findByNameDesc(candidates, keywords)
	mergeContentMatches(r, keywords, candidateSet, results)

	if len(results) == 0 {
		fmt.Println("No skills found.")
		return nil
	}

	sorted := make([]*findResult, 0, len(results))
	for _, r := range results {
		sorted = append(sorted, r)
	}
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].s.Name < sorted[j-1].s.Name; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	var rows [][]string
	for _, res := range sorted {
		rows = append(rows, []string{res.s.Name, res.s.Category, string(res.source), res.s.Description})
	}

	lipgloss.Println(buildSkillTable([]string{"Name", "Category", "Match", "Description"}, rows, 1, 2))
	return nil
}

func runSkillAddID(name, flicknoteID, category, description string, force bool) error {
	// Validate that the flicknote ID exists
	if err := exec.Command("flicknote", "detail", "--json", flicknoteID).Run(); err != nil {
		return fmt.Errorf("flicknote ID %q not found or inaccessible: %w", flicknoteID, err)
	}

	// Auto-populate description from frontmatter if not provided
	if description == "" {
		out, err := exec.Command("flicknote", "content", flicknoteID).Output()
		if err == nil {
			_, desc, _ := skill.ParseFrontmatter(out)
			description = desc
		}
	}

	r, err := loadRegistry()
	if err != nil {
		return err
	}

	if err := r.Add(skill.Skill{
		Name:        name,
		FlicknoteID: flicknoteID,
		Category:    category,
		Description: description,
	}, force); err != nil {
		return err
	}

	fmt.Printf("Added skill %q → flicknote %s\n", name, flicknoteID)
	return nil
}

var flicknoteIDRegexp = regexp.MustCompile(`Created note ([0-9a-f]+)`)

func runSkillAddFile(name, filePath, category, description string, force bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", filePath, err)
	}

	fmName, fmDesc, body := skill.ParseFrontmatter(content)

	if description == "" {
		description = fmDesc
	}
	// Frontmatter name takes precedence over the CLI argument: it's the file's canonical identity.
	// e.g. `ttal skill add my-alias --file foo.md` registers as the frontmatter name if present.
	if fmName != "" {
		name = fmName
	}

	// Upload body (frontmatter stripped) to flicknote
	cmd := exec.Command("flicknote", "add", "--project", "ttal.skills")
	cmd.Stdin = bytes.NewReader(body)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("uploading to flicknote: %w", err)
	}

	matches := flicknoteIDRegexp.FindSubmatch(output)
	if len(matches) < 2 {
		return fmt.Errorf("could not parse flicknote ID from output: %s", string(output))
	}
	flicknoteID := string(matches[1])

	r, err := loadRegistry()
	if err != nil {
		return err
	}

	if err := r.Add(skill.Skill{
		Name:        name,
		FlicknoteID: flicknoteID,
		Category:    category,
		Description: description,
	}, force); err != nil {
		return err
	}

	fmt.Printf("Uploaded to flicknote %s, added skill %q\n", flicknoteID, name)
	return nil
}

func runSkillRemove(name string) error {
	r, err := loadRegistry()
	if err != nil {
		return err
	}

	removed, agents, err := r.Remove(name)
	if err != nil {
		return err
	}

	agentMsg := "no agent allow-lists affected"
	if len(agents) > 0 {
		agentMsg = fmt.Sprintf("removed from agent allow-lists: %s", strings.Join(agents, ", "))
	}
	fmt.Printf("Removed skill %q (was flicknote %s). %s. (note NOT archived — manage manually in flicknote)\n",
		name, removed.FlicknoteID, agentMsg)
	return nil
}

type importEntry struct {
	name     string
	filePath string
	category string
	status   string
	id       string
}

// scanFolder scans dir for skill files. Returns entries with auto-detected categories.
// categoryOverride, if non-empty, overrides auto-detection for all entries.
func scanFolder(dir, categoryOverride string) ([]importEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("folder not found: %s", dir)
		}
		return nil, fmt.Errorf("reading dir %s: %w", dir, err)
	}

	var result []importEntry
	for _, entry := range entries {
		if entry.IsDir() {
			skillMD := filepath.Join(dir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillMD); err != nil {
				continue
			}
			cat := categoryOverride
			if cat == "" {
				cat = "tool"
				if strings.HasPrefix(entry.Name(), "sp-") {
					cat = "methodology"
				}
			}
			result = append(result, importEntry{
				name: entry.Name(), filePath: skillMD, category: cat,
			})
		} else if strings.HasSuffix(entry.Name(), ".md") {
			cat := categoryOverride
			if cat == "" {
				cat = "reference"
			}
			result = append(result, importEntry{
				name:     strings.TrimSuffix(entry.Name(), ".md"),
				filePath: filepath.Join(dir, entry.Name()),
				category: cat,
			})
		}
	}
	return result, nil
}

// uploadAndRegister uploads one entry to flicknote and registers it, setting e.status and e.id.
// If apply is false, it only reports what would be done.
// If an entry already exists, it updates in-place using flicknote modify; otherwise it creates a new note.
func uploadAndRegister(r *skill.Registry, e *importEntry, apply bool) {
	existing, _ := r.Get(e.name) // not-found is expected; only real registry errors would matter here

	if !apply {
		if existing != nil {
			e.status = "would update"
		} else {
			e.status = "would upload"
		}
		return
	}

	content, err := os.ReadFile(e.filePath)
	if err != nil {
		e.status = fmt.Sprintf("error: %v", err)
		return
	}

	fmName, fmDesc, body := skill.ParseFrontmatter(content)
	if fmName != "" {
		e.name = fmName
	}

	var flicknoteID string
	if existing != nil {
		// Update existing note in-place
		cmd := exec.Command("flicknote", "modify", existing.FlicknoteID)
		cmd.Stdin = bytes.NewReader(body)
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := "update error: %s (if note was deleted, run: ttal skill remove %s && reimport)"
			e.status = fmt.Sprintf(msg, strings.TrimSpace(string(out)), e.name)
			return
		}
		flicknoteID = existing.FlicknoteID
		e.status = "updated"
		e.id = flicknoteID
	} else {
		// Upload new note
		cmd := exec.Command("flicknote", "add", "--project", "ttal.skills")
		cmd.Stdin = bytes.NewReader(body)
		output, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				e.status = fmt.Sprintf("upload error: %s", strings.TrimSpace(string(exitErr.Stderr)))
			} else {
				e.status = fmt.Sprintf("upload error: %v", err)
			}
			return
		}

		matches := flicknoteIDRegexp.FindSubmatch(output)
		if len(matches) < 2 {
			e.status = fmt.Sprintf("parse error: %s", strings.TrimSpace(string(output)))
			return
		}
		flicknoteID = string(matches[1])
		e.status = "uploaded"
		e.id = flicknoteID
	}

	s := skill.Skill{Name: e.name, FlicknoteID: flicknoteID, Category: e.category, Description: fmDesc}
	force := existing != nil // pass true for force when updating existing
	if err := r.Add(s, force); err != nil {
		e.status = fmt.Sprintf("register error: %v", err)
		return
	}
}

func runSkillImport(folder string, apply bool, category string) error {
	dir := config.ExpandHome(folder)

	entries, err := scanFolder(dir, category)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Printf("No skill files found in %s\n", dir)
		return nil
	}

	r, err := loadRegistry()
	if err != nil {
		return err
	}

	for i := range entries {
		uploadAndRegister(r, &entries[i], apply)
	}

	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{e.name, e.category, e.id, e.status})
	}
	lipgloss.Println(buildSkillTable([]string{"Name", "Category", "Flicknote ID", "Status"}, rows, 1, 2))

	if !apply {
		fmt.Println("\nDry run — use --apply to upload and register.")
	}
	return nil
}
