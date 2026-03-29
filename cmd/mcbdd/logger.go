package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// bracketHandler ist ein slog.Handler, der Log-Nachrichten im Format
// "2006/01/02 15:04:05 [LEVEL] message key=value" ausgibt.
// Die Level-Tags [INFO], [WARN], [ERROR] und [DEBUG] ermöglichen eine
// schnelle visuelle Zuordnung in Container-Logs.
type bracketHandler struct {
	level slog.Level
	out   io.Writer
	mu    *sync.Mutex
	attrs []slog.Attr
}

func newBracketHandler(out io.Writer, level slog.Level) *bracketHandler {
	return &bracketHandler{
		level: level,
		out:   out,
		mu:    &sync.Mutex{},
	}
}

func (h *bracketHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *bracketHandler) Handle(_ context.Context, r slog.Record) error {
	var levelTag string
	switch {
	case r.Level >= slog.LevelError:
		levelTag = "[ERROR]"
	case r.Level >= slog.LevelWarn:
		levelTag = "[WARN]"
	case r.Level >= slog.LevelInfo:
		levelTag = "[INFO]"
	default:
		levelTag = "[DEBUG]"
	}

	var b strings.Builder
	b.WriteString(r.Time.Format("2006/01/02 15:04:05"))
	b.WriteByte(' ')
	b.WriteString(levelTag)
	b.WriteByte(' ')
	b.WriteString(r.Message)

	writeAttr := func(a slog.Attr) bool {
		a.Value = a.Value.Resolve()
		if a.Equal(slog.Attr{}) {
			return true
		}
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(formatAttrValue(a.Value))
		return true
	}

	for _, a := range h.attrs {
		writeAttr(a)
	}
	r.Attrs(writeAttr)

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write([]byte(b.String()))
	return err
}

func (h *bracketHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &bracketHandler{
		level: h.level,
		out:   h.out,
		mu:    h.mu,
		attrs: newAttrs,
	}
}

func (h *bracketHandler) WithGroup(_ string) slog.Handler {
	// Gruppen werden für dieses Projekt nicht benötigt.
	return h
}

// formatAttrValue formatiert einen slog.Value als String.
// String-Werte mit Leerzeichen oder Sonderzeichen werden in Anführungszeichen gesetzt.
func formatAttrValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if s == "" || strings.ContainsAny(s, " \t\n\"\\") {
			return fmt.Sprintf("%q", s)
		}
		return s
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindDuration:
		return v.Duration().String()
	default:
		return fmt.Sprintf("%v", v.Any())
	}
}
