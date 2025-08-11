schema_set "core" {
  version      = "v1.0.5"
  repository   = "https://github.com/ttab/revisorschemas.git"

  schemas = [
    "core",
    "core-planning",
    "core-metadoc",
  ]
}

schema_set "tt" {
  version      = "v1.0.5"
  url_template = "https://raw.githubusercontent.com/ttab/revisorschemas/refs/tags/{{.Version}}/{{.Name}}.json"

  schemas = [
    "tt",
    "tt-planning",
    "tt-wires",
    "tt-print",
  ]
}
