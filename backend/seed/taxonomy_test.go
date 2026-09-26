package main

import (
	"strings"
	"testing"
)

func TestRepoTaxonomyIsValid(t *testing.T) {
	tax, err := LoadTaxonomy("../../data/taxonomy/skills.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := tax.Validate(); err != nil {
		t.Fatal(err)
	}
	if n := len(tax.Skills); n < 60 || n > 80 {
		t.Errorf("jumlah skill %d, target 60–80", n)
	}
}

func TestValidateDetectsProblems(t *testing.T) {
	tax := &Taxonomy{
		Skills: []Skill{
			{Slug: "a", Name: "A", Category: "ml", Prerequisites: []string{"b"}},
			{Slug: "b", Name: "B", Category: "ml", Prerequisites: []string{"a"}},
			{Slug: "c", Name: "C", Category: "bogus", Aliases: []string{"Upper", "a"}, Prerequisites: []string{"missing"}},
		},
	}
	err := tax.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"siklus prerequisite", "category \"bogus\"", "harus lowercase", "alias \"a\" dipakai", "\"missing\" tidak ada"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error tidak memuat %q:\n%v", want, err)
		}
	}
}
