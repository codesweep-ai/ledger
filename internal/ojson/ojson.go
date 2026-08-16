// Package ojson is an order-preserving JSON representation whose serializer
// reproduces ECMAScript JSON.stringify byte-for-byte for values that came
// from JSON.parse: object keys keep file order, strings use JS escaping
// (no HTML escaping), numbers render as JS doubles. That is what makes a
// rendered ledger.html reproducible byte for byte, which the freshness gate
// in `cs-ledger check` depends on.
package ojson

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Kind byte

const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

type KV struct {
	Key string
	Val *Value
}

type Value struct {
	Kind Kind
	Str  string
	Num  float64
	B    bool
	Arr  []*Value
	Obj  []KV
}

// ---- constructors (for building payloads by hand) ----
//
// No A(...) for arrays: nothing builds one by hand, and a constructor no caller
// reaches is a claim about the API that nothing backs. If one is ever wanted,
// &Value{Kind: Array, Arr: vs} is the line it would have held.

func S(s string) *Value        { return &Value{Kind: String, Str: s} }
func N(f float64) *Value       { return &Value{Kind: Number, Num: f} }
func Nil() *Value              { return &Value{Kind: Null} }
func O(kvs ...KV) *Value       { return &Value{Kind: Object, Obj: kvs} }
func Kv(k string, v *Value) KV { return KV{Key: k, Val: v} }

// ---- typed accessors ----

func (v *Value) IsNull() bool { return v == nil || v.Kind == Null }

// Get returns the member value, or nil if absent (or not an object).
func (v *Value) Get(key string) *Value {
	if v == nil || v.Kind != Object {
		return nil
	}
	for _, kv := range v.Obj {
		if kv.Key == key {
			return kv.Val
		}
	}
	return nil
}

func (v *Value) Has(key string) bool {
	if v == nil || v.Kind != Object {
		return false
	}
	for _, kv := range v.Obj {
		if kv.Key == key {
			return true
		}
	}
	return false
}

// Keys returns object keys in file order.
func (v *Value) Keys() []string {
	if v == nil || v.Kind != Object {
		return nil
	}
	ks := make([]string, len(v.Obj))
	for i, kv := range v.Obj {
		ks[i] = kv.Key
	}
	return ks
}

// StrVal returns the string value and whether the value is a string.
func (v *Value) StrVal() (string, bool) {
	if v == nil || v.Kind != String {
		return "", false
	}
	return v.Str, true
}

// ---- parsing ----

// Parse decodes JSON preserving object key order.
func Parse(data []byte) (*Value, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	// reject trailing content
	if dec.More() {
		return nil, errors.New("unexpected trailing content")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (*Value, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return fromToken(dec, tok)
}

func fromToken(dec *json.Decoder, tok json.Token) (*Value, error) {
	switch t := tok.(type) {
	case nil:
		return Nil(), nil
	case bool:
		return &Value{Kind: Bool, B: t}, nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return nil, err
		}
		return N(f), nil
	case string:
		return S(t), nil
	case json.Delim:
		switch t {
		case '{':
			obj := &Value{Kind: Object}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := kt.(string)
				if !ok {
					return nil, errors.New("object key is not a string")
				}
				val, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				obj.Obj = append(obj.Obj, KV{Key: key, Val: val})
			}
			if _, err := dec.Token(); err != nil { // consume '}'
				return nil, err
			}
			return obj, nil
		case '[':
			arr := &Value{Kind: Array}
			for dec.More() {
				val, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				arr.Arr = append(arr.Arr, val)
			}
			if _, err := dec.Token(); err != nil { // consume ']'
				return nil, err
			}
			return arr, nil
		}
	}
	return nil, fmt.Errorf("unexpected token %v", tok)
}

// ---- serialization (JSON.stringify compatible, compact) ----

func (v *Value) Stringify(sb *strings.Builder) {
	if v == nil {
		sb.WriteString("null")
		return
	}
	switch v.Kind {
	case Null:
		sb.WriteString("null")
	case Bool:
		if v.B {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case Number:
		sb.WriteString(jsNumber(v.Num))
	case String:
		writeJSString(sb, v.Str)
	case Array:
		sb.WriteByte('[')
		for i, e := range v.Arr {
			if i > 0 {
				sb.WriteByte(',')
			}
			e.Stringify(sb)
		}
		sb.WriteByte(']')
	case Object:
		sb.WriteByte('{')
		for i, kv := range v.Obj {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeJSString(sb, kv.Key)
			sb.WriteByte(':')
			kv.Val.Stringify(sb)
		}
		sb.WriteByte('}')
	}
}

func (v *Value) String() string {
	var sb strings.Builder
	v.Stringify(&sb)
	return sb.String()
}

// jsNumber formats a float64 the way ECMAScript Number::toString does for
// the values that occur in JSON data (shortest round-trip representation).
func jsNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null" // JSON.stringify of non-finite numbers
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	// Go emits "1e+21"; JS emits "1e+21" too for >=1e21, but "0.000001" vs
	// "1e-06" boundaries differ. Corpus values are plain; the differential
	// oracle guards this.
	return s
}

// writeJSString escapes exactly like JSON.stringify: quote, backslash, the
// named control escapes, \u00XX for other control chars. No HTML escaping,
// non-ASCII passes through raw.
func writeJSString(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(sb, `\u%04x`, r)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
}

// StringifyIndent reproduces JSON.stringify(value, null, 2) — used when the
// tool rewrites ledger.json (e.g. the toolVersion pin) so config diffs stay
// minimal against hand-authored 2-space files.
func (v *Value) StringifyIndent() string {
	var sb strings.Builder
	v.writeIndent(&sb, 0)
	return sb.String()
}

func (v *Value) writeIndent(sb *strings.Builder, depth int) {
	ind := strings.Repeat("  ", depth)
	indIn := strings.Repeat("  ", depth+1)
	if v == nil {
		sb.WriteString("null")
		return
	}
	switch v.Kind {
	case Array:
		if len(v.Arr) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteString("[\n")
		for i, e := range v.Arr {
			if i > 0 {
				sb.WriteString(",\n")
			}
			sb.WriteString(indIn)
			e.writeIndent(sb, depth+1)
		}
		sb.WriteString("\n" + ind + "]")
	case Object:
		if len(v.Obj) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteString("{\n")
		for i, kv := range v.Obj {
			if i > 0 {
				sb.WriteString(",\n")
			}
			sb.WriteString(indIn)
			writeJSString(sb, kv.Key)
			sb.WriteString(": ")
			kv.Val.writeIndent(sb, depth+1)
		}
		sb.WriteString("\n" + ind + "}")
	default:
		v.Stringify(sb)
	}
}

// Set replaces the value for key, or appends the pair if absent.
func (v *Value) Set(key string, val *Value) {
	if v.Kind != Object {
		return
	}
	for i, kv := range v.Obj {
		if kv.Key == key {
			v.Obj[i].Val = val
			return
		}
	}
	v.Obj = append(v.Obj, KV{Key: key, Val: val})
}
