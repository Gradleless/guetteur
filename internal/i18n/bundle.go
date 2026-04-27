package i18n

import (
	"embed"
	"log/slog"

	"github.com/BurntSushi/toml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales
var localesFS embed.FS

var bundle *goi18n.Bundle

func init() {
	bundle = goi18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		slog.Error("i18n: read locales dir", "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, err := bundle.LoadMessageFileFS(localesFS, "locales/"+e.Name()); err != nil {
			slog.Error("i18n: load message file", "file", e.Name(), "err", err)
		}
	}
}

type Localizer struct {
	l *goi18n.Localizer
}

func New(lang string) *Localizer {
	if lang == "" {
		lang = "fr"
	}
	return &Localizer{l: goi18n.NewLocalizer(bundle, lang, "en")}
}

// T returns the translation for the given message ID.
func (loc *Localizer) T(id string) string {
	msg, err := loc.l.Localize(&goi18n.LocalizeConfig{MessageID: id})
	if err != nil {
		return id
	}
	return msg
}

// Tf returns the translation with named template data substituted.
func (loc *Localizer) Tf(id string, data any) string {
	msg, err := loc.l.Localize(&goi18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: data,
	})
	if err != nil {
		return id
	}
	return msg
}