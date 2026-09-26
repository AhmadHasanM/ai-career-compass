package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Taxonomy struct {
	Version int     `yaml:"version"`
	Roles   []Role  `yaml:"roles"`
	Skills  []Skill `yaml:"skills"`
}

type Role struct {
	Name string `yaml:"name"`
	Slug string `yaml:"slug"`
}

type Skill struct {
	Slug          string   `yaml:"slug"`
	Name          string   `yaml:"name"`
	Category      string   `yaml:"category"`
	Description   string   `yaml:"description"`
	Aliases       []string `yaml:"aliases"`
	Prerequisites []string `yaml:"prerequisites"`
}

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	validCategories = map[string]bool{
		"language": true, "framework": true, "llm": true, "ml": true,
		"mlops": true, "cloud": true, "data": true,
	}
)

func LoadTaxonomy(path string) (*Taxonomy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Taxonomy
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &t, nil
}

// AllAliases mengembalikan alias final sebuah skill: nama dan slug (lowercase)
// ditambah alias eksplisit, tanpa duplikat.
func (s Skill) AllAliases() []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range append([]string{strings.ToLower(s.Name), s.Slug}, s.Aliases...) {
		a = strings.TrimSpace(a)
		if a != "" && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

// Validate mengumpulkan semua pelanggaran agar bisa diperbaiki sekaligus.
func (t *Taxonomy) Validate() error {
	var errs []string
	addErr := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	roleSlugs := map[string]bool{}
	for _, r := range t.Roles {
		if !slugPattern.MatchString(r.Slug) {
			addErr("role %q: slug tidak valid", r.Slug)
		}
		if roleSlugs[r.Slug] {
			addErr("role %q: slug duplikat", r.Slug)
		}
		roleSlugs[r.Slug] = true
	}

	skills := map[string]Skill{}
	for _, s := range t.Skills {
		if !slugPattern.MatchString(s.Slug) {
			addErr("skill %q: slug tidak valid", s.Slug)
		}
		if _, dup := skills[s.Slug]; dup {
			addErr("skill %q: slug duplikat", s.Slug)
		}
		if strings.TrimSpace(s.Name) == "" {
			addErr("skill %q: name kosong", s.Slug)
		}
		if !validCategories[s.Category] {
			addErr("skill %q: category %q tidak valid", s.Slug, s.Category)
		}
		skills[s.Slug] = s
	}

	aliasOwner := map[string]string{}
	for _, s := range t.Skills {
		for _, a := range s.Aliases {
			if a != strings.ToLower(a) {
				addErr("skill %q: alias %q harus lowercase", s.Slug, a)
			}
		}
		for _, a := range s.AllAliases() {
			if owner, ok := aliasOwner[a]; ok && owner != s.Slug {
				addErr("alias %q dipakai oleh %q dan %q", a, owner, s.Slug)
			}
			aliasOwner[a] = s.Slug
		}
		for _, p := range s.Prerequisites {
			if _, ok := skills[p]; !ok {
				addErr("skill %q: prerequisite %q tidak ada di taxonomy", s.Slug, p)
			}
			if p == s.Slug {
				addErr("skill %q: prerequisite ke dirinya sendiri", s.Slug)
			}
		}
	}

	if cycle := findCycle(t.Skills); cycle != nil {
		addErr("siklus prerequisite: %s", strings.Join(cycle, " -> "))
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("taxonomy tidak valid:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// findCycle mengembalikan satu siklus pada graf prerequisite, atau nil jika graf berupa DAG.
func findCycle(skills []Skill) []string {
	edges := map[string][]string{}
	for _, s := range skills {
		edges[s.Slug] = s.Prerequisites
	}
	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var stack []string

	var visit func(string) []string
	visit = func(n string) []string {
		state[n] = visiting
		stack = append(stack, n)
		for _, m := range edges[n] {
			switch state[m] {
			case visiting:
				for i, v := range stack {
					if v == m {
						return append(append([]string{}, stack[i:]...), m)
					}
				}
			case unvisited:
				if c := visit(m); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
		return nil
	}

	for _, s := range skills {
		if state[s.Slug] == unvisited {
			if c := visit(s.Slug); c != nil {
				return c
			}
		}
	}
	return nil
}
