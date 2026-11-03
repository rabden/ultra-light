// Package raycast holds what Raycast extensions and script commands are to
// Nanolight, apart from the panel and the processes: the host's protocol
// (raycast_model.zig, raycast/README.md "Protocol"), finding installed
// commands and Node (raycast.zig), and script commands' headers
// (script_commands.zig).
package raycast

import (
	"encoding/json"
	"strings"
)

// Icon is an item's picture: text (an emoji or a rank), a file, a URL, or a
// built-in Raycast icon's name.
type Icon struct {
	Text    *string `json:"text,omitempty"`
	File    *string `json:"file,omitempty"`
	URL     *string `json:"url,omitempty"`
	Builtin *string `json:"builtin,omitempty"`
}

type Action struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Shortcut *string `json:"shortcut,omitempty"`
}

type Item struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle,omitempty"`
	Icon        *Icon    `json:"icon,omitempty"`
	Accessories []string `json:"accessories,omitempty"`
	Actions     []Action `json:"actions,omitempty"`
}

type Section struct {
	Title string `json:"title,omitempty"`
	Items []Item `json:"items"`
}

type Empty struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

type Meta struct {
	Label string `json:"label"`
	Text  string `json:"text,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Title string `json:"title"`
}

// Field is a form's field; Value is a string, a bool or nil.
type Field struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Value       any      `json:"value,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []Option `json:"options,omitempty"`
}

// View is the top view: a list (a Grid comes as one), a detail or a form.
type View struct {
	Kind        string    `json:"kind"`
	Title       string    `json:"title,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Loading     bool      `json:"loading,omitempty"`
	Sections    []Section `json:"sections,omitempty"`
	Empty       *Empty    `json:"empty,omitempty"`
	Selected    *string   `json:"selected,omitempty"`
	Actions     []Action  `json:"actions,omitempty"`
	Markdown    *string   `json:"markdown,omitempty"`
	// Output is text shown as printed (a script command's).
	Output   *string `json:"output,omitempty"`
	Metadata []Meta  `json:"metadata,omitempty"`
	Fields   []Field `json:"fields,omitempty"`
}

func (v *View) IsList() bool { return v.Kind == "" || v.Kind == "list" }
func (v *View) IsForm() bool { return v.Kind == "form" }

// Count is the number of items through the sections.
func (v *View) Count() int {
	n := 0
	for _, s := range v.Sections {
		n += len(s.Items)
	}
	return n
}

// Item is the i-th item counting through the sections, or nil.
func (v *View) Item(i int) *Item {
	for si := range v.Sections {
		s := &v.Sections[si]
		if i < len(s.Items) {
			return &s.Items[i]
		}
		i -= len(s.Items)
	}
	return nil
}

// ActionsAt are the actions in play: the selected item's in a list, else
// the view's.
func (v *View) ActionsAt(sel int) []Action {
	if v.IsList() {
		if it := v.Item(sel); it != nil {
			return it.Actions
		}
	}
	return v.Actions
}

// HasArea reports whether a form has a text area (Return breaks its lines).
func (v *View) HasArea() bool {
	for _, f := range v.Fields {
		if f.Type == "textarea" {
			return true
		}
	}
	return false
}

// Message is one line from the host.
type Message struct {
	Type    string          `json:"type"`
	View    *View           `json:"view,omitempty"`
	Style   string          `json:"style,omitempty"`
	Title   *string         `json:"title,omitempty"`
	Message *string         `json:"message,omitempty"`
	Target  string          `json:"target,omitempty"`
	Text    *string         `json:"text,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Value   any             `json:"value,omitempty"`
}

// Parse reads one line.
func Parse(line []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(line, &m)
	return m, err
}

// Say is a message to the host: {"type":kind, field: value} and a newline;
// field "" leaves it out.
func Say(kind, field string, value any) []byte {
	b, _ := orderedSay{kind, field, value}.MarshalJSON()
	return append(b, '\n')
}

// orderedSay writes "type" first, as the Zig build does.
type orderedSay struct {
	kind, field string
	value       any
}

func (o orderedSay) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	k, _ := json.Marshal(o.kind)
	b.WriteString(`{"type":`)
	b.Write(k)
	if o.field != "" {
		f, _ := json.Marshal(o.field)
		v, err := json.Marshal(o.value)
		if err != nil {
			return nil, err
		}
		b.WriteByte(',')
		b.Write(f)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// FieldValue is a form field's new value, for the host.
func FieldValue(id string, value any) []byte {
	b, _ := json.Marshal(struct {
		Type  string `json:"type"`
		ID    string `json:"id"`
		Value any    `json:"value"`
	}{"field", id, value})
	return append(b, '\n')
}

// Reply answers a request by its id.
func Reply(id json.RawMessage, result any) []byte {
	b, _ := json.Marshal(struct {
		Type   string          `json:"type"`
		ID     json.RawMessage `json:"id"`
		Result any             `json:"result"`
	}{"reply", id, result})
	return append(b, '\n')
}

// Plain is markdown as plain lines for the panel: headings, emphasis and
// code marks dropped, images left out, links as their words.
func Plain(md string) string {
	var b strings.Builder
	lineStart := true
	for i := 0; i < len(md); {
		ch := md[i]
		if lineStart {
			for i < len(md) && (md[i] == '#' || (md[i] == '>' && (i+1 >= len(md) || md[i+1] == ' '))) {
				i++
			}
			out := b.String()
			for i < len(md) && md[i] == ' ' && len(out) > 0 && out[len(out)-1] == '\n' {
				i++
			}
			lineStart = false
			continue
		}
		if ch == '!' && i+1 < len(md) && md[i+1] == '[' {
			// ![alt](src): left out.
			if c := strings.IndexByte(md[i:], ')'); c >= 0 {
				i += c + 1
			} else {
				i = len(md)
			}
			continue
		}
		if ch == '[' {
			// [words](href): the words.
			if rb := strings.IndexByte(md[i:], ']'); rb >= 0 && i+rb+1 < len(md) && md[i+rb+1] == '(' {
				b.WriteString(md[i+1 : i+rb])
				if c := strings.IndexByte(md[i+rb:], ')'); c >= 0 {
					i += rb + c + 1
				} else {
					i = len(md)
				}
				continue
			}
		}
		if ch == '*' || ch == '`' || (ch == '_' && i+1 < len(md) && md[i+1] == '_') {
			i++
			continue
		}
		b.WriteByte(ch)
		if ch == '\n' {
			lineStart = true
		}
		i++
	}
	return strings.Trim(b.String(), " \n")
}
