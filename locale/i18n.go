package locale

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type Strmap map[string]interface{}

//go:embed *.yaml
var localesFS embed.FS
var lang *i18n.Localizer

func load_language(bundle *i18n.Bundle, tag language.Tag) error {
	_, err := bundle.LoadMessageFileFS(localesFS, fmt.Sprintf("%s.yaml", tag.String()))
	return err
}

func init() {
	var defaultTag language.Tag = language.English
	var err error

	var languageName string
	f := flag.NewFlagSet("bedrocktool", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&languageName, "lang", "", "")
	f.Parse(os.Args[1:])

	// get default language
	if languageName == "" {
		languageName = getLanguageName()
	}
	defaultTag, err = language.Parse(languageName)
	if err != nil {
		logrus.Warn("failed to parse language name")
	}

	bundle := i18n.NewBundle(defaultTag)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	err = load_language(bundle, defaultTag)
	if err != nil {
		//logrus.Warnf("Couldnt load Language %s", languageName)
		err = load_language(bundle, language.English)
		if err != nil {
			logrus.Error("failed to load english language")
		}
	}

	lang = i18n.NewLocalizer(bundle, "en")
}

func Loc(id string, tmpl Strmap) string {
	s, err := lang.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: tmpl,
	})
	if err != nil {
		return fmt.Sprintf("failed to translate! %s", id)
	}
	return s
}

func Locm(id string, tmpl Strmap, count int) string {
	s, err := lang.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: tmpl,
		PluralCount:  count,
	})
	if err != nil {
		return fmt.Sprintf("failed to translate! %s", id)
	}
	return s
}
