package core

import "strings"

func splitFrontmatter(text string) (body string, frontmatter string) {
	text = strings.TrimPrefix(text, "\ufeff")
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return text, ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n"), strings.Join(lines[1:i], "\n")
		}
	}
	return text, ""
}

func frontmatterFields(frontmatter string) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(frontmatter) == "" {
		return fields
	}
	lines := strings.Split(frontmatter, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if value == "" {
			var list []string
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if strings.HasPrefix(next, "-") {
					list = append(list, cleanFrontmatterValue(strings.TrimSpace(strings.TrimPrefix(next, "-"))))
					continue
				}
				if next != "" && !strings.HasPrefix(lines[j], " ") && !strings.HasPrefix(lines[j], "\t") {
					break
				}
			}
			if len(list) > 0 {
				fields[key] = strings.Join(list, ",")
			}
			continue
		}
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
			var list []string
			for _, part := range strings.Split(value, ",") {
				if cleaned := cleanFrontmatterValue(part); cleaned != "" {
					list = append(list, cleaned)
				}
			}
			fields[key] = strings.Join(list, ",")
			continue
		}
		fields[key] = cleanFrontmatterValue(value)
	}
	return fields
}

func frontmatterTags(frontmatter string) []string {
	fields := frontmatterFields(frontmatter)
	value := fields["tags"]
	if value == "" {
		value = fields["Tags"]
	}
	if value == "" {
		return nil
	}
	var tags []string
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if tag := cleanFrontmatterValue(part); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func cleanFrontmatterValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}
