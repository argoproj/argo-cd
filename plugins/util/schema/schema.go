package schema

type Properties map[string]*Schema

type Schema struct {
	Type       string     `json:"type,omitempty"`
	Title      string     `json:"title,omitempty"`
	Format     string     `json:"format,omitempty"`
	Properties Properties `json:"properties,omitempty"`
	Items      *Schema    `json:"items,omitempty"`
	Enum       []string   `json:"enum,omitempty"`
}
