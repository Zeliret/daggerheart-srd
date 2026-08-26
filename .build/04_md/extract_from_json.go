package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

func main() {
	jsonDir := ".build/03_json"
	templateDir := ".build/04_md/templates"
	outputDir := ".build/04_md/docs"
	srdBasePath := ".build/01_pdf/DH-SRD-2.0-2026-08-25.md"
	srdPath := "README.md"
	adversaryLinks := map[string]string{}
	if adversaries, err := loadJSON(filepath.Join(jsonDir, "adversaries.json")); err == nil {
		for _, adversary := range adversaries {
			if name, ok := adversary["name"].(string); ok && strings.TrimSpace(name) != "" {
				target := fmt.Sprintf("../adversaries/%s.md", url.PathEscape(sanitizeFilename(name)))
				adversaryLinks[name] = target
				if !strings.HasSuffix(name, "s") {
					adversaryLinks[name+"s"] = target
				}
			}
		}
	}
	// Retained legacy adversary documents are valid link targets too. Read their
	// H1s so an environment can link any matching adversary in the repository,
	// not only entries present in the current appendix CSV.
	if files, err := os.ReadDir("adversaries"); err == nil {
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
				continue
			}
			if content, err := os.ReadFile(filepath.Join("adversaries", file.Name())); err == nil {
				name := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(string(content), "\n", 2)[0], "# "))
				if name != "" {
					target := "../adversaries/" + url.PathEscape(file.Name())
					adversaryLinks[name] = target
					if !strings.HasSuffix(name, "s") {
						adversaryLinks[name+"s"] = target
					}
				}
			}
		}
	}

	entries, err := os.ReadDir(jsonDir)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", jsonDir, err)
		return
	}

	funcs := template.FuncMap{
		"upper":            strings.ToUpper,
		"urlEncode":        url.PathEscape,
		"featureQuestions": featureQuestions,
		"fileName":         sanitizeFilename,
		"abilityLink":      abilityLink,
		"environmentAdversaryLinks": func(value string) string {
			return linkEnvironmentAdversaries(value, adversaryLinks)
		},
		"adversaryFeatureText": formatAdversaryFeatureText,
		"mechanicsText":        mechanicsText,
		"optionAt":             optionAt,
		"add1":                 add1,
		"sourceMarkdown":       sourceMarkdown,
		"classSourceMarkdown":  classSourceMarkdown,
	}

	var beastforms []map[string]any
	var beastformTiers []map[string]any
	if bf, err := loadJSON(filepath.Join(jsonDir, "beastforms.json")); err == nil {
		beastforms = bf
		beastformTiers = groupBeastformsByTier(beastforms)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		templatePath := filepath.Join(templateDir, base+".md")
		if _, err := os.Stat(templatePath); err != nil {
			continue
		}
		jsonPath := filepath.Join(jsonDir, entry.Name())
		items, err := loadJSON(jsonPath)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", jsonPath, err)
			continue
		}
		templateName := filepath.Base(templatePath)
		tmpl, err := template.New(templateName).Funcs(funcs).ParseFiles(templatePath)
		if err != nil {
			fmt.Printf("Error parsing template %s: %v\n", templatePath, err)
			continue
		}

		categoryDir := filepath.Join(outputDir, base)
		if err := os.MkdirAll(categoryDir, 0755); err != nil {
			fmt.Printf("Error creating %s: %v\n", categoryDir, err)
			continue
		}

		for _, item := range items {
			name, _ := item["name"].(string)
			if strings.TrimSpace(name) == "" {
				continue
			}
			if base == "classes" && len(beastformTiers) > 0 {
				item["beastform_tiers"] = beastformTiers
			}
			normalizeItem(item)
			outPath := filepath.Join(categoryDir, sanitizeFilename(name)+".md")
			if err := renderTemplate(tmpl, templateName, outPath, item); err != nil {
				fmt.Printf("Error rendering %s (%s): %v\n", outPath, name, err)
				continue
			}
		}
	}

	// The README's catalog sections are generated from the current entity data.
	// Its prose remains curated (rather than copied from Marker extraction).
	if err := refreshReadmeIndexes(srdPath, jsonDir); err != nil {
		fmt.Printf("Error refreshing README indexes: %v\n", err)
	}
	// README.md is a curated, readable presentation of the SRD. The Marker
	// source remains intentionally unprocessed so it can be audited against
	// the PDF, and is not safe to publish directly. Keep that prose intact
	// during normal entity regeneration; explicitly opt in only when working
	// on the README generation pipeline.
	if os.Getenv("DAGGERHEART_REGENERATE_README") == "1" {
		if err := generateSRD(srdBasePath, srdPath, jsonDir); err != nil {
			fmt.Printf("Error generating %s: %v\n", srdPath, err)
		}
	} else {
		fmt.Println("Preserving curated README.md prose (set DAGGERHEART_REGENERATE_README=1 to regenerate it).")
	}
}

var sourceArtifact = regexp.MustCompile(`^(?:[0-9]+|Daggerheart SRD|<!-- PDF page [0-9]+ -->)$`)
var sourceAllCaps = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 '’&–—-]+$`)
var sourceMetadata = regexp.MustCompile(`^(DOMAINS|STARTING EVASION|STARTING HIT POINTS|CLASS ITEMS)\s+–\s+(.+)$`)
var sourceFeature = regexp.MustCompile(`^([A-Z][^:]{1,59}):\s+(.+)$`)
var sourceSubclassHeading = regexp.MustCompile(`(?m)^([A-Z][A-Z '’&–—-]+) SUBCLASSES\n`)
var classDomainLine = regexp.MustCompile(`(?m)^- \*\*DOMAINS —\*\* ([^&\n]+) & ([^\n]+)$`)
var rollEffectRow = regexp.MustCompile(`(?:^|\s)(\d+(?:[–-]\d+)?)\s+`)
var mechanicsResourceAction = regexp.MustCompile(`(?i)\b(?:spend|mark)\s+(?:(?:any|an?|one|two|three|four|five|six|\d+)\s+)?(?:(?:equal\s+)?(?:number|amount)\s+of\s+)?(?:Hope|Stress|Fear|Focus|Armor Slots?|Hit Points?)\b`)
var mechanicsRoll = regexp.MustCompile(`\b[A-Z][a-z]+(?:\s+or\s+[A-Z][a-z]+)?\s+(?:Reaction\s+)?Roll(?:\s+\(\d+\))?`)
var mechanicsDie = regexp.MustCompile(`\b(?:\d+)?d(?:4|6|8|10|12|20)s?(?:[+−-](?:(?:\d+)?d(?:4|6|8|10|12|20)s?|\d+))*\b`)
var mechanicsCondition = regexp.MustCompile(`(?i)\b(?:Marked for Death|Vulnerable|Restrained|Hidden|Cloaked|Chained|Cursed|Dazed|Poisoned|Rattled|Sickened|Trapped|Stunned|Silenced|Horrified|Frostbitten|Nauseated|Hungover)\b`)
var mechanicsMarkdownSpan = regexp.MustCompile(`\*\*[^*]+\*\*|_[^_]+_`)

// sourceMarkdown turns the clean text extracted from the two-column SRD PDF
// into readable Markdown. SRD 2.0 additions that do not yet have every legacy
// CSV field parsed use this path, avoiding empty template placeholders.
func sourceMarkdown(source string) string {
	var out, paragraph []string
	flush := func() {
		if len(paragraph) > 0 {
			out = append(out, mechanicsText(formatRollEffectTable(strings.Join(paragraph, " "))), "")
			paragraph = nil
		}
	}
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || sourceArtifact.MatchString(line) {
			flush()
			continue
		}
		if strings.HasPrefix(line, "•") {
			flush()
			out = append(out, "- "+mechanicsText(strings.TrimSpace(strings.TrimPrefix(line, "•"))))
			continue
		}
		if match := sourceMetadata.FindStringSubmatch(line); match != nil {
			flush()
			out = append(out, fmt.Sprintf("- **%s —** %s", match[1], match[2]), "")
			continue
		}
		if sourceAllCaps.MatchString(line) {
			flush()
			// The first line is the record title, which is already the document H1.
			if len(out) == 0 {
				continue
			}
			out = append(out, "### "+line, "")
			continue
		}
		if match := sourceFeature.FindStringSubmatch(line); match != nil {
			flush()
			paragraph = append(paragraph, fmt.Sprintf("**_%s:_** %s", match[1], match[2]))
			continue
		}
		paragraph = append(paragraph, line)
	}
	flush()
	return strings.TrimSpace(normalizeMarkdown(strings.Join(out, "\n")))
}

// formatRollEffectTable restores the compact outcome tables embedded in the
// tagged PDF text (for example, Witch's Commune feature).
func formatRollEffectTable(value string) string {
	marker := "Roll Effect"
	at := strings.Index(value, marker)
	if at < 0 {
		return value
	}
	prefix, table := strings.TrimSpace(value[:at]), strings.TrimSpace(value[at+len(marker):])
	matches := rollEffectRow.FindAllStringSubmatchIndex(table, -1)
	if len(matches) < 2 {
		return value
	}
	rows := make([]string, 0, len(matches))
	for i, match := range matches {
		end := len(table)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		effect := strings.TrimSpace(table[match[1]:end])
		if effect == "" {
			return value
		}
		rows = append(rows, fmt.Sprintf("| %s | %s |", table[match[2]:match[3]], effect))
	}
	return prefix + "\n\n| Roll | Effect |\n| --- | --- |\n" + strings.Join(rows, "\n")
}

// mechanicsText mirrors the PDF's selective emphasis in generated rules text:
// explicit costs, named rolls, and dice are bold; named conditions are italic.
// It operates on raw CSV/JSON content before Markdown is rendered.
func mechanicsText(value string) string {
	var out strings.Builder
	last := 0
	for _, match := range mechanicsMarkdownSpan.FindAllStringIndex(value, -1) {
		out.WriteString(emphasizeMechanics(value[last:match[0]]))
		out.WriteString(value[match[0]:match[1]])
		last = match[1]
	}
	out.WriteString(emphasizeMechanics(value[last:]))
	return out.String()
}

func emphasizeMechanics(value string) string {
	value = mechanicsResourceAction.ReplaceAllStringFunc(value, func(match string) string {
		return "**" + match + "**"
	})
	value = mechanicsRoll.ReplaceAllStringFunc(value, func(match string) string {
		return "**" + match + "**"
	})
	value = mechanicsDie.ReplaceAllStringFunc(value, func(match string) string {
		return "**" + match + "**"
	})
	return mechanicsCondition.ReplaceAllStringFunc(value, func(match string) string {
		return "_" + match + "_"
	})
}

// classSourceMarkdown keeps subclass rules in their canonical documents. The
// PDF presents a class followed by both subclass cards; legacy class documents
// instead link to those cards, so preserve that established repository shape.
func classSourceMarkdown(source, subclass1, subclass2 string) string {
	nameEnd := strings.Index(source, "\n")
	className := source
	if nameEnd >= 0 {
		className = source[:nameEnd]
		source = source[nameEnd+1:]
	}
	upper1, upper2 := strings.ToUpper(subclass1), strings.ToUpper(subclass2)
	classEnd := strings.Index(source, "\n"+strings.ToUpper(className)+" SUBCLASSES\n")
	if classEnd < 0 {
		classEnd = strings.Index(source, "\n"+upper1+"\n")
	}
	if classEnd < 0 {
		classEnd = strings.Index(source, "\n"+upper2+"\n")
	}
	backgroundAt := strings.Index(source, "\nBACKGROUND QUESTIONS\n")
	connectionsAt := strings.Index(source, "\nCONNECTIONS\n")
	classPart := source
	if classEnd >= 0 {
		classPart = source[:classEnd]
	}
	// The class-level source includes a transitional "SUBCLASSES" heading;
	// remove it because the canonical link section is added below.
	classPart = sourceSubclassHeading.ReplaceAllString(classPart, "")
	classPart = joinClassItemContinuations(classPart)
	classPart = formatSphereOfInfluenceExamples(classPart)
	var out []string
	if body := sourceMarkdown(classPart); body != "" {
		body = linkClassDomains(body)
		out = append(out, body)
	}
	out = append(out, "### SUBCLASSES", "", fmt.Sprintf("Choose either the **[%s](../subclasses/%s.md)** or **[%s](../subclasses/%s.md)** subclass.", subclass1, url.PathEscape(subclass1), subclass2, url.PathEscape(subclass2)))
	if backgroundAt >= 0 {
		end := len(source)
		if connectionsAt > backgroundAt {
			end = connectionsAt
		}
		questions := source[backgroundAt+len("\nBACKGROUND QUESTIONS\n") : end]
		if bullet := strings.Index(questions, "\n•"); bullet >= 0 {
			questions = questions[bullet:]
		}
		body := sourceMarkdown(questions)
		out = append(out, "", "### BACKGROUND QUESTIONS", "", "_Answer any of the following background questions. You can also create your own questions._", "", body)
	}
	if connectionsAt >= 0 {
		questions := source[connectionsAt+len("\nCONNECTIONS\n"):]
		if bullet := strings.Index(questions, "\n•"); bullet >= 0 {
			questions = questions[bullet:]
		}
		body := sourceMarkdown(questions)
		out = append(out, "", "### CONNECTIONS", "", "_Ask your fellow players one of the following questions for their character to answer, or create your own questions._", "", body)
	}
	return normalizeMarkdown(strings.Join(out, "\n"))
}

// linkClassDomains preserves the domain order printed by the SRD while
// matching the linked-domain presentation used by legacy class documents.
func linkClassDomains(body string) string {
	match := classDomainLine.FindStringSubmatchIndex(body)
	if match == nil {
		return body
	}
	first, second := strings.TrimSpace(body[match[2]:match[3]]), strings.TrimSpace(body[match[4]:match[5]])
	linked := fmt.Sprintf("- **DOMAINS —** [%s](../domains/%s.md) & [%s](../domains/%s.md)", first, url.PathEscape(first), second, url.PathEscape(second))
	return strings.TrimSpace(body[:match[0]]) + "\n\n---\n\n" + linked + body[match[1]:]
}

// joinClassItemContinuations repairs the one PDF layout case where a class's
// item list wraps onto the next line. Keep the item sentence in its metadata
// row so it renders like the established legacy class documents.
func joinClassItemContinuations(source string) string {
	lines := strings.Split(source, "\n")
	for i := 0; i+1 < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		next := strings.TrimSpace(lines[i+1])
		if !strings.HasPrefix(line, "CLASS ITEMS") || next == "" || sourceArtifact.MatchString(next) || sourceAllCaps.MatchString(next) {
			continue
		}
		first := next[0]
		if first >= 'a' && first <= 'z' {
			lines[i] = line + " " + next
			lines[i+1] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// formatSphereOfInfluenceExamples retains the Warlock source's one-entry-per-
// line examples as a Markdown list. Without bullets, the general PDF-source
// normalizer joins those short lines into one unreadable paragraph.
func formatSphereOfInfluenceExamples(source string) string {
	examples := []string{
		"Ambition", "Artists", "Chaos", "Darkness", "Death", "Gamblers",
		"Honor", "Justice", "Leaders", "Love", "Mercy", "Mischief",
		"Nature", "Protectors", "Revenge", "Scholars", "Secrets", "Soldiers",
		"Strength", "Travelers", "Tricksters", "Truth", "War", "Wisdom",
	}
	raw := strings.Join(examples, "\n")
	bullets := "• " + strings.Join(examples, "\n• ")
	return strings.Replace(source, raw, bullets, 1)
}

func loadJSON(path string) ([]map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func renderTemplate(tmpl *template.Template, name, outPath string, data map[string]any) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := tmpl.ExecuteTemplate(f, name, data); err != nil {
		return err
	}
	if err := collapseExtraBlankLines(outPath); err != nil {
		return err
	}
	return ensureCanonicalFilename(outPath)
}

func normalizeItem(item map[string]any) {
	ensureSlice(item, "feature")
	ensureSlice(item, "background")
	ensureSlice(item, "connection")
	ensureSlice(item, "question")
	ensureSlice(item, "foundation")
	ensureSlice(item, "specialization")
	ensureSlice(item, "mastery")
	ensureString(item, "suggested_secondary")
	stripEmbeddedFeatureQuestions(item)
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "’", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ReplaceAll(name, "&", "and")
	return name
}

func ensureSlice(item map[string]any, key string) {
	if _, ok := item[key]; !ok {
		item[key] = []any{}
	}
}

func ensureString(item map[string]any, key string) {
	if _, ok := item[key]; !ok {
		item[key] = ""
	}
}

func stripEmbeddedFeatureQuestions(item map[string]any) {
	features, ok := item["feature"].([]any)
	if !ok {
		return
	}
	for _, entry := range features {
		f, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		text, _ := f["text"].(string)
		question, _ := f["question"].(string)
		if text == "" || question == "" {
			continue
		}
		if strings.Contains(text, question) {
			f["question"] = ""
		}
	}
}

func collapseExtraBlankLines(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := normalizeMarkdown(strings.ReplaceAll(string(raw), "\r\n", "\n"))
	return os.WriteFile(path, []byte(out), 0644)
}

type linkTarget struct {
	name     string
	path     string
	category string
}

// refreshReadmeIndexes keeps the two large GM-facing catalogs complete as
// appendices grow. The PDF's printed lists are only a starting point; the JSON
// data is the repository's complete, linkable SRD 2.0 inventory.
func refreshReadmeIndexes(path, jsonDir string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	adversaries, err := loadJSON(filepath.Join(jsonDir, "adversaries.json"))
	if err != nil {
		return err
	}
	environments, err := loadJSON(filepath.Join(jsonDir, "environments.json"))
	if err != nil {
		return err
	}
	updated := string(content)
	updated, err = replaceReadmeSection(updated, "#### ADVERSARIES BY TIER\n", "### USING ENVIRONMENTS\n", readmeAdversaryIndex(adversaries))
	if err != nil {
		return err
	}
	updated, err = replaceReadmeSection(updated, "##### ENVIRONMENT STAT BLOCKS BY TIER\n", "### ADDITIONAL GM GUIDANCE\n", readmeEnvironmentIndex(environments))
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), 0644)
}

func replaceReadmeSection(content, start, end, replacement string) (string, error) {
	startAt := strings.Index(content, start)
	if startAt < 0 {
		return content, fmt.Errorf("missing README section %q", strings.TrimSpace(start))
	}
	endAt := strings.Index(content[startAt+len(start):], end)
	if endAt < 0 {
		return content, fmt.Errorf("missing README section terminator %q", strings.TrimSpace(end))
	}
	endAt += startAt + len(start)
	return content[:startAt] + replacement + "\n" + content[endAt:], nil
}

func readmeAdversaryIndex(adversaries []map[string]any) string {
	headings := map[int]string{1: "###### TIER 1 (LEVEL 1)", 2: "###### TIER 2 (LEVELS 2-4)", 3: "###### TIER 3 (LEVELS 5-7)", 4: "###### TIER 4 (LEVELS 8-10)"}
	return readmeTierIndex("#### ADVERSARIES BY TIER", "This section contains the following stat blocks:", "adversaries", adversaries, headings, false)
}

func readmeEnvironmentIndex(environments []map[string]any) string {
	headings := map[int]string{1: "###### TIER 1 (LEVEL 1)", 2: "###### TIER 2 (LEVELS 2-4)", 3: "###### TIER 3 (LEVELS 5-7)", 4: "###### TIER 4 (LEVELS 8-10)"}
	return readmeTierIndex("##### ENVIRONMENT STAT BLOCKS BY TIER", "This section contains the following stat blocks.", "environments", environments, headings, true)
}

func readmeTierIndex(title, description, category string, items []map[string]any, headings map[int]string, includeType bool) string {
	buckets := map[int][]map[string]any{}
	for _, item := range items {
		name, _ := item["name"].(string)
		tier := tierFromValue(item["tier"])
		if name != "" && headings[tier] != "" {
			buckets[tier] = append(buckets[tier], item)
		}
	}
	var out []string
	out = append(out, title, "", description)
	for tier := 1; tier <= 4; tier++ {
		items := buckets[tier]
		sort.Slice(items, func(i, j int) bool {
			return strings.ToLower(items[i]["name"].(string)) < strings.ToLower(items[j]["name"].(string))
		})
		out = append(out, "", headings[tier], "")
		for _, item := range items {
			name := item["name"].(string)
			line := fmt.Sprintf("- [%s](%s/%s.md)", name, category, url.PathEscape(sanitizeFilename(name)))
			if includeType {
				if kind, _ := item["type"].(string); kind != "" {
					line += " (" + kind + ")"
				}
			}
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func generateSRD(basePath, outPath, jsonDir string) error {
	baseRaw, err := os.ReadFile(basePath)
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(baseRaw), "\r\n", "\n")

	nameLinks, sectionLinks, categoryLinks, err := loadSRDLinkMaps(jsonDir)
	if err != nil {
		return err
	}

	content = replaceTableNamesWithLinks(content, sectionLinks)
	content = replaceListItemsWithLinks(content, categoryLinks)
	content = replaceHeadingBlocksWithLinks(content, nameLinks)
	content = replaceClassDomainLinks(content, categoryLinks["classes"], categoryLinks["domains"])
	content = replaceClassMentionList(content, categoryLinks["classes"])
	content = replaceAncestryMentionList(content, categoryLinks["ancestries"])
	content = insertCommunityParagraph(content, categoryLinks["communities"])
	content = insertDomainCardReferenceList(content, categoryLinks["domains"])
	content = removeDomainCardReferenceSections(content, categoryLinks["domains"])
	content = removeSectionsByHeadingPrefix(content, 2, []string{
		"TIER 1 ENVIRONMENTS",
		"TIER 2 ENVIRONMENTS",
		"TIER 3 ENVIRONMENTS",
		"TIER 4 ENVIRONMENTS",
		"TIER 1 ADVERSARIES",
		"TIER 2 ADVERSARIES",
		"TIER 3 ADVERSARIES",
		"TIER 4 ADVERSARIES",
	})
	content = normalizeMarkdown(content)
	content = insertContents(content)
	content = demoteHeadings(content)
	return os.WriteFile(outPath, []byte(content), 0644)
}

func loadSRDLinkMaps(jsonDir string) (map[string]linkTarget, map[string]map[string]linkTarget, map[string]map[string]linkTarget, error) {
	nameLinks := map[string]linkTarget{}
	sectionLinks := map[string]map[string]linkTarget{}
	categoryLinks := map[string]map[string]linkTarget{}
	sectionMap := map[string]string{
		"WEAPONS":     "weapons",
		"ARMOR":       "armor",
		"CONSUMABLES": "consumables",
		"ITEMS":       "items",
		"LOOT":        "items",
	}

	entries, err := os.ReadDir(jsonDir)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		items, err := loadJSON(filepath.Join(jsonDir, entry.Name()))
		if err != nil {
			return nil, nil, nil, err
		}
		for _, item := range items {
			name, _ := item["name"].(string)
			if strings.TrimSpace(name) == "" {
				continue
			}
			link := buildSRDLink(base, name)
			key := normalizeHeading(name)
			if _, exists := nameLinks[key]; !exists {
				nameLinks[key] = linkTarget{name: name, path: link, category: base}
			}
			for section, sectionBase := range sectionMap {
				if base == sectionBase {
					if sectionLinks[section] == nil {
						sectionLinks[section] = map[string]linkTarget{}
					}
					sectionLinks[section][key] = linkTarget{name: name, path: link, category: base}
				}
			}
			if categoryLinks[base] == nil {
				categoryLinks[base] = map[string]linkTarget{}
			}
			categoryLinks[base][key] = linkTarget{name: name, path: link, category: base}
		}
	}
	return nameLinks, sectionLinks, categoryLinks, nil
}

func buildSRDLink(category, name string) string {
	file := url.PathEscape(sanitizeFilename(name))
	return fmt.Sprintf("%s/%s.md", category, file)
}

func replaceHeadingBlocksWithLinks(content string, nameLinks map[string]linkTarget) string {
	lines := strings.Split(content, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		level, title := parseHeading(line)
		if level == 0 {
			out = append(out, line)
			continue
		}
		key := normalizeHeading(title)
		target, ok := nameLinks[key]
		if !ok {
			out = append(out, line)
			continue
		}
		if target.category == "adversaries" || target.category == "environments" {
			i = skipHeadingBlock(lines, i, level)
			continue
		}
		if target.category == "classes" {
			i = skipHeadingBlock(lines, i, level)
			continue
		}
		if target.category == "ancestries" {
			i = skipHeadingBlock(lines, i, level)
			continue
		}
		if target.category == "communities" {
			i = skipHeadingBlock(lines, i, level)
			continue
		}
		if target.category == "domains" {
			out = append(out, fmt.Sprintf("- [%s](%s)", target.name, target.path))
			out = append(out, "")
			i = skipHeadingBlock(lines, i, level)
			continue
		}
		out = append(out, line)
		out = append(out, "")
		out = append(out, fmt.Sprintf("See: [%s](%s)", target.name, target.path))
		out = append(out, "")
		i = skipHeadingBlock(lines, i, level)
	}
	return strings.Join(out, "\n")
}

func replaceTableNamesWithLinks(content string, sectionLinks map[string]map[string]linkTarget) string {
	lines := strings.Split(content, "\n")
	currentSection := ""
	for i, line := range lines {
		level, title := parseHeading(line)
		if level == 2 {
			currentSection = strings.ToUpper(strings.TrimSpace(title))
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		links := sectionLinks[currentSection]
		if len(links) == 0 {
			continue
		}
		columnIndex := 1
		if currentSection == "LOOT" || currentSection == "CONSUMABLES" {
			columnIndex = 2
		}
		updated, changed := replaceTableCell(line, links, columnIndex)
		if changed {
			lines[i] = updated
		}
	}
	return strings.Join(lines, "\n")
}

func replaceListItemsWithLinks(content string, categoryLinks map[string]map[string]linkTarget) string {
	lines := strings.Split(content, "\n")
	inAdversaryList := false
	inEnvironmentSection := false
	for i, line := range lines {
		level, title := parseHeading(line)
		if level > 0 {
			if level == 3 && strings.EqualFold(strings.TrimSpace(title), "ADVERSARIES BY TIER") {
				inAdversaryList = true
			} else if level <= 3 {
				inAdversaryList = false
			}
			if level == 2 {
				inEnvironmentSection = strings.EqualFold(strings.TrimSpace(title), "USING ENVIRONMENTS")
			}
		}
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		itemText := strings.TrimSpace(trimmed[2:])
		var updated string
		var changed bool
		if inAdversaryList {
			updated, changed = linkifyListItem(itemText, categoryLinks["adversaries"], false)
		} else if inEnvironmentSection {
			updated, changed = linkifyListItem(itemText, categoryLinks["environments"], true)
		}
		if changed {
			lines[i] = strings.Replace(line, trimmed, "- "+updated, 1)
		}
	}
	return strings.Join(lines, "\n")
}

func replaceClassDomainLinks(content string, classLinks, domainLinks map[string]linkTarget) string {
	if len(classLinks) == 0 || len(domainLinks) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	inClassDomains := false
	for i, line := range lines {
		level, title := parseHeading(line)
		if level == 2 {
			inClassDomains = strings.EqualFold(strings.TrimSpace(title), "CLASS DOMAINS")
		}
		if !inClassDomains {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- **") || !strings.Contains(trimmed, ":") {
			continue
		}
		start := strings.Index(trimmed, "**")
		if start == -1 {
			continue
		}
		rest := trimmed[start+2:]
		end := strings.Index(rest, "**")
		if end == -1 {
			continue
		}
		className := strings.TrimSuffix(rest[:end], ":")
		after := strings.TrimSpace(rest[end+2:])
		domainPart := strings.TrimSpace(strings.TrimPrefix(after, ":"))
		if domainPart == "" {
			continue
		}
		linkedDomains := linkDomainList(domainPart, domainLinks)
		linkedClass := className
		if target, ok := classLinks[normalizeHeading(className)]; ok {
			linkedClass = fmt.Sprintf("[%s](%s)", target.name, target.path)
		}
		lines[i] = fmt.Sprintf("- **%s:** %s", linkedClass, linkedDomains)
	}
	return strings.Join(lines, "\n")
}

func replaceClassMentionList(content string, classLinks map[string]linkTarget) string {
	if len(classLinks) == 0 {
		return content
	}
	prefix := "There are 9 classes in the Daggerheart core materials:"
	start := strings.Index(content, prefix)
	if start == -1 {
		return content
	}
	end := strings.Index(content[start:], ".")
	if end == -1 {
		return content
	}
	end += start
	listText := content[start+len(prefix) : end]
	parts := strings.Split(listText, ",")
	if len(parts) < 2 {
		return content
	}
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	lastPart := parts[len(parts)-1]
	if strings.HasPrefix(lastPart, "and ") {
		lastPart = strings.TrimSpace(strings.TrimPrefix(lastPart, "and "))
		parts[len(parts)-1] = lastPart
	}
	for i, name := range parts {
		target, ok := classLinks[normalizeHeading(name)]
		if !ok {
			continue
		}
		parts[i] = fmt.Sprintf("[%s](%s)", target.name, target.path)
	}
	if len(parts) > 1 {
		parts[len(parts)-1] = "and " + parts[len(parts)-1]
	}
	repl := prefix + " " + strings.Join(parts, ", ") + "."
	content = content[:start] + repl + content[end+1:]
	return content
}

func replaceAncestryMentionList(content string, ancestryLinks map[string]linkTarget) string {
	if len(ancestryLinks) == 0 {
		return content
	}
	prefix := "The core ruleset includes the following ancestries:"
	start := strings.Index(content, prefix)
	if start == -1 {
		return content
	}
	end := strings.Index(content[start:], ".")
	if end == -1 {
		return content
	}
	end += start
	listText := content[start+len(prefix) : end]
	parts := strings.Split(listText, ",")
	if len(parts) < 2 {
		return content
	}
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	lastPart := parts[len(parts)-1]
	if strings.HasPrefix(lastPart, "and ") {
		lastPart = strings.TrimSpace(strings.TrimPrefix(lastPart, "and "))
		parts[len(parts)-1] = lastPart
	}
	for i, name := range parts {
		target, ok := ancestryLinks[normalizeHeading(name)]
		if !ok {
			continue
		}
		parts[i] = fmt.Sprintf("[%s](%s)", target.name, target.path)
	}
	if len(parts) > 1 {
		parts[len(parts)-1] = "and " + parts[len(parts)-1]
	}
	repl := prefix + " " + strings.Join(parts, ", ") + "."
	content = content[:start] + repl + content[end+1:]
	return content
}

func insertCommunityParagraph(content string, communityLinks map[string]linkTarget) string {
	if len(communityLinks) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		level, title := parseHeading(line)
		if level == 2 && strings.EqualFold(strings.TrimSpace(title), "COMMUNITIES") {
			end := i + 1
			for end < len(lines) {
				nextLevel, _ := parseHeading(lines[end])
				if nextLevel > 0 && nextLevel <= 2 {
					break
				}
				end++
			}
			var names []string
			for _, target := range communityLinks {
				names = append(names, target.name)
			}
			sort.Strings(names)
			var linked []string
			for _, name := range names {
				target := communityLinks[normalizeHeading(name)]
				linked = append(linked, fmt.Sprintf("[%s](%s)", target.name, target.path))
			}
			paragraph := "The core ruleset includes the following communities: " + strings.Join(linked, ", ") + "."
			block := []string{"", paragraph, ""}
			out := append([]string{}, lines[:end]...)
			out = append(out, block...)
			out = append(out, lines[end:]...)
			return strings.Join(out, "\n")
		}
	}
	return content
}

func insertDomainCardReferenceList(content string, domainLinks map[string]linkTarget) string {
	if len(domainLinks) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		level, title := parseHeading(line)
		if level == 2 && strings.EqualFold(strings.TrimSpace(title), "DOMAIN CARD REFERENCE") {
			insertAt := i + 1
			var names []string
			for _, target := range domainLinks {
				names = append(names, target.name)
			}
			sort.Strings(names)
			var block []string
			block = append(block, "")
			for _, name := range names {
				target := domainLinks[normalizeHeading(name)]
				block = append(block, fmt.Sprintf("- [%s](%s)", target.name, target.path))
			}
			block = append(block, "")
			out := append([]string{}, lines[:insertAt]...)
			out = append(out, block...)
			out = append(out, lines[insertAt:]...)
			return strings.Join(out, "\n")
		}
	}
	return content
}

func removeDomainCardReferenceSections(content string, domainLinks map[string]linkTarget) string {
	if len(domainLinks) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		level, title := parseHeading(lines[i])
		if level == 3 && strings.HasSuffix(strings.ToUpper(strings.TrimSpace(title)), " DOMAIN") {
			name := strings.TrimSpace(strings.TrimSuffix(title, "DOMAIN"))
			name = strings.TrimSpace(strings.TrimSuffix(name, "Domain"))
			if _, ok := domainLinks[normalizeHeading(name)]; ok {
				i = skipHeadingBlock(lines, i, level)
				continue
			}
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}

func linkDomainList(value string, domainLinks map[string]linkTarget) string {
	parts := strings.Split(value, "&")
	if len(parts) == 1 {
		return linkSingleDomain(value, domainLinks)
	}
	for i, part := range parts {
		parts[i] = linkSingleDomain(part, domainLinks)
	}
	return strings.Join(parts, " & ")
}

func linkSingleDomain(value string, domainLinks map[string]linkTarget) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return value
	}
	if target, ok := domainLinks[normalizeHeading(name)]; ok {
		return fmt.Sprintf("[%s](%s)", target.name, target.path)
	}
	return name
}

func linkifyListItem(item string, links map[string]linkTarget, allowSuffix bool) (string, bool) {
	if len(links) == 0 {
		return item, false
	}
	parts := strings.Split(item, "•")
	changed := false
	for i, part := range parts {
		part = strings.TrimSpace(part)
		base := part
		suffix := ""
		if allowSuffix {
			if idx := strings.Index(part, " ("); idx != -1 {
				base = strings.TrimSpace(part[:idx])
				suffix = part[idx:]
			}
		}
		key := normalizeHeading(base)
		target, ok := links[key]
		if ok {
			parts[i] = fmt.Sprintf("[%s](%s)%s", target.name, target.path, suffix)
			changed = true
		} else {
			parts[i] = part
		}
	}
	return strings.Join(parts, " • "), changed
}

func replaceTableCell(line string, links map[string]linkTarget, columnIndex int) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return line, false
	}
	parts := strings.Split(line, "|")
	if len(parts) <= columnIndex {
		return line, false
	}
	if isTableSeparatorRow(parts) {
		return line, false
	}
	cell := strings.TrimSpace(parts[columnIndex])
	cellPlain := stripMarkdownEmphasis(cell)
	if strings.EqualFold(cellPlain, "name") || strings.EqualFold(cellPlain, "roll") || strings.EqualFold(cellPlain, "loot") {
		return line, false
	}
	key := normalizeHeading(cell)
	target, ok := links[key]
	if !ok {
		return line, false
	}
	parts[columnIndex] = " [" + target.name + "](" + target.path + ") "
	return strings.Join(parts, "|"), true
}

func isTableSeparatorRow(parts []string) bool {
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		for _, r := range trimmed {
			if r != '-' {
				return false
			}
		}
		return true
	}
	return false
}

func stripMarkdownEmphasis(value string) string {
	out := strings.ReplaceAll(value, "**", "")
	out = strings.ReplaceAll(out, "__", "")
	out = strings.ReplaceAll(out, "*", "")
	out = strings.ReplaceAll(out, "_", "")
	return strings.TrimSpace(out)
}

// linkEnvironmentAdversaries preserves environment group labels while linking
// every exact adversary name available in the generated adversary collection.
func linkEnvironmentAdversaries(value string, adversaryLinks map[string]string) string {
	names := make([]string, 0, len(adversaryLinks))
	for name := range adversaryLinks {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	patterns := make([]string, 0, len(names))
	for _, name := range names {
		patterns = append(patterns, regexp.QuoteMeta(name))
	}
	matcher := regexp.MustCompile(`\b(?:` + strings.Join(patterns, "|") + `)\b`)
	return matcher.ReplaceAllStringFunc(value, func(name string) string {
		return fmt.Sprintf("[%s](%s)", name, adversaryLinks[name])
	})
}

// formatAdversaryFeatureText restores list semantics lost when the PDF's
// wrapped feature copy is compacted into a CSV field.
func formatAdversaryFeatureText(value string) string {
	numbered := regexp.MustCompile(`(?:^|\s)([1-9][0-9]*)\.\s+`)
	matches := numbered.FindAllStringSubmatchIndex(value, -1)
	if len(matches) >= 2 && value[matches[0][2]:matches[0][3]] == "1" && value[matches[1][2]:matches[1][3]] == "2" {
		items := make([]string, 0, len(matches))
		for index, match := range matches {
			end := len(value)
			if index+1 < len(matches) {
				end = matches[index+1][0]
			}
			items = append(items, value[match[2]:match[3]]+". "+strings.TrimSpace(value[match[1]:end]))
		}
		return strings.TrimSpace(value[:matches[0][0]]) + "\n\n" + strings.Join(items, "\n")
	}

	if strings.Contains(value, "• ") {
		parts := strings.Split(value, "• ")
		if len(parts) > 1 {
			items := make([]string, 0, len(parts)-1)
			for _, part := range parts[1:] {
				if item := strings.TrimSpace(part); item != "" {
					items = append(items, "- "+item)
				}
			}
			return strings.TrimSpace(parts[0]) + "\n\n" + strings.Join(items, "\n")
		}
	}
	return value
}

func abilityLink(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" || clean == "—" {
		return "—"
	}
	return fmt.Sprintf("[%s](../abilities/%s.md)", clean, url.PathEscape(sanitizeFilename(clean)))
}

func optionAt(options any, index int) string {
	if index <= 0 {
		return ""
	}
	switch typed := options.(type) {
	case []any:
		if len(typed) < index {
			return ""
		}
		if val, ok := typed[index-1].(string); ok {
			return val
		}
	case []string:
		if len(typed) < index {
			return ""
		}
		return typed[index-1]
	}
	return ""
}

func add1(value int) int {
	return value + 1
}

func parseHeading(line string) (int, string) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, ""
	}
	hashes := 0
	for hashes < len(trimmed) && trimmed[hashes] == '#' {
		hashes++
	}
	if hashes == 0 || hashes >= len(trimmed) || trimmed[hashes] != ' ' {
		return 0, ""
	}
	return hashes, strings.TrimSpace(trimmed[hashes:])
}

func skipHeadingBlock(lines []string, start, level int) int {
	for i := start + 1; i < len(lines); i++ {
		nextLevel, _ := parseHeading(lines[i])
		if nextLevel > 0 && nextLevel <= level {
			return i - 1
		}
	}
	return len(lines) - 1
}

func normalizeHeading(text string) string {
	clean := strings.ToLower(strings.TrimSpace(text))
	clean = strings.ReplaceAll(clean, "’", "")
	clean = strings.ReplaceAll(clean, "'", "")
	var b strings.Builder
	space := false
	for _, r := range clean {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func removeSectionsByHeadingPrefix(content string, level int, prefixes []string) string {
	lines := strings.Split(content, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		lvl, title := parseHeading(lines[i])
		if lvl == level && hasHeadingPrefix(title, prefixes) {
			i = skipHeadingBlock(lines, i, lvl)
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}

func hasHeadingPrefix(title string, prefixes []string) bool {
	upper := strings.ToUpper(strings.TrimSpace(title))
	for _, prefix := range prefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func rtrimLines(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

func removeBlankLinesBetweenListItems(input string) string {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line == "" && i > 0 && i+1 < len(lines) {
			if isListItem(lines[i-1]) && isListItem(lines[i+1]) {
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isListItem(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "- ") {
		return true
	}
	dot := strings.IndexByte(trimmed, '.')
	if dot <= 0 {
		return false
	}
	for i := 0; i < dot; i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return false
		}
	}
	return len(trimmed) > dot+1 && trimmed[dot+1] == ' '
}

func normalizeMarkdown(input string) string {
	out := rtrimLines(input)
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	out = removeBlankLinesBetweenListItems(out)
	return out
}

func ensureCanonicalFilename(path string) error {
	dir := filepath.Dir(path)
	want := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.EqualFold(name, want) || name == want {
			continue
		}
		tmp := want + ".casefix"
		oldPath := filepath.Join(dir, name)
		tmpPath := filepath.Join(dir, tmp)
		if err := os.Rename(oldPath, tmpPath); err != nil {
			return err
		}
		return os.Rename(tmpPath, path)
	}
	return nil
}

func insertContents(content string) string {
	lines := strings.Split(content, "\n")
	var toc []string
	seenFirst := false
	for _, line := range lines {
		level, title := parseHeading(line)
		if level == 0 || level > 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(title), "contents") {
			continue
		}
		if level == 1 && !seenFirst {
			seenFirst = true
			continue
		}
		anchor := headingAnchor(title)
		if anchor == "" {
			continue
		}
		display := titleCaseHeading(title)
		if level == 1 {
			if len(toc) > 0 && toc[len(toc)-1] != "" {
				toc = append(toc, "")
			}
			toc = append(toc, fmt.Sprintf("**[%s](#%s)**", display, anchor), "")
			continue
		}
		toc = append(toc, fmt.Sprintf("- [%s](#%s)", display, anchor))
	}
	if len(toc) == 0 {
		return content
	}
	for i, line := range lines {
		level, title := parseHeading(line)
		if level == 1 && strings.EqualFold(strings.TrimSpace(title), "INTRODUCTION") {
			block := []string{
				"**REFERENCE SITE**",
				"",
				"[seansbox.github.io/daggerheart-srd](https://seansbox.github.io/daggerheart-srd/)",
				"",
				"# CONTENTS",
				"",
			}
			block = append(block, toc...)
			block = append(block, "")
			out := append([]string{}, lines[:i]...)
			out = append(out, block...)
			out = append(out, lines[i:]...)
			return strings.Join(out, "\n")
		}
	}
	block := append([]string{
		"#### REFERENCE SITE",
		"",
		"[seansbox.github.io/daggerheart-srd](https://seansbox.github.io/daggerheart-srd/)",
		"",
		"# CONTENTS",
		"",
	}, toc...)
	block = append(block, "", "")
	return strings.Join(append(block, lines...), "\n")
}

func demoteHeadings(content string) string {
	lines := strings.Split(content, "\n")
	first := true
	for i, line := range lines {
		level, title := parseHeading(line)
		if level == 0 {
			continue
		}
		if first {
			first = false
			continue
		}
		level++
		if level > 6 {
			level = 6
		}
		lines[i] = strings.Repeat("#", level) + " " + strings.TrimSpace(title)
	}
	return strings.Join(lines, "\n")
}

func headingAnchor(title string) string {
	clean := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	dash := false
	for _, r := range clean {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	anchor := strings.Trim(b.String(), "-")
	return anchor
}

func titleCaseHeading(title string) string {
	smallWords := map[string]bool{
		"a": true, "an": true, "and": true, "as": true, "at": true,
		"but": true, "by": true, "for": true, "from": true, "in": true,
		"of": true, "on": true, "or": true, "the": true, "to": true,
		"via": true, "with": true, "over": true, "into": true,
	}
	acronyms := map[string]bool{
		"GM": true, "SRD": true, "HP": true, "XP": true, "PC": true, "NPC": true, "ATK": true,
	}
	parts := strings.Fields(title)
	for i, part := range parts {
		upper := strings.ToUpper(part)
		lower := strings.ToLower(part)
		if i > 0 && smallWords[lower] {
			parts[i] = lower
			continue
		}
		if part == upper && acronyms[part] {
			parts[i] = part
			continue
		}
		parts[i] = titleizeWord(lower)
	}
	return strings.Join(parts, " ")
}

func titleizeWord(word string) string {
	if word == "" {
		return word
	}
	segments := strings.Split(word, "-")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		runes := []rune(seg)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		segments[i] = string(runes)
	}
	return strings.Join(segments, "-")
}

func groupBeastformsByTier(beastforms []map[string]any) []map[string]any {
	tierBuckets := map[int][]map[string]any{}
	for _, beast := range beastforms {
		tier := tierFromValue(beast["tier"])
		tierBuckets[tier] = append(tierBuckets[tier], beast)
	}
	ordered := []int{1, 2, 3, 4}
	seen := map[int]bool{}
	for _, t := range ordered {
		seen[t] = true
	}
	var tiers []map[string]any
	for _, t := range ordered {
		items := tierBuckets[t]
		if len(items) == 0 {
			continue
		}
		tiers = append(tiers, map[string]any{
			"tier":  t,
			"items": items,
		})
	}
	for t, items := range tierBuckets {
		if seen[t] || len(items) == 0 {
			continue
		}
		tiers = append(tiers, map[string]any{
			"tier":  t,
			"items": items,
		})
	}
	return tiers
}

func tierFromValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var out int
		fmt.Sscanf(v, "%d", &out)
		if out > 0 {
			return out
		}
	}
	return 0
}

func featureQuestions(features any) []string {
	list, ok := features.([]any)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		q, ok := m["question"].(string)
		if !ok {
			continue
		}
		q = strings.TrimSpace(q)
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		out = append(out, q)
	}
	return out
}
