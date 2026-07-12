package registrydigest

import "testing"

// Frozen golden vectors. A change here means the canonicalizer or the pinned
// yaml.v3 version changed; either is a deliberate, reviewed event that must
// re-freeze these constants in the same commit (conventions §18).
const (
	goldenRevenueDigest = "sha256:1a667f19ec657dd0697e3de63fcf80f8393375c4baa74fe6c53e81b12823cd22"
	goldenStyleADigest  = "sha256:dea7ac181bf220b3fa64a556149193fba34ba7345264361740472179410211aa"
	goldenCohortDigest  = "sha256:09b7195bc9560b1c6a4525bda1f03b5f044061e2f9fdab8991e55a99995f0d29"

	goldenRevenueCanon = `{"apiVersion":"bino.bi/v1alpha1","kind":"Table","metadata":{"description":"Regional revenue table with AC vs PY variances.","labels":{"registry.category":"finance","registry.compat":">=1.0.0 <2.0.0","registry.dependencies":"@bino/style_a,@bino/translation_b","registry.description":"Standard regional revenue table with absolute and % variance columns.","registry.icon":"📊","registry.requires.dataset.revenue.columns":"month,amount","registry.requires.dataset.revenue.description":"Regional monthly revenue facts","registry.requires.datasets":"revenue","registry.requires.param.REGION.required":"true","registry.requires.param.REGION.type":"string","registry.requires.param.YEAR.required":"true","registry.requires.param.YEAR.type":"number","registry.requires.params":"REGION,YEAR","registry.tags":"revenue,finance,variance,kpi","registry.title":"Revenue Table (AC/PY)"},"name":"@acme/revenue-table"},"spec":{"dataset":"$revenue","grouped":true,"scenarios":["ac1","pp1"],"style":"$@bino/style_a","variances":["dac1_pp1_pos"]}}`
)

func TestDigest_GoldenVectors(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"revenue_table", revenueTableYAML, goldenRevenueDigest},
		{"style_a", styleAYAML, goldenStyleADigest},
		{"cohort_retention", cohortRetentionYAML, goldenCohortDigest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Digest([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("Digest: %v", err)
			}
			if got != tc.want {
				t.Errorf("digest drift:\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestCanonicalize_GoldenBytes(t *testing.T) {
	got, err := Canonicalize([]byte(revenueTableYAML))
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if string(got) != goldenRevenueCanon {
		t.Errorf("canonical bytes drift:\n got %s\nwant %s", got, goldenRevenueCanon)
	}
}
