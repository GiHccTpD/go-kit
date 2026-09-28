package v4

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const redactedValue = "🔒 [REDACTED]"

type redactingCore struct {
	zapcore.Core
	keys map[string]struct{}
}

func newRedactingCore(core zapcore.Core, keys []string) zapcore.Core {
	redactKeys := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = normalizeKey(key)
		if key != "" {
			redactKeys[key] = struct{}{}
		}
	}
	return &redactingCore{Core: core, keys: redactKeys}
}

func (c *redactingCore) With(fields []zapcore.Field) zapcore.Core {
	return &redactingCore{Core: c.Core.With(redactFields(fields, c.keys)), keys: c.keys}
}

func (c *redactingCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *redactingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	return c.Core.Write(entry, redactFields(fields, c.keys))
}

func redactFields(fields []zapcore.Field, keys map[string]struct{}) []zapcore.Field {
	if len(fields) == 0 {
		return fields
	}
	redacted := make([]zapcore.Field, len(fields))
	for i, field := range fields {
		redacted[i] = redactField(field, keys)
	}
	return redacted
}

func redactField(field zapcore.Field, keys map[string]struct{}) zapcore.Field {
	if isSensitiveKey(field.Key, keys) {
		return zap.String(field.Key, redactedValue)
	}
	switch field.Type {
	case zapcore.ArrayMarshalerType:
		return zap.Array(field.Key, redactingArrayMarshaler{marshaler: field.Interface.(zapcore.ArrayMarshaler), keys: keys})
	case zapcore.ObjectMarshalerType:
		return zap.Object(field.Key, redactingObjectMarshaler{marshaler: field.Interface.(zapcore.ObjectMarshaler), keys: keys})
	case zapcore.InlineMarshalerType:
		return zap.Inline(redactingObjectMarshaler{marshaler: field.Interface.(zapcore.ObjectMarshaler), keys: keys})
	case zapcore.ReflectType:
		if value, changed := redactReflectedValue(field.Interface, keys); changed {
			return zap.Any(field.Key, value)
		}
	}
	return field
}

func isSensitiveKey(key string, keys map[string]struct{}) bool {
	if _, ok := keys[normalizeKey(key)]; ok {
		return true
	}
	compact := normalizeKey(key)
	if strings.Contains(compact, "password") || strings.Contains(compact, "passwd") {
		return true
	}
	for _, segment := range keySegments(key) {
		if segment == "pwd" {
			return true
		}
	}
	return false
}

func normalizeKey(key string) string {
	var normalized strings.Builder
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			normalized.WriteRune(unicode.ToLower(r))
		}
	}
	return normalized.String()
}

func keySegments(key string) []string {
	runes := []rune(key)
	segments := make([]string, 0, 3)
	start := 0
	for i, current := range runes {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			if start < i {
				segments = append(segments, strings.ToLower(string(runes[start:i])))
			}
			start = i + 1
			continue
		}
		if i > start && unicode.IsUpper(current) &&
			(unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) ||
				(i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1]))) {
			segments = append(segments, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	if start < len(runes) {
		segments = append(segments, strings.ToLower(string(runes[start:])))
	}
	return segments
}

func redactReflectedValue(value any, keys map[string]struct{}) (any, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value, false
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return value, false
	}
	redacted, changed := redactJSONValue(decoded, keys)
	if !changed {
		return value, false
	}
	return redacted, true
}

func redactJSONValue(value any, keys map[string]struct{}) (any, bool) {
	switch value := value.(type) {
	case map[string]any:
		changed := false
		for key, child := range value {
			if isSensitiveKey(key, keys) {
				value[key] = redactedValue
				changed = true
				continue
			}
			redacted, childChanged := redactJSONValue(child, keys)
			if childChanged {
				value[key] = redacted
				changed = true
			}
		}
		return value, changed
	case []any:
		changed := false
		for i, child := range value {
			redacted, childChanged := redactJSONValue(child, keys)
			if childChanged {
				value[i] = redacted
				changed = true
			}
		}
		return value, changed
	default:
		return value, false
	}
}

type redactingObjectMarshaler struct {
	marshaler zapcore.ObjectMarshaler
	keys      map[string]struct{}
}

func (m redactingObjectMarshaler) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	return m.marshaler.MarshalLogObject(redactingObjectEncoder{ObjectEncoder: encoder, keys: m.keys})
}

type redactingArrayMarshaler struct {
	marshaler zapcore.ArrayMarshaler
	keys      map[string]struct{}
}

func (m redactingArrayMarshaler) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	return m.marshaler.MarshalLogArray(redactingArrayEncoder{ArrayEncoder: encoder, keys: m.keys})
}

type redactingObjectEncoder struct {
	zapcore.ObjectEncoder
	keys map[string]struct{}
}

func (e redactingObjectEncoder) AddArray(key string, value zapcore.ArrayMarshaler) error {
	if e.sensitive(key) {
		e.ObjectEncoder.AddString(key, redactedValue)
		return nil
	}
	return e.ObjectEncoder.AddArray(key, redactingArrayMarshaler{marshaler: value, keys: e.keys})
}

func (e redactingObjectEncoder) AddObject(key string, value zapcore.ObjectMarshaler) error {
	if e.sensitive(key) {
		e.ObjectEncoder.AddString(key, redactedValue)
		return nil
	}
	return e.ObjectEncoder.AddObject(key, redactingObjectMarshaler{marshaler: value, keys: e.keys})
}

func (e redactingObjectEncoder) AddBinary(key string, value []byte) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddBinary(key, value)
}

func (e redactingObjectEncoder) AddByteString(key string, value []byte) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddByteString(key, value)
}

func (e redactingObjectEncoder) AddBool(key string, value bool) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddBool(key, value)
}

func (e redactingObjectEncoder) AddComplex128(key string, value complex128) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddComplex128(key, value)
}

func (e redactingObjectEncoder) AddComplex64(key string, value complex64) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddComplex64(key, value)
}

func (e redactingObjectEncoder) AddDuration(key string, value time.Duration) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddDuration(key, value)
}

func (e redactingObjectEncoder) AddFloat64(key string, value float64) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddFloat64(key, value)
}

func (e redactingObjectEncoder) AddFloat32(key string, value float32) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddFloat32(key, value)
}

func (e redactingObjectEncoder) AddInt(key string, value int) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddInt(key, value)
}

func (e redactingObjectEncoder) AddInt64(key string, value int64) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddInt64(key, value)
}

func (e redactingObjectEncoder) AddInt32(key string, value int32) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddInt32(key, value)
}

func (e redactingObjectEncoder) AddInt16(key string, value int16) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddInt16(key, value)
}

func (e redactingObjectEncoder) AddInt8(key string, value int8) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddInt8(key, value)
}

func (e redactingObjectEncoder) AddString(key, value string) {
	if e.sensitive(key) {
		value = redactedValue
	}
	e.ObjectEncoder.AddString(key, value)
}

func (e redactingObjectEncoder) AddTime(key string, value time.Time) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddTime(key, value)
}

func (e redactingObjectEncoder) AddUint(key string, value uint) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddUint(key, value)
}

func (e redactingObjectEncoder) AddUint64(key string, value uint64) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddUint64(key, value)
}

func (e redactingObjectEncoder) AddUint32(key string, value uint32) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddUint32(key, value)
}

func (e redactingObjectEncoder) AddUint16(key string, value uint16) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddUint16(key, value)
}

func (e redactingObjectEncoder) AddUint8(key string, value uint8) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddUint8(key, value)
}

func (e redactingObjectEncoder) AddUintptr(key string, value uintptr) {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return
	}
	e.ObjectEncoder.AddUintptr(key, value)
}

func (e redactingObjectEncoder) AddReflected(key string, value any) error {
	if e.sensitive(key) {
		e.AddString(key, redactedValue)
		return nil
	}
	if redacted, changed := redactReflectedValue(value, e.keys); changed {
		value = redacted
	}
	return e.ObjectEncoder.AddReflected(key, value)
}

func (e redactingObjectEncoder) OpenNamespace(key string) {
	e.ObjectEncoder.OpenNamespace(key)
}

func (e redactingObjectEncoder) sensitive(key string) bool {
	return isSensitiveKey(key, e.keys)
}

type redactingArrayEncoder struct {
	zapcore.ArrayEncoder
	keys map[string]struct{}
}

func (e redactingArrayEncoder) AppendArray(value zapcore.ArrayMarshaler) error {
	return e.ArrayEncoder.AppendArray(redactingArrayMarshaler{marshaler: value, keys: e.keys})
}

func (e redactingArrayEncoder) AppendObject(value zapcore.ObjectMarshaler) error {
	return e.ArrayEncoder.AppendObject(redactingObjectMarshaler{marshaler: value, keys: e.keys})
}

func (e redactingArrayEncoder) AppendReflected(value any) error {
	if redacted, changed := redactReflectedValue(value, e.keys); changed {
		value = redacted
	}
	return e.ArrayEncoder.AppendReflected(value)
}
