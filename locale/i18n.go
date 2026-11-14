package locale

import (
	"embed"
	"os"
	"strings"
	"sync/atomic"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type Strmap map[string]interface{}

//go:embed *.yaml
var localesFS embed.FS

var (
	bundle  *i18n.Bundle
	english *i18n.Localizer
	lang    atomic.Pointer[i18n.Localizer]
)

// LanguageEnv overrides the detected system language (for example "de" or "en").
const LanguageEnv = "BEDROCKTOOL_LANG"

func init() {
	// English is always loaded and is the fallback for missing keys or languages.
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	if _, err := bundle.LoadMessageFileFS(localesFS, "en.yaml"); err != nil {
		panic(err) // embedded file, can only fail at development time
	}
	entries, _ := localesFS.ReadDir(".")
	for _, e := range entries {
		name := e.Name()
		if name == "en.yaml" || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		// a broken translation file must never stop the program; English still works
		_, _ = bundle.LoadMessageFileFS(localesFS, name)
	}
	english = i18n.NewLocalizer(bundle, "en")

	languageName := os.Getenv(LanguageEnv)
	if languageName == "" {
		languageName = getLanguageName()
	}
	SetLanguage(languageName)
}

// SetLanguage switches the language used by Loc and Locm.
// Unknown or empty names fall back to English.
func SetLanguage(name string) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "_", "-")
	langs := []string{"en"}
	if tag, err := language.Parse(name); err == nil {
		base, _ := tag.Base()
		langs = []string{tag.String(), base.String(), "en"}
	}
	lang.Store(i18n.NewLocalizer(bundle, langs...))
}

func localize(cfg *i18n.LocalizeConfig) string {
	s, err := lang.Load().Localize(cfg)
	if s != "" {
		// go-i18n returns the English text together with an error when
		// the selected language lacks the key, which is what we want.
		return s
	}
	if err != nil {
		// fall back to plain English, then to the key itself; never crash
		if s, _ := english.Localize(cfg); s != "" {
			return s
		}
	}
	return cfg.MessageID
}

// Loc returns the message for id in the current language, filled with tmpl.
func Loc(id string, tmpl Strmap) string {
	return localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: tmpl,
	})
}

// Locm is Loc with plural selection by count (yaml "one"/"other").
// The count is available to the template as .Count unless tmpl sets it.
func Locm(id string, tmpl Strmap, count int) string {
	data := Strmap{"Count": count}
	for k, v := range tmpl {
		data[k] = v
	}
	return localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: data,
		PluralCount:  count,
	})
}

// Has reports whether id exists in the English messages.
func Has(id string) bool {
	_, err := english.Localize(&i18n.LocalizeConfig{MessageID: id})
	return err == nil
}
