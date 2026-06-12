package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"neoray/internal/config"
)

var typeGlobMap = map[string][]string{
	"py":       {"*.py", "*.pyi"},
	"python":   {"*.py", "*.pyi"},
	"js":       {"*.js", "*.jsx", "*.mjs", "*.cjs"},
	"ts":       {"*.ts", "*.tsx", "*.mts", "*.cts"},
	"tsx":      {"*.tsx"},
	"jsx":      {"*.jsx"},
	"json":     {"*.json"},
	"md":       {"*.md", "*.mdx"},
	"markdown": {"*.md", "*.mdx"},
	"go":       {"*.go"},
	"rs":       {"*.rs"},
	"rust":     {"*.rs"},
	"java":     {"*.java"},
	"sh":       {"*.sh", "*.bash"},
	"yaml":     {"*.yaml", "*.yml"},
	"yml":      {"*.yaml", "*.yml"},
	"toml":     {"*.toml"},
	"sql":      {"*.sql"},
	"html":     {"*.html", "*.htm"},
	"css":      {"*.css", "*.scss", "*.sass"},
}

var ignoreDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	"__pycache__":  true,
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"build":        true,
	"dist":         true,
	".next":        true,
	".nuxt":        true,
	".cache":       true,
}

const (
	defaultHeadLimit     = 250
	defaultFileHeadLimit = 200
	maxResultChars       = 128000
	maxFileBytes         = 2000000
)

// ======================================
// FindFilesTool
// ======================================

type FindFilesTool struct {
	workspace string
}

type FindFilesArgs struct {
	Path         string `json:"path"`
	Query        string `json:"query"`
	Glob         string `json:"glob"`
	Type         string `json:"type"`
	IncludeDirs  bool   `json:"include_dirs"`
	Sort         string `json:"sort"`
	HeadLimit    int    `json:"head_limit"`
	Offset       int    `json:"offset"`
}

func NewFindFilesTool() *FindFilesTool {
	return &FindFilesTool{
		workspace: config.GetWorkspace(),
	}
}

func (t *FindFilesTool) Name() string {
	return "find_files"
}

func (t *FindFilesTool) Description() string {
	return "Find files by path fragment, glob, or file type. Use this before read_file when you need to locate files, and prefer it over shell find/ls for ordinary workspace discovery. Returns workspace-relative paths and skips common dependency/build directories."
}

func (t *FindFilesTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{
		"path": StringParam("Directory or file to search in (default '.')"),
		"query": StringParam("Optional case-insensitive path fragment search. Whitespace-separated terms must all be present."),
		"glob": StringParam("Optional file filter, e.g. '*.py' or 'tests/**/test_*.py'"),
		"type": StringParam("Optional file type shorthand, e.g. 'py', 'ts', 'md', 'json'"),
		"include_dirs": BooleanParam("Include matching directories as well as files (default false)"),
		"sort": map[string]any{
			"type": "string",
			"enum": []string{"path", "modified"},
			"description": "Sort by path or most recently modified first (default path)",
		},
		"head_limit": map[string]any{
			"type": "integer",
			"description": "Maximum number of paths to return (default 200, 0 for all, max 1000)",
			"minimum": 0,
			"maximum": 1000,
		},
		"offset": map[string]any{
			"type": "integer",
			"description": "Skip the first N results before applying head_limit",
			"minimum": 0,
			"maximum": 100000,
		},
	}, nil)
}

func (t *FindFilesTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input FindFilesArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	path := input.Path
	if path == "" {
		path = "."
	}

	target := t.resolvePath(path)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		result := fmt.Sprintf("Error: Path not found: %s", path)
		return json.Marshal(result)
	}

	sortMode := input.Sort
	if sortMode != "path" && sortMode != "modified" {
		sortMode = "path"
	}

	limit := defaultFileHeadLimit
	if input.HeadLimit > 0 {
		limit = input.HeadLimit
	} else if input.HeadLimit == 0 {
		limit = 0 // 0 means no limit
	}

	root := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		root = filepath.Dir(target)
	}

	type matchEntry struct {
		path string
		mtime float64
	}
	var matches []matchEntry

	err := filepath.Walk(target, func(candidate string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if ignoreDirs[name] && candidate != target {
				return filepath.SkipDir
			}
			if !input.IncludeDirs && candidate != target {
				return nil
			}
		} else {
			// Check file type
			if input.Type != "" {
				if !matchesType(info.Name(), input.Type) {
					return nil
				}
			}
		}

		// Calculate relative path
		displayPath, err := filepath.Rel(t.workspace, candidate)
		if err != nil {
			displayPath = candidate
		}
		displayPath = filepath.ToSlash(displayPath)

		if info.IsDir() {
			displayPath += "/"
		}

		// Check glob
		if input.Glob != "" {
			relToRoot, err := filepath.Rel(root, candidate)
			if err != nil {
				relToRoot = candidate
			}
			relToRoot = filepath.ToSlash(relToRoot)
			if !matchGlob(relToRoot, info.Name(), input.Glob) {
				return nil
			}
		}

		// Check query
		if input.Query != "" && !matchesQuery(displayPath, input.Query) {
			return nil
		}

		// Get mtime
		mtime := float64(0)
		if stat, err := os.Stat(candidate); err == nil {
			mtime = float64(stat.ModTime().Unix())
		}

		matches = append(matches, matchEntry{path: displayPath, mtime: mtime})
		return nil
	})

	if err != nil {
		result := fmt.Sprintf("Error finding files: %v", err)
		return json.Marshal(result)
	}

	// Sort
	if sortMode == "modified" {
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].mtime != matches[j].mtime {
				return matches[i].mtime > matches[j].mtime
			}
			return matches[i].path < matches[j].path
		})
	} else {
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].path < matches[j].path
		})
	}

	// Paginate
	start := input.Offset
	if start < 0 {
		start = 0
	}
	end := len(matches)
	if limit > 0 && start+limit < len(matches) {
		end = start + limit
	}
	truncated := end < len(matches)

	if start >= len(matches) {
		return json.Marshal("No files found")
	}

	var resultLines []string
	for i := start; i < end; i++ {
		resultLines = append(resultLines, matches[i].path)
	}

	output := strings.Join(resultLines, "\n")

	// Add pagination notes
	if truncated || start > 0 {
		var note string
		if truncated {
			if limit > 0 {
				note = fmt.Sprintf("\n\n(pagination: limit=%d, offset=%d)", limit, start)
			} else {
				note = fmt.Sprintf("\n\n(pagination: offset=%d)", start)
			}
		} else if start > 0 {
			note = fmt.Sprintf("\n\n(pagination: offset=%d)", start)
		}
		if note != "" {
			output += note
		}
	}

	return json.Marshal(output)
}

func (t *FindFilesTool) resolvePath(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(t.workspace, path)
	}
	return path
}

// ======================================
// GrepTool
// ======================================

type GrepTool struct {
	workspace string
}

type GrepArgs struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	Glob            string `json:"glob"`
	Type            string `json:"type"`
	CaseInsensitive bool   `json:"case_insensitive"`
	FixedStrings    bool   `json:"fixed_strings"`
	OutputMode      string `json:"output_mode"`
	ContextBefore   int    `json:"context_before"`
	ContextAfter    int    `json:"context_after"`
	MaxMatches      int    `json:"max_matches"`
	MaxResults      int    `json:"max_results"`
	HeadLimit       int    `json:"head_limit"`
	Offset          int    `json:"offset"`
}

func NewGrepTool() *GrepTool {
	return &GrepTool{
		workspace: config.GetWorkspace(),
	}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search file contents with a regex pattern. Default output_mode is files_with_matches (file paths only); use content mode for matching lines with context. Prefer this over shell grep for ordinary workspace searches. Skips binary and files >2 MB. Supports glob/type filtering."
}

func (t *GrepTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{
		"pattern": map[string]any{
			"type": "string",
			"description": "Regex or plain text pattern to search for",
			"minLength": 1,
		},
		"path": StringParam("File or directory to search in (default '.')"),
		"glob": StringParam("Optional file filter, e.g. '*.py' or 'tests/**/test_*.py'"),
		"type": StringParam("Optional file type shorthand, e.g. 'py', 'ts', 'md', 'json'"),
		"case_insensitive": BooleanParam("Case-insensitive search (default false)"),
		"fixed_strings": BooleanParam("Treat pattern as plain text instead of regex (default false)"),
		"output_mode": map[string]any{
			"type": "string",
			"enum": []string{"content", "files_with_matches", "count"},
			"description": "content: matching lines with optional context; files_with_matches: only matching file paths; count: matching line counts per file. Default: files_with_matches",
		},
		"context_before": map[string]any{
			"type": "integer",
			"description": "Number of lines of context before each match",
			"minimum": 0,
			"maximum": 20,
		},
		"context_after": map[string]any{
			"type": "integer",
			"description": "Number of lines of context after each match",
			"minimum": 0,
			"maximum": 20,
		},
		"head_limit": map[string]any{
			"type": "integer",
			"description": "Maximum number of results to return",
			"minimum": 0,
			"maximum": 1000,
		},
		"offset": map[string]any{
			"type": "integer",
			"description": "Skip the first N results before applying head_limit",
			"minimum": 0,
			"maximum": 100000,
		},
	}, []string{"pattern"})
}

func (t *GrepTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input GrepArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if input.Pattern == "" {
		return json.Marshal("Error: pattern is required")
	}

	path := input.Path
	if path == "" {
		path = "."
	}

	target := t.resolvePath(path)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return json.Marshal(fmt.Sprintf("Error: Path not found: %s", path))
	}

	// Compile regex
	pattern := input.Pattern
	if input.FixedStrings {
		pattern = regexp.QuoteMeta(pattern)
	}
	regexOpts := ""
	if input.CaseInsensitive {
		regexOpts = "(?i)"
	}
	re, err := regexp.Compile(regexOpts + pattern)
	if err != nil {
		return json.Marshal(fmt.Sprintf("Error: invalid regex pattern: %v", err))
	}

	// Determine limit
	limit := defaultHeadLimit
	if input.HeadLimit > 0 {
		limit = input.HeadLimit
	} else if input.HeadLimit == 0 {
		limit = 0
	} else if input.OutputMode == "content" && input.MaxMatches > 0 {
		limit = input.MaxMatches
	} else if input.OutputMode != "content" && input.MaxResults > 0 {
		limit = input.MaxResults
	}

	outputMode := input.OutputMode
	if outputMode != "content" && outputMode != "files_with_matches" && outputMode != "count" {
		outputMode = "files_with_matches"
	}

	contextBefore := input.ContextBefore
	if contextBefore < 0 { contextBefore = 0 }
	if contextBefore > 20 { contextBefore = 20 }
	contextAfter := input.ContextAfter
	if contextAfter < 0 { contextAfter = 0 }
	if contextAfter > 20 { contextAfter = 20 }

	var blocks []string
	resultChars := 0
	seenContentMatches := 0
	truncated := false
	sizeTruncated := false
	skippedBinary := 0
	skippedLarge := 0
	matchingFiles := make(map[string]bool)
	counts := make(map[string]int)
	fileMtimes := make(map[string]float64)

	root := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		root = filepath.Dir(target)
	}

	err = filepath.Walk(target, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if ignoreDirs[name] && filePath != target {
				return filepath.SkipDir
			}
			return nil
		}

		// Check glob
		if input.Glob != "" {
			relToRoot, err := filepath.Rel(root, filePath)
			if err != nil {
				relToRoot = filePath
			}
			relToRoot = filepath.ToSlash(relToRoot)
			if !matchGlob(relToRoot, info.Name(), input.Glob) {
				return nil
			}
		}

		// Check type
		if input.Type != "" && !matchesType(info.Name(), input.Type) {
			return nil
		}

		// Check file size
		if info.Size() > maxFileBytes {
			skippedLarge++
			return nil
		}

		// Read file
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		// Check if binary
		if isBinary(raw) {
			skippedBinary++
			return nil
		}

		// Check UTF-8
		if !utf8.Valid(raw) {
			skippedBinary++
			return nil
		}

		// Get mtime
		mtime := float64(info.ModTime().Unix())

		// Search
		lines := strings.Split(string(raw), "\n")
		displayPath, err := filepath.Rel(t.workspace, filePath)
		if err != nil {
			displayPath = filePath
		}
		displayPath = filepath.ToSlash(displayPath)

		fileHadMatch := false
		for idx, line := range lines {
			lineNum := idx + 1
			if !re.MatchString(line) {
				continue
			}
			fileHadMatch = true

			if outputMode == "count" {
				counts[displayPath]++
				continue
			}
			if outputMode == "files_with_matches" {
				if !matchingFiles[displayPath] {
					matchingFiles[displayPath] = true
					fileMtimes[displayPath] = mtime
				}
				break
			}

			// Content mode
			seenContentMatches++
			if seenContentMatches <= input.Offset {
				continue
			}
			if limit > 0 && len(blocks) >= limit {
				truncated = true
				break
			}

			block := formatBlock(displayPath, lines, lineNum, contextBefore, contextAfter)
			extraSep := 0
			if len(blocks) > 0 {
				extraSep = 2
			}
			if resultChars+extraSep+len(block) > maxResultChars {
				sizeTruncated = true
				break
			}

			blocks = append(blocks, block)
			resultChars += extraSep + len(block)
		}

		if outputMode == "count" && fileHadMatch {
			if !matchingFiles[displayPath] {
				matchingFiles[displayPath] = true
				fileMtimes[displayPath] = mtime
			}
		}

		if truncated || sizeTruncated {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return json.Marshal(fmt.Sprintf("Error searching files: %v", err))
	}

	var output string
	var notes []string

	if outputMode == "files_with_matches" {
		if len(matchingFiles) == 0 {
			output = fmt.Sprintf("No matches found for pattern '%s' in %s", input.Pattern, path)
		} else {
			// Convert to list and sort
			files := make([]string, 0, len(matchingFiles))
			for f := range matchingFiles {
				files = append(files, f)
			}
			sort.Slice(files, func(i, j int) bool {
				mt1 := fileMtimes[files[i]]
				mt2 := fileMtimes[files[j]]
				if mt1 != mt2 {
					return mt1 > mt2
				}
				return files[i] < files[j]
			})
			// Paginate
			paged, isTruncated := paginate(files, limit, input.Offset)
			truncated = isTruncated
			output = strings.Join(paged, "\n")
		}
	} else if outputMode == "count" {
		if len(counts) == 0 {
			output = fmt.Sprintf("No matches found for pattern '%s' in %s", input.Pattern, path)
		} else {
			// Get file list and sort
			files := make([]string, 0, len(matchingFiles))
			for f := range matchingFiles {
				files = append(files, f)
			}
			sort.Slice(files, func(i, j int) bool {
				mt1 := fileMtimes[files[i]]
				mt2 := fileMtimes[files[j]]
				if mt1 != mt2 {
					return mt1 > mt2
				}
				return files[i] < files[j]
			})
			// Paginate
			paged, isTruncated := paginate(files, limit, input.Offset)
			truncated = isTruncated
			var countLines []string
			for _, name := range paged {
				countLines = append(countLines, fmt.Sprintf("%s: %d", name, counts[name]))
			}
			output = strings.Join(countLines, "\n")
			total := 0
			for _, c := range counts {
				total += c
			}
			notes = append(notes, fmt.Sprintf("(total matches: %d in %d files)", total, len(counts)))
		}
	} else {
		if len(blocks) == 0 {
			output = fmt.Sprintf("No matches found for pattern '%s' in %s", input.Pattern, path)
		} else {
			output = strings.Join(blocks, "\n\n")
		}
	}

	// Add notes
	if outputMode == "content" && truncated {
		notes = append(notes, fmt.Sprintf("(pagination: limit=%d, offset=%d)", limit, input.Offset))
	} else if outputMode == "content" && sizeTruncated {
		notes = append(notes, "(output truncated due to size)")
	} else if truncated && (outputMode == "count" || outputMode == "files_with_matches") {
		notes = append(notes, fmt.Sprintf("(pagination: limit=%d, offset=%d)", limit, input.Offset))
	} else if (outputMode == "count" || outputMode == "files_with_matches") && input.Offset > 0 {
		notes = append(notes, fmt.Sprintf("(pagination: offset=%d)", input.Offset))
	} else if outputMode == "content" && input.Offset > 0 && len(blocks) > 0 {
		notes = append(notes, fmt.Sprintf("(pagination: offset=%d)", input.Offset))
	}

	if skippedBinary > 0 {
		notes = append(notes, fmt.Sprintf("(skipped %d binary/unreadable files)", skippedBinary))
	}
	if skippedLarge > 0 {
		notes = append(notes, fmt.Sprintf("(skipped %d large files)", skippedLarge))
	}

	if len(notes) > 0 {
		output += "\n\n" + strings.Join(notes, "\n")
	}

	return json.Marshal(output)
}

func (t *GrepTool) resolvePath(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(t.workspace, path)
	}
	return path
}

// ======================================
// Helper functions
// ======================================

func normalizePattern(pattern string) string {
	return strings.TrimSpace(strings.ReplaceAll(pattern, "\\", "/"))
}

func matchGlob(relPath string, name string, pattern string) bool {
	normalized := normalizePattern(pattern)
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "/") || strings.HasPrefix(normalized, "**") {
		matched, _ := filepath.Match(normalized, relPath)
		return matched
	}
	matched, _ := filepath.Match(normalized, name)
	return matched
}

func isBinary(raw []byte) bool {
	// Check for null bytes
	for _, b := range raw {
		if b == 0 {
			return true
		}
	}
	sample := raw
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	if len(sample) == 0 {
		return false
	}
	nonText := 0
	for _, b := range sample {
		if b < 9 || (13 < b && b < 32) {
			nonText++
		}
	}
	return float64(nonText)/float64(len(sample)) > 0.2
}

func matchesType(name string, fileType string) bool {
	if fileType == "" {
		return true
	}
	lowered := strings.ToLower(strings.TrimSpace(fileType))
	if lowered == "" {
		return true
	}
	patterns, ok := typeGlobMap[lowered]
	if !ok {
		patterns = []string{fmt.Sprintf("*.%s", lowered)}
	}
	for _, p := range patterns {
		matched, _ := filepath.Match(p, strings.ToLower(name))
		if matched {
			return true
		}
	}
	return false
}

func matchesQuery(relPath string, query string) bool {
	if query == "" {
		return true
	}
	haystack := strings.ToLower(relPath)
	terms := strings.Fields(strings.ToLower(query))
	for _, term := range terms {
		if term != "" && !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

func paginate(items []string, limit int, offset int) ([]string, bool) {
	if limit == 0 {
		if offset >= len(items) {
			return []string{}, false
		}
		return items[offset:], false
	}
	start := offset
	if start < 0 {
		start = 0
	}
	if start >= len(items) {
		return []string{}, false
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], end < len(items)
}

func formatBlock(displayPath string, lines []string, matchLine int, before int, after int) string {
	start := max(1, matchLine-before)
	end := min(len(lines), matchLine+after)
	var block []string
	block = append(block, fmt.Sprintf("%s:%d", displayPath, matchLine))
	for lineNo := start; lineNo <= end; lineNo++ {
		marker := " "
		if lineNo == matchLine {
			marker = ">"
		}
		lineContent := ""
		if lineNo-1 < len(lines) {
			lineContent = lines[lineNo-1]
		}
		block = append(block, fmt.Sprintf("%s %4d| %s", marker, lineNo, lineContent))
	}
	return strings.Join(block, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
