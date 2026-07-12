package registrydigest

// Golden fixture Documents drawn from the running examples (conventions §13,
// domain-model §8.1). They are the single source of truth for cross-build
// determinism: the service-publish and plugin-verify suites import this package
// and assert the same digests.

const revenueTableYAML = `apiVersion: bino.bi/v1alpha1
kind: Table
metadata:
  name: "@acme/revenue-table"          # metadata.name IS the fully-scoped @scope/name
  description: Regional revenue table with AC vs PY variances.
  labels:
    registry.title: "Revenue Table (AC/PY)"
    registry.description: "Standard regional revenue table with absolute and % variance columns."
    registry.tags: "revenue,finance,variance,kpi"
    registry.category: "finance"
    registry.icon: "📊"
    registry.compat: ">=1.0.0 <2.0.0"
    registry.dependencies: "@bino/style_a,@bino/translation_b"
    registry.requires.datasets: "revenue"
    registry.requires.dataset.revenue.columns: "month,amount"
    registry.requires.dataset.revenue.description: "Regional monthly revenue facts"
    registry.requires.params: "REGION,YEAR"
    registry.requires.param.REGION.type: "string"
    registry.requires.param.REGION.required: "true"
    registry.requires.param.YEAR.type: "number"
    registry.requires.param.YEAR.required: "true"
spec:
  style: "$@bino/style_a"
  dataset: $revenue
  scenarios:
    - ac1
    - pp1
  variances:
    - dac1_pp1_pos
  grouped: true
`

// Same Document, re-indented to 4 spaces, comments stripped, and labels
// reordered. Must digest identically to revenueTableYAML (the headline property).
const revenueTableReformattedYAML = `apiVersion: bino.bi/v1alpha1
kind: Table
metadata:
    name: "@acme/revenue-table"
    description: Regional revenue table with AC vs PY variances.
    labels:
        registry.requires.param.YEAR.required: "true"
        registry.requires.param.YEAR.type: "number"
        registry.requires.param.REGION.required: "true"
        registry.requires.param.REGION.type: "string"
        registry.requires.params: "REGION,YEAR"
        registry.requires.dataset.revenue.description: "Regional monthly revenue facts"
        registry.requires.dataset.revenue.columns: "month,amount"
        registry.requires.datasets: "revenue"
        registry.dependencies: "@bino/style_a,@bino/translation_b"
        registry.compat: ">=1.0.0 <2.0.0"
        registry.icon: "📊"
        registry.category: "finance"
        registry.tags: "revenue,finance,variance,kpi"
        registry.description: "Standard regional revenue table with absolute and % variance columns."
        registry.title: "Revenue Table (AC/PY)"
spec:
    style: "$@bino/style_a"
    dataset: $revenue
    scenarios: [ac1, pp1]
    variances: [dac1_pp1_pos]
    grouped: true
`

// JSON form of the same Document. Valid JSON is valid YAML 1.2, so this must
// digest identically to revenueTableYAML.
const revenueTableJSON = `{
  "apiVersion": "bino.bi/v1alpha1",
  "kind": "Table",
  "metadata": {
    "name": "@acme/revenue-table",
    "description": "Regional revenue table with AC vs PY variances.",
    "labels": {
      "registry.title": "Revenue Table (AC/PY)",
      "registry.description": "Standard regional revenue table with absolute and % variance columns.",
      "registry.tags": "revenue,finance,variance,kpi",
      "registry.category": "finance",
      "registry.icon": "📊",
      "registry.compat": ">=1.0.0 <2.0.0",
      "registry.dependencies": "@bino/style_a,@bino/translation_b",
      "registry.requires.datasets": "revenue",
      "registry.requires.dataset.revenue.columns": "month,amount",
      "registry.requires.dataset.revenue.description": "Regional monthly revenue facts",
      "registry.requires.params": "REGION,YEAR",
      "registry.requires.param.REGION.type": "string",
      "registry.requires.param.REGION.required": "true",
      "registry.requires.param.YEAR.type": "number",
      "registry.requires.param.YEAR.required": "true"
    }
  },
  "spec": {
    "style": "$@bino/style_a",
    "dataset": "$revenue",
    "scenarios": ["ac1", "pp1"],
    "variances": ["dac1_pp1_pos"],
    "grouped": true
  }
}
`

const styleAYAML = `apiVersion: bino.bi/v1alpha1
kind: ComponentStyle
metadata:
  name: "@bino/style_a"
  description: First-party shared component style.
  labels:
    registry.title: "Style A"
    registry.description: "First-party shared component style."
    registry.tags: "style,theme,first-party"
    registry.category: "styling"
spec:
  palette:
    primary: "#1f6feb"
    secondary: "#8250df"
  font:
    family: "Inter"
    size: 14
  radius: 6
`

const cohortRetentionYAML = `apiVersion: bino.bi/v1alpha1
kind: ChartTime
metadata:
  name: "@growth/cohort-retention"
  description: Cohort retention over time.
  labels:
    registry.title: "Cohort Retention"
    registry.description: "Retention by signup cohort over time."
    registry.tags: "growth,retention,cohort,timeseries"
    registry.category: "growth"
    registry.requires.datasets: "cohorts"
    registry.requires.dataset.cohorts.columns: "cohort,period,retained"
    registry.requires.params: "WINDOW"
    registry.requires.param.WINDOW.type: "number"
    registry.requires.param.WINDOW.default: "12"
spec:
  x: $period
  y: $retained
  series: $cohort
  smooth: false
  maxPeriods: 24
`
